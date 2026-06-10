package service

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	exchange2 "github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var ErrTradeNotFound = errors.New("trade not found")

type QueryTradesOptions struct {
	Exchange types.ExchangeName
	Sessions []string
	Symbol   string
	LastGID  int64

	// inclusive
	Since *time.Time

	// exclusive
	Until *time.Time

	// margin, futures, isolated
	IsMargin   *bool
	IsFutures  *bool
	IsIsolated *bool

	// ASC or DESC
	Ordering string

	// Strategy filters by the strategy instance ID stored on trades
	Strategy string

	// StrategyInstanceID filters by the unique strategy instance identifier
	StrategyInstanceID string

	// OrderByColumn is the column name to order by
	// Currently we only support traded_at and gid column.
	OrderByColumn string
	Limit         uint64
}

type TradingVolume struct {
	Year        int       `db:"year" json:"year"`
	Month       int       `db:"month" json:"month,omitempty"`
	Day         int       `db:"day" json:"day,omitempty"`
	Time        time.Time `json:"time,omitempty"`
	Exchange    string    `db:"exchange" json:"exchange,omitempty"`
	Symbol      string    `db:"symbol" json:"symbol,omitempty"`
	QuoteVolume float64   `db:"quote_volume" json:"quoteVolume"`
}

type TradingVolumeQueryOptions struct {
	GroupByPeriod string
	SegmentBy     string
}

type TradeService struct {
	DB          *sqlx.DB
	TablePrefix string
	UserID      string
}

func (s *TradeService) tableName(base string) string { return s.TablePrefix + base }

func NewTradeService(db *sqlx.DB) *TradeService {
	return &TradeService{DB: db}
}

