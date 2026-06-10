package service

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/types"
)

type ProfitService struct {
	DB          *sqlx.DB
	TablePrefix string
	UserID      string
}

func (s *ProfitService) tableName(base string) string { return s.TablePrefix + base }

func (s *ProfitService) Load(ctx context.Context, id int64) (*types.Trade, error) {
	var trade types.Trade

	rows, err := s.DB.NamedQueryContext(ctx, "SELECT * FROM "+s.tableName("trades")+" WHERE id = :id", map[string]interface{}{
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

func (s *ProfitService) Insert(profit types.Profit) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tableName := s.tableName("profits")

	if s.DB.DriverName() == "postgres" {
		args := map[string]interface{}{
			"strategy":             profit.Strategy,
			"strategy_instance_id": profit.StrategyInstanceID,
			"symbol":               profit.Symbol,
			"quote_currency":       profit.QuoteCurrency,
			"base_currency":        profit.BaseCurrency,
			"average_cost":         profit.AverageCost,
			"profit":               profit.Profit,
			"net_profit":           profit.NetProfit,
			"profit_margin":        profit.ProfitMargin,
			"net_profit_margin":    profit.NetProfitMargin,
			"trade_id":             profit.TradeID,
			"price":                profit.Price,
			"quantity":             profit.Quantity,
			"quote_quantity":       profit.QuoteQuantity,
			"side":                 profit.Side,
			"is_buyer":             profit.IsBuyer,
			"is_maker":             profit.IsMaker,
			"fee":                  profit.Fee,
			"fee_currency":         profit.FeeCurrency,
			"fee_in_usd":           profit.FeeInUSD,
			"traded_at":            profit.TradedAt,
			"exchange":             profit.Exchange,
			"is_margin":            profit.IsMargin,
			"is_futures":           profit.IsFutures,
			"is_isolated":          profit.IsIsolated,
			"user_id":              s.UserID,
		}
		sql := `INSERT INTO "` + tableName + `" (
			strategy, strategy_instance_id, symbol, quote_currency, base_currency, average_cost,
			profit, net_profit, profit_margin, net_profit_margin, trade_id, price, quantity,
			quote_quantity, side, is_buyer, is_maker, fee, fee_currency, fee_in_usd,
			traded_at, exchange, is_margin, is_futures, is_isolated, user_id
		) VALUES (
			:strategy, :strategy_instance_id, :symbol, :quote_currency, :base_currency, :average_cost,
			:profit, :net_profit, :profit_margin, :net_profit_margin, :trade_id, :price, :quantity,
			:quote_quantity, :side, :is_buyer, :is_maker, :fee, :fee_currency, :fee_in_usd,
			:traded_at, :exchange, :is_margin, :is_futures, :is_isolated, :user_id
		) ON CONFLICT (user_id, exchange, symbol, side, trade_id) DO NOTHING`
		_, err := s.DB.NamedExecContext(ctx, sql, args)
		return err
	}

	sql := `
		INSERT INTO ` + tableName + ` (
			strategy, strategy_instance_id, symbol, quote_currency, base_currency, average_cost,
			profit, net_profit, profit_margin, net_profit_margin, trade_id, price, quantity,
			quote_quantity, side, is_buyer, is_maker, fee, fee_currency, fee_in_usd,
			traded_at, exchange, is_margin, is_futures, is_isolated
		) VALUES (
			:strategy, :strategy_instance_id, :symbol, :quote_currency, :base_currency, :average_cost,
			:profit, :net_profit, :profit_margin, :net_profit_margin, :trade_id, :price, :quantity,
			:quote_quantity, :side, :is_buyer, :is_maker, :fee, :fee_currency, :fee_in_usd,
			:traded_at, :exchange, :is_margin, :is_futures, :is_isolated
		)`

	_, err := s.DB.NamedExecContext(ctx, sql, profit)
	return err
}

type ProfitQueryOptions struct {
	Strategy           string
	StrategyInstanceID string
	Symbol             string
	StartTime          time.Time // inclusive
	EndTime            time.Time // inclusive
}

func (s *ProfitService) Delete(ctx context.Context, options ProfitQueryOptions) error {
	del := sq.Delete(s.tableName("profits"))
	if options.Strategy != "" {
		del = del.Where(sq.Eq{"strategy": options.Strategy})
	}
	if options.StrategyInstanceID != "" {
		del = del.Where(sq.Eq{"strategy_instance_id": options.StrategyInstanceID})
	}
	if options.Symbol != "" {
		del = del.Where(sq.Eq{"symbol": options.Symbol})
	}
	if !options.StartTime.IsZero() {
		del = del.Where(sq.GtOrEq{"traded_at": options.StartTime})
	}
	if !options.EndTime.IsZero() {
		del = del.Where(sq.LtOrEq{"traded_at": options.EndTime})
	}
	sql, args, err := del.ToSql()
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, sql, args...)
	return err
}
