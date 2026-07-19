package service

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	exchange2 "github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type OrderService struct {
	DB          *sqlx.DB
	TablePrefix string
	UserID      string
}

func (s *OrderService) tableName(base string) string { return s.TablePrefix + base }

func (s *OrderService) Sync(
	ctx context.Context, exchange types.Exchange, symbol string,
	startTime, endTime time.Time,
) error {
	if s.DB == nil {
		return nil
	}
	isMargin, isFutures, isIsolated, isolatedSymbol := exchange2.GetSessionAttributes(exchange)
	// override symbol if isolatedSymbol is not empty
	if isIsolated && len(isolatedSymbol) > 0 {
		symbol = isolatedSymbol
	}
	logger := util.GetLoggerFromCtx(ctx)
	logger.Infof("session attributes: isMargin=%v isFutures=%v isIsolated=%v isolatedSymbol=%s", isMargin, isFutures, isIsolated, isolatedSymbol)

	api, ok := exchange.(types.ExchangeTradeHistoryService)
	if !ok {
		logger.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing orders", exchange.Name())
		return nil
	}

	lastOrderID := uint64(0)
	tasks := []SyncTask{
		{
			Type: types.Order{},
			Time: func(obj interface{}) time.Time {
				return obj.(types.Order).CreationTime.Time()
			},
			ID: func(obj interface{}) string {
				order := obj.(types.Order)
				return strconv.FormatUint(order.OrderID, 10)
			},
			Select: SelectLastOrders(s.DB.DriverName(), exchange.Name(), symbol, isMargin, isFutures, isIsolated, 100),
			OnLoad: func(objs interface{}) {
				// update last order ID
				orders := objs.([]types.Order)
				if len(orders) > 0 {
					end := len(orders) - 1
					last := orders[end]
					lastOrderID = last.OrderID
				}
			},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.ClosedOrderBatchQuery{
					ExchangeTradeHistoryService: api,
				}

				return query.Query(ctx, symbol, startTime, endTime, lastOrderID)
			},
			Filter: func(obj interface{}) bool {
				// skip canceled and not filled orders
				order := obj.(types.Order)
				if order.Status == types.OrderStatusCanceled && order.ExecutedQuantity.IsZero() {
					return false
				}

				return true
			},
			Insert: func(obj interface{}) error {
				order := obj.(types.Order)
				return s.Insert(order)
			},
			LogInsert: true,
		},
	}

	for _, sel := range tasks {
		if err := sel.execute(ctx, s.DB, startTime, endTime); err != nil {
			return err
		}
	}

	return nil
}

func SelectLastOrders(driver string, ex types.ExchangeName, symbol string, isMargin, isFutures, isIsolated bool, limit uint64) sq.SelectBuilder {
	if driver == "postgres" {
		return sq.Select(
			"gid", "exchange", "order_id", "client_order_id", "order_type",
			"status", "symbol", "price", "stop_price", "quantity",
			"executed_quantity", "side", "is_working", "time_in_force",
			"created_at", "updated_at", "is_margin", "is_futures", "is_isolated",
			"order_uuid AS uuid", "actual_order_id",
			"strategy_instance_id",
		).
			From("orders").
			Where(sq.And{
				sq.Eq{"symbol": symbol},
				sq.Eq{"exchange": ex},
				sq.Eq{"is_margin": isMargin},
				sq.Eq{"is_futures": isFutures},
				sq.Eq{"is_isolated": isIsolated},
			}).
			OrderBy("created_at DESC").
			Limit(limit)
	}
	return sq.Select("*").
		From("orders").
		Where(sq.And{
			sq.Eq{"symbol": symbol},
			sq.Eq{"exchange": ex},
			sq.Eq{"is_margin": isMargin},
			sq.Eq{"is_futures": isFutures},
			sq.Eq{"is_isolated": isIsolated},
		}).
		OrderBy("created_at DESC").
		Limit(limit)
}

type AggOrder struct {
	types.Order
	AveragePrice *float64 `json:"averagePrice" db:"average_price"`
}

type QueryOrdersOptions struct {
	Exchange types.ExchangeName
	Symbol   string
	LastGID  int64
	Ordering string
	Since    *time.Time
	Until    *time.Time
	Limit    int
}

