package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/c9s/bbgo/pkg/types"
)

type FuturesService struct {
	DB                         *sqlx.DB
	TablePrefix                string
	PositionRiskUpdateInterval time.Duration
	UserID                     string

	positionRiskLastUpdateTime map[string]time.Time
}

func (s *FuturesService) tableName(base string) string { return s.TablePrefix + base }

func NewFuturesService(db *sqlx.DB) *FuturesService {
	return &FuturesService{
		DB:                         db,
		positionRiskLastUpdateTime: make(map[string]time.Time),
	}
}

func (s *FuturesService) QueryPositionsAndInsert(
	ctx context.Context, exchange types.ExchangeRiskService, currentTime time.Time, symbol ...string) error {
	symbolStr := "*"
	if len(symbol) > 0 {
		sort.Slice(symbol, func(i, j int) bool {
			return symbol[i] < symbol[j]
		})
		symbolStr = strings.Join(symbol, ",")
	}
	var lastUpdateTime time.Time
	if updateTime, ok := s.positionRiskLastUpdateTime[symbolStr]; ok {
		lastUpdateTime = updateTime
	}

	if !lastUpdateTime.IsZero() {
		if currentTime.Before(lastUpdateTime) {
			return nil
		}

		if s.PositionRiskUpdateInterval != 0 && currentTime.Sub(lastUpdateTime) < s.PositionRiskUpdateInterval {
			return nil
		}
	}
	s.positionRiskLastUpdateTime[symbolStr] = currentTime

	risks, err := exchange.QueryPositionRisk(ctx, symbol...)
	if err != nil {
		return fmt.Errorf("failed to query %s position risk: %w", symbol, err)
	}

	for _, risk := range risks {
		risk.UpdateTime = types.MillisecondTimestamp(time.Now())
		if err := s.Insert(risk); err != nil {
			return fmt.Errorf("failed to insert position risk (%+v): %w", risk, err)
		}
	}

	return nil
}

type QueryFuturesPositionRiskOptions struct {
	Exchange string
	Symbol   string
}

func (s *FuturesService) Sync(
	ctx context.Context, service types.ExchangeRiskService, symbol string,
) error {
	if s.DB == nil {
		return nil
	}
	risks, err := service.QueryPositionRisk(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to query position risk: %w", err)
	}
	if len(risks) == 0 {
		return nil
	}

	risk := risks[0]
	risk.UpdateTime = types.MillisecondTimestamp(time.Now())
	if err := s.Insert(risk); err != nil {
		return fmt.Errorf("failed to insert position risk (%+v): %w", risk, err)
	}
	return nil
}

func (s *FuturesService) Query(options QueryFuturesPositionRiskOptions) ([]types.PositionRisk, error) {
	tableName := s.tableName("futures_position_risks")
	builder := sq.
		Select("*").
		From(tableName).
		Where(sq.Eq{"exchange": options.Exchange, "symbol": options.Symbol}).
		OrderBy("updated_at DESC")
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.NamedQuery(sql, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var risks []types.PositionRisk
	for rows.Next() {
		var risk types.PositionRisk
		if err := rows.StructScan(&risk); err != nil {
			return nil, err
		}

		risks = append(risks, risk)
	}

	return risks, nil
}

func (s *FuturesService) Insert(risk types.PositionRisk) (err error) {
	tableName := s.tableName("futures_position_risks")

	switch s.DB.DriverName() {
	case "mysql":
		sql := `INSERT INTO ` + tableName + ` (
			exchange, symbol, position_side, entry_price, leverage, liquidation_price,
			mark_price, break_even_price, unrealized_pnl, notional, initial_margin, maint_margin,
			position_initial_margin, open_order_initial_margin, adl, margin_asset,
			position_amount, updated_at
		) VALUES (
			:exchange, :symbol, :position_side, :entry_price, :leverage, :liquidation_price,
			:mark_price, :break_even_price, :unrealized_pnl, :notional, :initial_margin, :maint_margin,
			:position_initial_margin, :open_order_initial_margin, :adl, :margin_asset,
			:position_amount, :updated_at
		) ON DUPLICATE KEY UPDATE
			entry_price=:entry_price, leverage=:leverage, liquidation_price=:liquidation_price,
			mark_price=:mark_price, break_even_price=:break_even_price,
			unrealized_pnl=:unrealized_pnl, notional=:notional,
			initial_margin=:initial_margin, maint_margin=:maint_margin,
			position_initial_margin=:position_initial_margin,
			open_order_initial_margin=:open_order_initial_margin,
			adl=:adl, margin_asset=:margin_asset,
			position_amount=:position_amount, updated_at=:updated_at`
		_, err = s.DB.NamedExec(sql, risk)

	case "postgres":
		_, err = s.DB.NamedExec(`INSERT INTO "`+tableName+`" (
			exchange, symbol, position_side, entry_price, leverage, liquidation_price,
			mark_price, break_even_price, unrealized_pnl, notional, initial_margin, maint_margin,
			position_initial_margin, open_order_initial_margin, adl, margin_asset,
			position_amount, updated_at, strategy_instance_id, user_id
		) VALUES (
			:exchange, :symbol, :position_side, :entry_price, :leverage, :liquidation_price,
			:mark_price, :break_even_price, :unrealized_pnl, :notional, :initial_margin, :maint_margin,
			:position_initial_margin, :open_order_initial_margin, :adl, :margin_asset,
			:position_amount, :updated_at, :strategy_instance_id, :user_id
	)`,
			map[string]interface{}{
				"exchange":                  risk.Exchange,
				"symbol":                    risk.Symbol,
				"position_side":             risk.PositionSide,
				"entry_price":               risk.EntryPrice,
				"leverage":                  risk.Leverage,
				"liquidation_price":         risk.LiquidationPrice,
				"mark_price":                risk.MarkPrice,
				"break_even_price":          risk.BreakEvenPrice,
				"unrealized_pnl":            risk.UnrealizedPnL,
				"notional":                  risk.Notional,
				"initial_margin":            risk.InitialMargin,
				"maint_margin":              risk.MaintMargin,
				"position_initial_margin":   risk.PositionInitialMargin,
				"open_order_initial_margin": risk.OpenOrderInitialMargin,
				"adl":                       risk.Adl,
				"margin_asset":              risk.MarginAsset,
				"position_amount":           risk.PositionAmount,
				"updated_at":                risk.UpdateTime.Time(),
			"strategy_instance_id":     risk.StrategyInstanceID,
				"user_id":                   s.UserID,
			})

	default: // sqlite3
		sql := `INSERT INTO ` + tableName + ` (
			exchange, symbol, position_side, entry_price, leverage, liquidation_price,
			mark_price, break_even_price, unrealized_pnl, notional, initial_margin, maint_margin,
			position_initial_margin, open_order_initial_margin, adl, margin_asset,
			position_amount, updated_at
		) VALUES (
			:exchange, :symbol, :position_side, :entry_price, :leverage, :liquidation_price,
			:mark_price, :break_even_price, :unrealized_pnl, :notional, :initial_margin, :maint_margin,
			:position_initial_margin, :open_order_initial_margin, :adl, :margin_asset,
			:position_amount, :updated_at
		)`
		_, err = s.DB.NamedExec(sql, risk)
	}

	return err
}
