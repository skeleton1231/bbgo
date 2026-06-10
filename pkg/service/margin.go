package service

import (
	"context"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

type MarginService struct {
	DB          *sqlx.DB
	TablePrefix string
	UserID      string
}

func (s *MarginService) tableName(base string) string { return s.TablePrefix + base }

func (s *MarginService) InsertLoan(loan types.MarginLoan) error {
	tableName := s.tableName("margin_loans")
	if s.DB.DriverName() == "postgres" {
		_, err := s.DB.NamedExec(`INSERT INTO "`+tableName+`" (exchange, transaction_id, asset, isolated_symbol, principle, time, user_id)
			VALUES (:exchange, :transaction_id, :asset, :isolated_symbol, :principle, :time, :user_id)
			ON CONFLICT (user_id, transaction_id) DO NOTHING`,
			map[string]interface{}{
				"exchange":        loan.Exchange,
				"transaction_id":  loan.TransactionID,
				"asset":           loan.Asset,
				"isolated_symbol": loan.IsolatedSymbol,
				"principle":       loan.Principle,
				"time":            loan.Time,
				"user_id":         s.UserID,
			})
		return err
	}
	_, err := s.DB.NamedExec(`INSERT INTO `+tableName+` (exchange, transaction_id, asset, isolated_symbol, principle, time)
			VALUES (:exchange, :transaction_id, :asset, :isolated_symbol, :principle, :time)`, loan)
	return err
}

func (s *MarginService) InsertRepay(repay types.MarginRepay) error {
	tableName := s.tableName("margin_repays")
	if s.DB.DriverName() == "postgres" {
		_, err := s.DB.NamedExec(`INSERT INTO "`+tableName+`" (exchange, transaction_id, asset, isolated_symbol, principle, time, user_id)
			VALUES (:exchange, :transaction_id, :asset, :isolated_symbol, :principle, :time, :user_id)
			ON CONFLICT (user_id, transaction_id) DO NOTHING`,
			map[string]interface{}{
				"exchange":        repay.Exchange,
				"transaction_id":  repay.TransactionID,
				"asset":           repay.Asset,
				"isolated_symbol": repay.IsolatedSymbol,
				"principle":       repay.Principle,
				"time":            repay.Time,
				"user_id":         s.UserID,
			})
		return err
	}
	_, err := s.DB.NamedExec(`INSERT INTO `+tableName+` (exchange, transaction_id, asset, isolated_symbol, principle, time)
			VALUES (:exchange, :transaction_id, :asset, :isolated_symbol, :principle, :time)`, repay)
	return err
}

func (s *MarginService) InsertInterest(interest types.MarginInterest) error {
	tableName := s.tableName("margin_interests")
	if s.DB.DriverName() == "postgres" {
		_, err := s.DB.NamedExec(`INSERT INTO "`+tableName+`" (exchange, asset, isolated_symbol, principle, interest, interest_rate, time, user_id)
			VALUES (:exchange, :asset, :isolated_symbol, :principle, :interest, :interest_rate, :time, :user_id)`,
			map[string]interface{}{
				"exchange":        interest.Exchange,
				"asset":           interest.Asset,
				"isolated_symbol": interest.IsolatedSymbol,
				"principle":       interest.Principle,
				"interest":        interest.Interest,
				"interest_rate":   interest.InterestRate,
				"time":            interest.Time,
				"user_id":         s.UserID,
			})
		return err
	}
	_, err := s.DB.NamedExec(`INSERT INTO `+tableName+` (exchange, asset, isolated_symbol, principle, interest, interest_rate, time)
			VALUES (:exchange, :asset, :isolated_symbol, :principle, :interest, :interest_rate, :time)`, interest)
	return err
}

func (s *MarginService) InsertLiquidation(liquidation types.MarginLiquidation) error {
	tableName := s.tableName("margin_liquidations")
	if s.DB.DriverName() == "postgres" {
		_, err := s.DB.NamedExec(`INSERT INTO "`+tableName+`" (exchange, symbol, side, order_id, price, quantity, average_price, executed_quantity, time_in_force, is_isolated, time, user_id)
			VALUES (:exchange, :symbol, :side, :order_id, :price, :quantity, :average_price, :executed_quantity, :time_in_force, :is_isolated, :time, :user_id)
			ON CONFLICT (user_id, order_id, exchange) DO NOTHING`,
			map[string]interface{}{
				"exchange":          liquidation.Exchange,
				"symbol":            liquidation.Symbol,
				"side":              liquidation.Side,
				"order_id":          liquidation.OrderID,
				"price":             liquidation.Price,
				"quantity":          liquidation.Quantity,
				"average_price":     liquidation.AveragePrice,
				"executed_quantity": liquidation.ExecutedQuantity,
				"time_in_force":     liquidation.TimeInForce,
				"is_isolated":       liquidation.IsIsolated,
				"time":              liquidation.UpdatedTime,
				"user_id":           s.UserID,
			})
		return err
	}
	_, err := s.DB.NamedExec(`INSERT INTO `+tableName+` (exchange, symbol, side, order_id, price, quantity, average_price, executed_quantity, time_in_force, is_isolated, time)
			VALUES (:exchange, :symbol, :side, :order_id, :price, :quantity, :average_price, :executed_quantity, :time_in_force, :is_isolated, :time)`, liquidation)
	return err
}

func (s *MarginService) Sync(ctx context.Context, ex types.Exchange, asset string, startTime time.Time) error {
	if s.DB == nil {
		return nil
	}
	api, ok := ex.(types.MarginHistoryService)
	if !ok {
		return nil
	}

	marginExchange, ok := ex.(types.MarginExchange)
	if !ok {
		return nil
	}

	marginSettings := marginExchange.GetMarginSettings()
	if !marginSettings.IsMargin {
		return nil
	}

	tasks := []SyncTask{
		{
			Select: SelectLastMarginLoans(ex.Name(), asset, 100),
			Type:   types.MarginLoan{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginLoanBatchQuery{
					MarginHistoryService: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginLoan).Time.Time()
			},
			ID: func(obj interface{}) string {
				return strconv.FormatUint(obj.(types.MarginLoan).TransactionID, 10)
			},
			LogInsert: true,
		},
		{
			Select: SelectLastMarginRepays(ex.Name(), asset, 100),
			Type:   types.MarginRepay{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginRepayBatchQuery{
					MarginHistoryService: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginRepay).Time.Time()
			},
			ID: func(obj interface{}) string {
				return strconv.FormatUint(obj.(types.MarginRepay).TransactionID, 10)
			},
			LogInsert: true,
		},
		{
			Select: SelectLastMarginInterests(ex.Name(), asset, 100),
			Type:   types.MarginInterest{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginInterestBatchQuery{
					MarginHistoryService: api,
				}
				return query.Query(ctx, asset, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginInterest).Time.Time()
			},
			ID: func(obj interface{}) string {
				m := obj.(types.MarginInterest)
				return m.Asset + m.IsolatedSymbol + strconv.FormatInt(m.Time.UnixMilli(), 10)
			},
			LogInsert: true,
		},
		{
			Select: SelectLastMarginLiquidations(ex.Name(), 100),
			Type:   types.MarginLiquidation{},
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.MarginLiquidationBatchQuery{
					MarginHistoryService: api,
				}
				return query.Query(ctx, startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.MarginLiquidation).UpdatedTime.Time()
			},
			ID: func(obj interface{}) string {
				m := obj.(types.MarginLiquidation)
				return strconv.FormatUint(m.OrderID, 10)
			},
			LogInsert: true,
		},
	}

	for _, sel := range tasks {
		if err := sel.execute(ctx, s.DB, startTime); err != nil {
			return err
		}
	}

	return nil
}

func SelectLastMarginLoans(ex types.ExchangeName, asset string, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_loans").
		Where(sq.Eq{"exchange": ex, "asset": asset}).
		OrderBy("time DESC").
		Limit(limit)
}

func SelectLastMarginRepays(ex types.ExchangeName, asset string, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_repays").
		Where(sq.Eq{"exchange": ex, "asset": asset}).
		OrderBy("time DESC").
		Limit(limit)
}

func SelectLastMarginInterests(ex types.ExchangeName, asset string, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_interests").
		Where(sq.Eq{"exchange": ex, "asset": asset}).
		OrderBy("time DESC").
		Limit(limit)
}

func SelectLastMarginLiquidations(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("margin_liquidations").
		Where(sq.Eq{"exchange": ex}).
		OrderBy("time DESC").
		Limit(limit)
}