func (s *OrderService) Query(options QueryOrdersOptions) ([]AggOrder, error) {
	sql := genOrderSQL(s.DB.DriverName(), s.tableName("orders"), options)

	params := map[string]interface{}{
		"exchange": options.Exchange,
		"symbol":   options.Symbol,
		"gid":      options.LastGID,
	}
	if options.Since != nil {
		params["since"] = *options.Since
	}
	if options.Until != nil {
		params["until"] = *options.Until
	}

	rows, err := s.DB.NamedQuery(sql, params)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanAggRows(rows)
}

func genOrderSQL(driver string, tableName string, options QueryOrdersOptions) string {
	// ascending
	ordering := "ASC"
	switch v := strings.ToUpper(options.Ordering); v {
	case "DESC", "ASC":
		ordering = options.Ordering
	}

	var where []string
	if options.LastGID > 0 {
		switch ordering {
		case "ASC":
			where = append(where, "orders.gid > :gid")
		case "DESC":
			where = append(where, "orders.gid < :gid")

		}
	}

	if len(options.Exchange) > 0 {
		where = append(where, "orders.exchange = :exchange")
	}
	if len(options.Symbol) > 0 {
		where = append(where, "orders.symbol = :symbol")
	}
	if options.Since != nil {
		where = append(where, "orders.created_at >= :since")
	}
	if options.Until != nil {
		where = append(where, "orders.created_at < :until")
	}

	var selColumns []string
	if driver == "mysql" {
		selColumns = append(selColumns, "orders.symbol", "orders.side", "orders.order_type", "orders.quantity", "orders.price")
		to := reflect.TypeOf(types.Order{})
		for i := 0; i < to.NumField(); i++ {
			field := to.Field(i)
			colName := field.Tag.Get("db")
			if colName == "" || colName == "-" {
				continue
			}
			if colName == "uuid" {
				selColumns = append(selColumns, binUuidSelector("orders", "uuid"))
			} else {
				selColumns = append(selColumns, "orders."+colName)
			}

		}
	} else if driver == "postgres" {
		selColumns = append(selColumns,
			"orders.gid", "orders.exchange", "orders.order_id",
			"orders.client_order_id", "orders.order_type", "orders.status",
			"orders.symbol", "orders.price", "orders.stop_price",
			"orders.quantity", "orders.executed_quantity", "orders.side",
			"orders.is_working", "orders.time_in_force",
			"orders.created_at", "orders.updated_at",
			"orders.is_margin", "orders.is_futures", "orders.is_isolated",
			"orders.order_uuid AS uuid",
			"orders.actual_order_id",
			"orders.strategy_instance_id",
		)
	} else {
		selColumns = append(selColumns, "orders.*")
	}

	avgPriceExpr := "IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price)"
	if driver == "postgres" {
		avgPriceExpr = "COALESCE(SUM(t.price * t.quantity)/NULLIF(SUM(t.quantity), 0), orders.price)"
	}
	selColumns = append(selColumns, avgPriceExpr+" AS average_price")

	sql := `SELECT ` + strings.Join(selColumns, ", ") + ` FROM ` + tableName +
		` LEFT JOIN trades AS t ON (t.order_id = orders.order_id)`
	if len(where) > 0 {
		sql += ` WHERE ` + strings.Join(where, " AND ")
	}
	sql += ` GROUP BY orders.gid `
	sql += ` ORDER BY orders.gid ` + ordering
	limit := 500
	if options.Limit > 0 {
		limit = options.Limit
	}
	sql += ` LIMIT ` + strconv.Itoa(limit)

	log.Info(sql)
	return sql
}

func (s *OrderService) scanAggRows(rows *sqlx.Rows) (orders []AggOrder, err error) {
	for rows.Next() {
		var order AggOrder
		if err := rows.StructScan(&order); err != nil {
			return nil, err
		}

		orders = append(orders, order)
	}

	return orders, rows.Err()
}

func (s *OrderService) scanRows(rows *sqlx.Rows) (orders []types.Order, err error) {
	for rows.Next() {
		var order types.Order
		if err := rows.StructScan(&order); err != nil {
			return nil, err
		}

		orders = append(orders, order)
	}

	return orders, rows.Err()
}