func (s *TradeService) Sync(
	ctx context.Context,
	exchange types.Exchange, symbol string,
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
		logger.Warnf("exchange %s does not implement ExchangeTradeHistoryService, skip syncing trades", exchange.Name())
		return nil
	}

	lastTradeID := uint64(1)
	tasks := []SyncTask{
		{
			Type:   types.Trade{},
			Select: SelectLastTrades(exchange.Name(), symbol, isMargin, isFutures, isIsolated, 100),
			OnLoad: func(objs interface{}) {
				// update last trade ID
				trades := objs.([]types.Trade)
				if len(trades) > 0 {
					end := len(trades) - 1
					last := trades[end]
					lastTradeID = last.ID
				}
				logger.Infof("on load: last trade ID: %d", lastTradeID)
			},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.TradeBatchQuery{
					ExchangeTradeHistoryService: api,
				}
				logger.Infof("sync trades from %s to %s, lastTradeID: %d", startTime, endTime, lastTradeID)
				return query.Query(ctx, symbol, &types.TradeQueryOptions{
					StartTime:   &startTime,
					EndTime:     &endTime,
					LastTradeID: lastTradeID,
				})
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.Trade).Time.Time()
			},
			ID: func(obj interface{}) string {
				trade := obj.(types.Trade)
				id := strconv.FormatUint(trade.ID, 10) + trade.Side.String()
				return id
			},
			Insert: func(obj interface{}) error {
				trade := obj.(types.Trade)
				return s.Insert(trade)
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

func (s *TradeService) QueryTradingVolume(startTime time.Time, options TradingVolumeQueryOptions) ([]TradingVolume, error) {
	args := map[string]interface{}{
		// "symbol":      symbol,
		// "exchange":    ex,
		// "is_margin":   isMargin,
		// "is_isolated": isIsolated,
		"start_time": startTime,
	}

	var sql string
	driverName := s.DB.DriverName()
	switch driverName {
	case "mysql":
		sql = generateMysqlTradingVolumeQuerySQL(s.tableName("trades"), options)
	case "postgres":
		sql = generatePostgresTradingVolumeSQL(s.tableName("trades"), options)
	default:
		sql = generateSqliteTradingVolumeSQL(s.tableName("trades"), options)
	}

	log.Info(sql)

	rows, err := s.DB.NamedQuery(sql, args)
	if err != nil {
		return nil, errors.Wrap(err, "query last trade error")
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	defer rows.Close()

	var records []TradingVolume
	for rows.Next() {
		var record TradingVolume
		err = rows.StructScan(&record)
		if err != nil {
			return records, err
		}

		record.Time = time.Date(record.Year, time.Month(record.Month), record.Day, 0, 0, 0, 0, time.Local)
		records = append(records, record)
	}

	return records, rows.Err()
}

func generatePostgresTimeRangeClauses(timeRangeColumn, period string) (selectors []string, groupBys []string, orderBys []string) {
	switch period {
	case "month":
		selectors = append(selectors, "EXTRACT(YEAR FROM "+timeRangeColumn+")::int AS year", "EXTRACT(MONTH FROM "+timeRangeColumn+")::int AS month")
		groupBys = append([]string{"month", "year"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC")

	case "year":
		selectors = append(selectors, "EXTRACT(YEAR FROM "+timeRangeColumn+")::int AS year")
		groupBys = append([]string{"year"}, groupBys...)
		orderBys = append(orderBys, "year ASC")

	case "day":
		fallthrough

	default:
		selectors = append(selectors, "EXTRACT(YEAR FROM "+timeRangeColumn+")::int AS year", "EXTRACT(MONTH FROM "+timeRangeColumn+")::int AS month", "EXTRACT(DAY FROM "+timeRangeColumn+")::int AS day")
		groupBys = append([]string{"day", "month", "year"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC", "day ASC")
	}

	return
}

func generatePostgresTradingVolumeSQL(tableName string, options TradingVolumeQueryOptions) string {
	timeRangeColumn := "traded_at"
	sel, groupBys, orderBys := generatePostgresTimeRangeClauses(timeRangeColumn, options.GroupByPeriod)

	switch options.SegmentBy {
	case "symbol":
		sel = append(sel, "symbol")
		groupBys = append([]string{"symbol"}, groupBys...)
		orderBys = append(orderBys, "symbol")
	case "exchange":
		sel = append(sel, "exchange")
		groupBys = append([]string{"exchange"}, groupBys...)
		orderBys = append(orderBys, "exchange")
	}

	sel = append(sel, "SUM(quantity * price) AS quote_volume")
	where := []string{timeRangeColumn + " > :start_time"}
	sql := `SELECT ` + strings.Join(sel, ", ") + ` FROM "` + tableName + `"` +
		` WHERE ` + strings.Join(where, " AND ") +
		` GROUP BY ` + strings.Join(groupBys, ", ") +
		` ORDER BY ` + strings.Join(orderBys, ", ")

	return sql
}

func generateSqliteTradingVolumeSQL(tableName string, options TradingVolumeQueryOptions) string {
	timeRangeColumn := "traded_at"
	sel, groupBys, orderBys := generateSqlite3TimeRangeClauses(timeRangeColumn, options.GroupByPeriod)

	switch options.SegmentBy {
	case "symbol":
		sel = append(sel, "symbol")
		groupBys = append([]string{"symbol"}, groupBys...)
		orderBys = append(orderBys, "symbol")
	case "exchange":
		sel = append(sel, "exchange")
		groupBys = append([]string{"exchange"}, groupBys...)
		orderBys = append(orderBys, "exchange")
	}

	sel = append(sel, "SUM(quantity * price) AS quote_volume")
	where := []string{timeRangeColumn + " > :start_time"}
	sql := `SELECT ` + strings.Join(sel, ", ") + ` FROM ` + tableName +
		` WHERE ` + strings.Join(where, " AND ") +
		` GROUP BY ` + strings.Join(groupBys, ", ") +
		` ORDER BY ` + strings.Join(orderBys, ", ")

	return sql
}

func generateSqlite3TimeRangeClauses(timeRangeColumn, period string) (selectors []string, groupBys []string, orderBys []string) {
	switch period {
	case "month":
		selectors = append(selectors, "strftime('%Y',"+timeRangeColumn+") AS year", "strftime('%m',"+timeRangeColumn+") AS month")
		groupBys = append([]string{"month", "year"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC")

	case "year":
		selectors = append(selectors, "strftime('%Y',"+timeRangeColumn+") AS year")
		groupBys = append([]string{"year"}, groupBys...)
		orderBys = append(orderBys, "year ASC")

	case "day":
		fallthrough

	default:
		selectors = append(selectors, "strftime('%Y',"+timeRangeColumn+") AS year", "strftime('%m',"+timeRangeColumn+") AS month", "strftime('%d',"+timeRangeColumn+") AS day")
		groupBys = append([]string{"day", "month", "year"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC", "day ASC")
	}

	return
}

func generateMysqlTimeRangeClauses(timeRangeColumn, period string) (selectors []string, groupBys []string, orderBys []string) {
	switch period {
	case "month":
		selectors = append(selectors, "YEAR("+timeRangeColumn+") AS year", "MONTH("+timeRangeColumn+") AS month")
		groupBys = append([]string{"MONTH(" + timeRangeColumn + ")", "YEAR(" + timeRangeColumn + ")"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC")

	case "year":
		selectors = append(selectors, "YEAR("+timeRangeColumn+") AS year")
		groupBys = append([]string{"YEAR(" + timeRangeColumn + ")"}, groupBys...)
		orderBys = append(orderBys, "year ASC")

	case "day":
		fallthrough

	default:
		selectors = append(selectors, "YEAR("+timeRangeColumn+") AS year", "MONTH("+timeRangeColumn+") AS month", "DAY("+timeRangeColumn+") AS day")
		groupBys = append([]string{"DAY(" + timeRangeColumn + ")", "MONTH(" + timeRangeColumn + ")", "YEAR(" + timeRangeColumn + ")"}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC", "day ASC")
	}

	return
}

func generateMysqlTradingVolumeQuerySQL(tableName string, options TradingVolumeQueryOptions) string {
	timeRangeColumn := "traded_at"
	sel, groupBys, orderBys := generateMysqlTimeRangeClauses(timeRangeColumn, options.GroupByPeriod)

	switch options.SegmentBy {
	case "symbol":
		sel = append(sel, "symbol")
		groupBys = append([]string{"symbol"}, groupBys...)
		orderBys = append(orderBys, "symbol")
	case "exchange":
		sel = append(sel, "exchange")
		groupBys = append([]string{"exchange"}, groupBys...)
		orderBys = append(orderBys, "exchange")
	}

	sel = append(sel, "SUM(quantity * price) AS quote_volume")
	where := []string{timeRangeColumn + " > :start_time"}
	sql := `SELECT ` + strings.Join(sel, ", ") + ` FROM ` + tableName +
		` WHERE ` + strings.Join(where, " AND ") +
		` GROUP BY ` + strings.Join(groupBys, ", ") +
		` ORDER BY ` + strings.Join(orderBys, ", ")

	return sql
}

func (s *TradeService) QueryForTradingFeeCurrency(ex types.ExchangeName, symbol string, feeCurrency string) ([]types.Trade, error) {
	tableName := s.tableName("trades")
	sql := "SELECT " + strings.Join(genTradeSelectColumns(s.DB.DriverName()), ", ") + " FROM " + tableName + " WHERE exchange = :exchange AND (symbol = :symbol OR fee_currency = :fee_currency) ORDER BY traded_at ASC"
	rows, err := s.DB.NamedQuery(sql, map[string]interface{}{
		"exchange":     ex,
		"symbol":       symbol,
		"fee_currency": feeCurrency,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *TradeService) Query(options QueryTradesOptions) ([]types.Trade, error) {
	sel := sq.Select(genTradeSelectColumns(s.DB.DriverName())...).
		From(s.tableName("trades"))

	if options.LastGID != 0 {
		sel = sel.Where(sq.Gt{"gid": options.LastGID})
	}
	if options.Since != nil {
		sel = sel.Where(sq.GtOrEq{"traded_at": options.Since})
	}
	if options.Until != nil {
		sel = sel.Where(sq.Lt{"traded_at": options.Until})
	}

	if options.Symbol != "" {
		sel = sel.Where(sq.Eq{"symbol": options.Symbol})
	}

	if options.Strategy != "" {
		sel = sel.Where(sq.Or{
			sq.Eq{"strategy": options.Strategy},
			sq.Like{"strategy": options.Strategy + "-%"},
			sq.Like{"strategy": options.Strategy + ":%"},
		})
	}

	if options.StrategyInstanceID != "" {
		sel = sel.Where(sq.Eq{"strategy_instance_id": options.StrategyInstanceID})
	}

	if options.Exchange != "" {
		sel = sel.Where(sq.Eq{"exchange": options.Exchange})
	}

	if len(options.Sessions) > 0 {
		// FIXME: right now we only have the exchange field in the db, we might need to add the session field too.
		sel = sel.Where(sq.Eq{"exchange": options.Sessions})
	}

	var orderByColumn string
	switch options.OrderByColumn {
	case "":
		orderByColumn = "traded_at"
	case "traded_at", "gid":
		orderByColumn = options.OrderByColumn
	default:
		return nil, fmt.Errorf("invalid order by column: %s", options.OrderByColumn)
	}

	var ordering string

	switch strings.ToUpper(options.Ordering) {
	case "":
		ordering = "ASC"
	case "ASC", "DESC":
		ordering = strings.ToUpper(options.Ordering)
	default:
		return nil, fmt.Errorf("invalid ordering: %s", options.Ordering)
	}

	sel = sel.OrderBy(orderByColumn + " " + ordering)

	if options.Limit > 0 {
		sel = sel.Limit(options.Limit)
	}

	if options.IsMargin != nil {
		sel = sel.Where(sq.Eq{"is_margin": *options.IsMargin})
	}

	if options.IsFutures != nil {
		sel = sel.Where(sq.Eq{"is_futures": *options.IsFutures})
	}

	if options.IsIsolated != nil {
		sel = sel.Where(sq.Eq{"is_isolated": *options.IsIsolated})
	}

	sql, args, err := sel.ToSql()
	if err != nil {
		return nil, err
	}

	log.Debug(sql)
	log.Debug(args)

	rows, err := s.DB.Queryx(sql, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

// NetPosition returns the net position (total buy quantity - total sell quantity)
// for trades matching the given options. When opts.Until is set, only trades before
// that time are included.
func (s *TradeService) NetPosition(opts QueryTradesOptions) (float64, error) {
	castExpr := "CAST(quantity AS REAL)"
	if s.DB.DriverName() != "sqlite3" {
		castExpr = "CAST(quantity AS DECIMAL(30,18))"
	}

	netExpr := fmt.Sprintf(
		"COALESCE(SUM(CASE WHEN side = 'BUY' THEN %s ELSE -%s END), 0)",
		castExpr, castExpr,
	)

	sel := sq.Select(netExpr).From(s.tableName("trades"))

	if opts.Exchange != "" {
		sel = sel.Where(sq.Eq{"exchange": opts.Exchange})
	}
	if opts.Symbol != "" {
		sel = sel.Where(sq.Eq{"symbol": opts.Symbol})
	}
	if opts.Strategy != "" {
		sel = sel.Where(sq.Or{
			sq.Eq{"strategy": opts.Strategy},
			sq.Like{"strategy": opts.Strategy + "-%"},
			sq.Like{"strategy": opts.Strategy + ":%"},
		})
	}
	if opts.Until != nil {
		sel = sel.Where(sq.Lt{"traded_at": opts.Until})
	}
	if opts.Since != nil {
		sel = sel.Where(sq.GtOrEq{"traded_at": opts.Since})
	}

	sqlStr, args, err := sel.ToSql()
	if err != nil {
		return 0, err
	}

	var net float64
	if err := s.DB.QueryRow(sqlStr, args...).Scan(&net); err != nil {
		return 0, err
	}
	return net, nil
}

func (s *TradeService) Load(ctx context.Context, id int64) (*types.Trade, error) {
	var trade types.Trade
	query := "SELECT " + strings.Join(genTradeSelectColumns(s.DB.DriverName()), ", ") + " FROM " + s.tableName("trades") + " WHERE id = :id"
	rows, err := s.DB.NamedQueryContext(ctx, query, map[string]interface{}{
		"id": id,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		err = rows.StructScan(&trade)
		return &trade, err
	}

	return nil, errors.Wrapf(ErrTradeNotFound, "trade id:%d not found", id)
}

func queryTradesSQL(options QueryTradesOptions) string {
	ordering := "ASC"
	switch v := strings.ToUpper(options.Ordering); v {
	case "DESC", "ASC":
		ordering = v
	}

	var where []string

	if options.LastGID > 0 {
		switch ordering {
		case "ASC":
			where = append(where, "gid > :gid")
		case "DESC":
			where = append(where, "gid < :gid")
		}
	}

	if len(options.Symbol) > 0 {
		where = append(where, `symbol = :symbol`)
	}

	if len(options.Exchange) > 0 {
		where = append(where, `exchange = :exchange`)
	}

	sql := `SELECT * FROM trades`
	if len(where) > 0 {
		sql += ` WHERE ` + strings.Join(where, " AND ")
	}

	sql += ` ORDER BY gid ` + ordering

	if options.Limit > 0 {
		sql += ` LIMIT ` + strconv.FormatUint(options.Limit, 10)
	}

	return sql
}

func genTradeSelectColumns(driver string) []string {
	if driver != "mysql" {
		return []string{"*"}
	}
	tt := reflect.TypeOf(types.Trade{})
	var columns []string
	for i := 0; i < tt.NumField(); i++ {
		field := tt.Field(i)
		if colName := field.Tag.Get("db"); colName != "" {
			if colName == "-" {
				continue
			}
			if colName == "order_uuid" {
				columns = append(columns, binUuidSelector("trades", "order_uuid"))
			} else {
				columns = append(columns, colName)
			}
		}
	}
	return columns
}

func (s *TradeService) scanRows(rows *sqlx.Rows) (trades []types.Trade, err error) {
	for rows.Next() {
		var trade types.Trade
		if err := rows.StructScan(&trade); err != nil {
			return trades, err
		}

		trades = append(trades, trade)
	}

	return trades, rows.Err()
}

func (s *TradeService) Insert(trade types.Trade) error {
	tableName := s.tableName("trades")

	switch s.DB.DriverName() {
	case "mysql":
		_, err := s.DB.NamedExec(`
			INSERT INTO `+"`"+tableName+"`"+` (id, order_id, order_uuid, exchange, price, quantity, quote_quantity, symbol, side, is_buyer, is_maker, traded_at, fee, fee_currency, is_margin, is_futures, is_isolated, strategy, strategy_instance_id, pnl)
			VALUES (:id, :order_id, IF(:order_uuid != '', UUID_TO_BIN(:order_uuid, true), ''), :exchange, :price, :quantity, :quote_quantity, :symbol, :side, :is_buyer, :is_maker, :traded_at, :fee, :fee_currency, :is_margin, :is_futures, :is_isolated, :strategy, :strategy_instance_id, :pnl)
			ON DUPLICATE KEY UPDATE id=:id, order_id=:order_id, order_uuid=:order_uuid, exchange=:exchange, price=:price, quantity=:quantity, quote_quantity=:quote_quantity, symbol=:symbol, side=:side, is_buyer=:is_buyer, is_maker=:is_maker, traded_at=:traded_at, fee=:fee, fee_currency=:fee_currency, is_margin=:is_margin, is_futures=:is_futures, is_isolated=:is_isolated, strategy=:strategy, pnl=:pnl;`,
			trade)
		return err

	case "postgres":
		_, err := s.DB.NamedExec(`
			INSERT INTO "`+tableName+`" (trade_id, order_id, order_uuid, exchange, price, quantity, quote_quantity, symbol, side, is_buyer, is_maker, traded_at, fee, fee_currency, is_margin, is_futures, is_isolated, strategy, strategy_instance_id, pnl, user_id)
			VALUES (:trade_id, :order_id, :order_uuid, :exchange, :price, :quantity, :quote_quantity, :symbol, :side, :is_buyer, :is_maker, :traded_at, :fee, :fee_currency, :is_margin, :is_futures, :is_isolated, :strategy, :strategy_instance_id, :pnl, :user_id)
			ON CONFLICT (user_id, exchange, trade_id) DO UPDATE SET order_id=:order_id, order_uuid=:order_uuid, price=:price, quantity=:quantity, quote_quantity=:quote_quantity, symbol=:symbol, side=:side, is_buyer=:is_buyer, is_maker=:is_maker, traded_at=:traded_at, fee=:fee, fee_currency=:fee_currency, is_margin=:is_margin, is_futures=:is_futures, is_isolated=:is_isolated, strategy=:strategy, pnl=:pnl`,
			map[string]interface{}{
				"trade_id":             strconv.FormatUint(trade.ID, 10),
				"order_id":             strconv.FormatUint(trade.OrderID, 10),
				"order_uuid":           trade.OrderUUID,
				"exchange":             trade.Exchange,
				"price":                trade.Price,
				"quantity":             trade.Quantity,
				"quote_quantity":       trade.QuoteQuantity,
				"symbol":               trade.Symbol,
				"side":                 trade.Side,
				"is_buyer":             trade.IsBuyer,
				"is_maker":             trade.IsMaker,
				"traded_at":            trade.Time,
				"fee":                  trade.Fee,
				"fee_currency":         trade.FeeCurrency,
				"is_margin":            trade.IsMargin,
				"is_futures":           trade.IsFutures,
				"is_isolated":          trade.IsIsolated,
				"strategy":             trade.StrategyID,
				"strategy_instance_id": trade.StrategyInstanceID,
				"pnl":                  trade.PnL,
				"user_id":              s.UserID,
			})
		return err

	default: // sqlite3
		sql := dbCache.InsertSqlOf(trade)
		_, err := s.DB.NamedExec(sql, trade)
		return err
	}
}

// UpdateStrategy writes a trade's strategy field.
func (s *TradeService) UpdateStrategy(trade types.Trade) error {
	if s.DB == nil {
		return nil
	}
	tableName := s.tableName("trades")
	switch s.DB.DriverName() {
	case "postgres":
		_, err := s.DB.Exec("UPDATE \""+tableName+"\" SET strategy = $1, strategy_instance_id = $2 WHERE trade_id = $3", trade.StrategyID.String, trade.StrategyInstanceID, strconv.FormatUint(trade.ID, 10))
		return err
	default:
		_, err := s.DB.Exec("UPDATE "+tableName+" SET strategy = ?, strategy_instance_id = ? WHERE id = ?", trade.StrategyID.String, trade.StrategyInstanceID, trade.ID)
		return err
	}
}

func (s *TradeService) DeleteAll() error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.Exec(`DELETE FROM ` + s.tableName("trades"))
	return err
}

func SelectLastTrades(ex types.ExchangeName, symbol string, isMargin, isFutures, isIsolated bool, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("trades").
		Where(sq.And{
			sq.Eq{"symbol": symbol},
			sq.Eq{"exchange": ex},
			sq.Eq{"is_margin": isMargin},
			sq.Eq{"is_futures": isFutures},
			sq.Eq{"is_isolated": isIsolated},
		}).
		OrderBy("traded_at DESC").
		Limit(limit)
}
