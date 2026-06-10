package service

import (
	"context"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	sq "github.com/Masterminds/squirrel"
)

type PositionService struct {
	DB          *sqlx.DB
	TablePrefix string
	UserID      string
}

func (s *PositionService) tableName(base string) string { return s.TablePrefix + base }

func NewPositionService(db *sqlx.DB) *PositionService {
	return &PositionService{DB: db}
}

func (s *PositionService) Load(ctx context.Context, id int64) (*types.Position, error) {
	var pos types.Position

	rows, err := s.DB.NamedQueryContext(ctx, "SELECT * FROM "+s.tableName("positions")+" WHERE id = :id", map[string]interface{}{
		"id": id,
	})
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		err = rows.StructScan(&pos)
		return &pos, err
	}

	return nil, errors.Wrapf(ErrTradeNotFound, "position id:%d not found", id)
}

func (s *PositionService) Insert(
	position *types.Position,
	trade types.Trade,
	profit, netProfit fixedpoint.Value,
) error {
	tableName := s.tableName("positions")
	args := map[string]interface{}{
		"user_id":              s.UserID,
		"strategy":             position.Strategy,
		"strategy_instance_id": position.StrategyInstanceID,
		"symbol":               position.Symbol,
		"quote_currency":       position.QuoteCurrency,
		"base_currency":        position.BaseCurrency,
		"average_cost":         position.AverageCost,
		"base":                 position.Base,
		"quote":                position.Quote,
		"profit":               profit,
		"net_profit":           netProfit,
		"trade_id":             strconv.FormatUint(trade.ID, 10),
		"exchange":             trade.Exchange,
		"side":                 trade.Side,
		"traded_at":            trade.Time,
	}

	var sql string
	switch s.DB.DriverName() {
	case "postgres":
		sql = `INSERT INTO "` + tableName + `" (
			strategy, strategy_instance_id, symbol, quote_currency, base_currency, average_cost,
			base, quote, profit, net_profit, trade_id, exchange, side, traded_at, user_id
		) VALUES (
			:strategy, :strategy_instance_id, :symbol, :quote_currency, :base_currency, :average_cost,
			:base, :quote, :profit, :net_profit, :trade_id, :exchange, :side, :traded_at, :user_id
		) ON CONFLICT (user_id, trade_id, side, symbol, exchange) DO NOTHING`
	default: // mysql, sqlite3
		sql = `INSERT OR IGNORE INTO ` + tableName + ` (
			strategy, strategy_instance_id, symbol, quote_currency, base_currency, average_cost,
			base, quote, profit, net_profit, trade_id, exchange, side, traded_at
		) VALUES (
			:strategy, :strategy_instance_id, :symbol, :quote_currency, :base_currency, :average_cost,
			:base, :quote, :profit, :net_profit, :trade_id, :exchange, :side, :traded_at
		)`
	}

	_, err := s.DB.NamedExec(sql, args)
	return err
}

type PositionQueryOptions struct {
	Strategy           string
	StrategyInstanceID string
	Symbol             string
	StartTime          time.Time // inclusive
	EndTime            time.Time // inclusive
}

func (s *PositionService) Delete(ctx context.Context, options PositionQueryOptions) error {
	del := sq.Delete(s.tableName("positions"))
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