func (s *OrderService) Insert(order types.Order) (err error) {
	tableName := s.tableName("orders")

	switch s.DB.DriverName() {
	case "mysql":
		_, err = s.DB.NamedExec(`
			INSERT INTO `+"`"+tableName+"`"+` (exchange, order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at, updated_at, is_margin, is_futures, is_isolated, uuid, actual_order_id, strategy_instance_id, position_action)
			VALUES (:exchange, :order_id, :client_order_id, :order_type, :status, :symbol, :price, :stop_price, :quantity, :executed_quantity, :side, :is_working, :time_in_force, :created_at, :updated_at, :is_margin, :is_futures, :is_isolated, IF(:uuid != '', UUID_TO_BIN(:uuid, true), ''), :actual_order_id, :strategy_instance_id, :position_action)
			ON DUPLICATE KEY UPDATE status=:status, executed_quantity=:executed_quantity, is_working=:is_working, updated_at=:updated_at, position_action=:position_action`, order)

	case "postgres":
		_, err = s.DB.NamedExec(`
			INSERT INTO "`+tableName+`" (exchange, order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at, updated_at, is_margin, is_futures, is_isolated, order_uuid, actual_order_id, strategy_instance_id, user_id, position_action)
			VALUES (:exchange, :order_id, :client_order_id, :order_type, :status, :symbol, :price, :stop_price, :quantity, :executed_quantity, :side, :is_working, :time_in_force, :created_at, :updated_at, :is_margin, :is_futures, :is_isolated, :order_uuid, :actual_order_id, :strategy_instance_id, :user_id, :position_action)
			ON CONFLICT (user_id, order_id, exchange) DO UPDATE SET status=:status, executed_quantity=:executed_quantity, is_working=:is_working, updated_at=:updated_at, position_action=:position_action, strategy_instance_id=:strategy_instance_id`,
			map[string]interface{}{
				"exchange":             order.Exchange,
				"order_id":             strconv.FormatUint(order.OrderID, 10),
				"client_order_id":      order.ClientOrderID,
				"order_type":           order.Type,
				"status":               order.Status,
				"symbol":               order.Symbol,
				"price":                order.Price,
				"stop_price":           order.StopPrice,
				"quantity":             order.Quantity,
				"executed_quantity":    order.ExecutedQuantity,
				"side":                 order.Side,
				"is_working":           order.IsWorking,
				"time_in_force":        order.TimeInForce,
				"created_at":           order.CreationTime,
				"updated_at":           order.UpdateTime,
				"is_margin":            order.IsMargin,
				"is_futures":           order.IsFutures,
				"is_isolated":          order.IsIsolated,
				"order_uuid":           order.UUID,
				"actual_order_id":      order.ActualOrderId,
				"strategy_instance_id": order.StrategyInstanceID,
				"user_id":              s.UserID,
				"position_action":      order.PositionAction,
			})

	default: // sqlite3
		_, err = s.DB.NamedExec(`
			INSERT INTO `+"`"+tableName+"`"+` (exchange, order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at, updated_at, is_margin, is_futures, is_isolated, uuid, actual_order_id, strategy_instance_id, position_action)
			VALUES (:exchange, :order_id, :client_order_id, :order_type, :status, :symbol, :price, :stop_price, :quantity, :executed_quantity, :side, :is_working, :time_in_force, :created_at, :updated_at, :is_margin, :is_futures, :is_isolated, :uuid, :actual_order_id, :strategy_instance_id, :position_action)
		`, order)
	}

	return err
}

func (s *OrderService) DeleteByGID(ctx context.Context, gids []uint64) error {
	if len(gids) == 0 {
		return nil
	}

	const batchSize = 100
	for i := 0; i < len(gids); i += batchSize {
		end := min(i+batchSize, len(gids))
		sql, args, err := sq.Delete("orders").Where(sq.Eq{"gid": gids[i:end]}).ToSql()
		if err != nil {
			return err
		}
		if _, err := s.DB.ExecContext(ctx, sql, args...); err != nil {
			return err
		}
	}
	return nil
}
