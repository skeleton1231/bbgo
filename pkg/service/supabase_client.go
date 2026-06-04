package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/supabasetypes"
	"github.com/c9s/bbgo/pkg/types"
	postgrest "github.com/supabase-community/postgrest-go"
	supabase "github.com/supabase-community/supabase-go"
)

type SupabaseService struct {
	client *supabase.Client
	userID string
}

func NewSupabaseService(url, key, userID string) (*SupabaseService, error) {
	client, err := supabase.NewClient(url, key, &supabase.ClientOptions{})
	if err != nil {
		return nil, fmt.Errorf("create supabase client: %w", err)
	}
	return &SupabaseService{client: client, userID: userID}, nil
}

func (s *SupabaseService) InsertOrder(order types.Order) error {
	row := supabasetypes.PublicOrdersInsert{
		UserId:           s.userID,
		OrderId:          fmt.Sprintf("%d", order.OrderID),
		Symbol:           order.Symbol,
		Side:             order.Side.String(),
		Price:            order.Price.String(),
		Quantity:         order.Quantity.String(),
		Status:           string(order.Status),
		OrderType:        string(order.Type),
		ExecutedQuantity: ptrStr(order.ExecutedQuantity.String()),
		Exchange:         ptrStr(order.Exchange.String()),
		ClientOrderId:    ptrStr(order.ClientOrderID),
		TimeInForce:      ptrStr(string(order.TimeInForce)),
		StopPrice:        ptrStr(order.StopPrice.String()),
		IsWorking:        &order.IsWorking,
		UpdatedAt:        ptrStr(time.Now().UTC().Format(time.RFC3339Nano)),
		IsMargin:         &order.IsMargin,
		IsIsolated:       &order.IsIsolated,
		IsFutures:        &order.IsFutures,
		OrderUuid:        ptrStr(order.UUID),
		ActualOrderId:    ptrInt64(int64(order.ActualOrderId)),
	}
	_, _, err := s.client.From("orders").Upsert(row, "user_id,order_id", "", "").Execute()
	if err != nil {
		return fmt.Errorf("supabase upsert order: %w", err)
	}
	return nil
}

func (s *SupabaseService) InsertTrade(trade types.Trade) error {
	pnlStr := fmt.Sprintf("%v", trade.PnL.Float64)
	row := supabasetypes.PublicTradesInsert{
		UserId:        s.userID,
		TradeId:       fmt.Sprintf("%d", trade.ID),
		OrderId:       fmt.Sprintf("%d", trade.OrderID),
		Symbol:        trade.Symbol,
		Side:          trade.Side.String(),
		Price:         trade.Price.String(),
		Quantity:      trade.Quantity.String(),
		Fee:           trade.Fee.String(),
		FeeCurrency:   trade.FeeCurrency,
		QuoteQuantity: ptrStr(trade.QuoteQuantity.String()),
		TradedAt:      ptrStr(trade.Time.Time().Format(time.RFC3339Nano)),
		Exchange:      ptrStr(trade.Exchange.String()),
		IsBuyer:       &trade.IsBuyer,
		IsMaker:       &trade.IsMaker,
		IsMargin:      &trade.IsMargin,
		IsIsolated:    &trade.IsIsolated,
		IsFutures:     &trade.IsFutures,
		Strategy:      ptrStr(trade.StrategyID.String),
		OrderUuid:     ptrStr(trade.OrderUUID),
		Pnl:           &pnlStr,
	}
	_, _, err := s.client.From("trades").Upsert(row, "user_id,trade_id", "", "").Execute()
	if err != nil {
		return fmt.Errorf("supabase upsert trade: %w", err)
	}
	return nil
}

func (s *SupabaseService) InsertProfit(profit types.Profit) error {
	row := supabasetypes.PublicProfitsInsert{
		UserId:             s.userID,
		Strategy:           profit.Strategy,
		StrategyInstanceId: ptrStr(profit.StrategyInstanceID),
		Symbol:             profit.Symbol,
		AverageCost:        ptrStr(profit.AverageCost.String()),
		Profit:             ptrStr(profit.Profit.String()),
		NetProfit:          ptrStr(profit.NetProfit.String()),
		ProfitMargin:       ptrStr(profit.ProfitMargin.String()),
		NetProfitMargin:    ptrStr(profit.NetProfitMargin.String()),
		QuoteCurrency:      ptrStr(profit.QuoteCurrency),
		BaseCurrency:       ptrStr(profit.BaseCurrency),
		Exchange:           ptrStr(profit.Exchange.String()),
		IsFutures:          ptrBool(profit.IsFutures),
		IsMargin:           ptrBool(profit.IsMargin),
		IsIsolated:         ptrBool(profit.IsIsolated),
		TradeId:            int64(profit.TradeID),
		Side:               ptrStr(profit.Side.String()),
		IsBuyer:            ptrBool(profit.IsBuyer),
		IsMaker:            ptrBool(profit.IsMaker),
		Price:              ptrStr(profit.Price.String()),
		Quantity:           ptrStr(profit.Quantity.String()),
		QuoteQuantity:      ptrStr(profit.QuoteQuantity.String()),
		TradedAt:           profit.TradedAt.Format(time.RFC3339Nano),
		FeeInUsd:           ptrStr(profit.FeeInUSD.String()),
		Fee:                ptrStr(profit.Fee.String()),
		FeeCurrency:        ptrStr(profit.FeeCurrency),
	}
	_, _, err := s.client.From("profits").Upsert(row, "user_id,trade_id", "", "").Execute()
	if err != nil {
		return fmt.Errorf("supabase upsert profit: %w", err)
	}
	return nil
}

func (s *SupabaseService) InsertPosition(
	position *types.Position,
	trade types.Trade,
	profit, netProfit fixedpoint.Value,
) error {
	row := supabasetypes.PublicPositionsInsert{
		UserId:             s.userID,
		Strategy:           position.Strategy,
		StrategyInstanceId: ptrStr(position.StrategyInstanceID),
		Symbol:             position.Symbol,
		QuoteCurrency:      ptrStr(position.QuoteCurrency),
		BaseCurrency:       ptrStr(position.BaseCurrency),
		AverageCost:        ptrStr(position.AverageCost.String()),
		Base:               ptrStr(position.Base.String()),
		Quote:              ptrStr(position.Quote.String()),
		Profit:             ptrStr(profit.String()),
		NetProfit:          ptrStr(netProfit.String()),
		TradeId:            int64(trade.ID),
		Exchange:           ptrStr(trade.Exchange.String()),
		Side:               ptrStr(trade.Side.String()),
		TradedAt:           trade.Time.Time().Format(time.RFC3339Nano),
	}
	_, _, err := s.client.From("positions").Upsert(row, "user_id,trade_id,side,exchange", "", "").Execute()
	if err != nil {
		return fmt.Errorf("supabase upsert position: %w", err)
	}
	return nil
}

// --- Read methods ---

func (s *SupabaseService) QueryOrders(options QueryOrdersOptions) ([]AggOrder, error) {
	q := s.client.From("orders").Select("*", "", false).Eq("user_id", s.userID)

	if options.Exchange != "" {
		q = q.Eq("exchange", string(options.Exchange))
	}
	if options.Symbol != "" {
		q = q.Eq("symbol", options.Symbol)
	}

	ordering := strings.ToUpper(options.Ordering)
	if ordering == "" {
		ordering = "ASC"
	}
	q = q.Order("created_at", &postgrest.OrderOpts{Ascending: ordering == "ASC"}).Limit(500, "")

	result, _, err := q.Execute()
	if err != nil {
		return nil, fmt.Errorf("supabase query orders: %w", err)
	}

	var rows []supabasetypes.PublicOrdersSelect
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("supabase unmarshal orders: %w", err)
	}

	orders := make([]AggOrder, 0, len(rows))
	for _, row := range rows {
		order, err := supabaseOrderToOrder(row)
		if err != nil {
			return nil, fmt.Errorf("convert order %s: %w", row.OrderId, err)
		}
		orders = append(orders, AggOrder{Order: order})
	}
	return orders, nil
}

func (s *SupabaseService) QueryTrades(options QueryTradesOptions) ([]types.Trade, error) {
	q := s.client.From("trades").Select("*", "", false).Eq("user_id", s.userID)

	if options.Exchange != "" {
		q = q.Eq("exchange", string(options.Exchange))
	}
	if options.Symbol != "" {
		q = q.Eq("symbol", options.Symbol)
	}
	if options.Since != nil {
		q = q.Gte("traded_at", options.Since.Format(time.RFC3339Nano))
	}
	if options.Until != nil {
		q = q.Lt("traded_at", options.Until.Format(time.RFC3339Nano))
	}
	if options.IsMargin != nil {
		q = q.Eq("is_margin", strconv.FormatBool(*options.IsMargin))
	}
	if options.IsFutures != nil {
		q = q.Eq("is_futures", strconv.FormatBool(*options.IsFutures))
	}
	if options.IsIsolated != nil {
		q = q.Eq("is_isolated", strconv.FormatBool(*options.IsIsolated))
	}

	ordering := strings.ToUpper(options.Ordering)
	if ordering == "" {
		ordering = "ASC"
	}
	q = q.Order("traded_at", &postgrest.OrderOpts{Ascending: ordering == "ASC"})

	if options.Limit > 0 {
		q = q.Limit(int(options.Limit), "")
	} else {
		q = q.Limit(500, "")
	}

	result, _, err := q.Execute()
	if err != nil {
		return nil, fmt.Errorf("supabase query trades: %w", err)
	}

	var rows []supabasetypes.PublicTradesSelect
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("supabase unmarshal trades: %w", err)
	}

	trades := make([]types.Trade, 0, len(rows))
	for _, row := range rows {
		trade, err := supabaseTradeToTrade(row)
		if err != nil {
			return nil, fmt.Errorf("convert trade %s: %w", row.TradeId, err)
		}
		trades = append(trades, trade)
	}
	return trades, nil
}

func (s *SupabaseService) LoadTrade(id int64) (*types.Trade, error) {
	result, _, err := s.client.From("trades").
		Select("*", "", false).
		Eq("user_id", s.userID).
		Eq("trade_id", strconv.FormatInt(id, 10)).
		Limit(1, "").
		Execute()
	if err != nil {
		return nil, fmt.Errorf("supabase load trade: %w", err)
	}

	var rows []supabasetypes.PublicTradesSelect
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("supabase unmarshal trade: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("trade id:%d not found: %w", id, ErrTradeNotFound)
	}

	trade, err := supabaseTradeToTrade(rows[0])
	if err != nil {
		return nil, err
	}
	return &trade, nil
}

func (s *SupabaseService) QueryForTradingFeeCurrency(ex types.ExchangeName, symbol, feeCurrency string) ([]types.Trade, error) {
	q := s.client.From("trades").Select("*", "", false).
		Eq("user_id", s.userID).
		Eq("exchange", string(ex))

	q = q.Or(fmt.Sprintf("symbol.eq.%s,fee_currency.eq.%s", symbol, feeCurrency), "")
	q = q.Order("traded_at", &postgrest.OrderOpts{Ascending: true})

	result, _, err := q.Execute()
	if err != nil {
		return nil, fmt.Errorf("supabase query trades for fee currency: %w", err)
	}

	var rows []supabasetypes.PublicTradesSelect
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("supabase unmarshal trades: %w", err)
	}

	trades := make([]types.Trade, 0, len(rows))
	for _, row := range rows {
		trade, err := supabaseTradeToTrade(row)
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	return trades, nil
}

func (s *SupabaseService) QueryTradingVolume(startTime time.Time, options TradingVolumeQueryOptions) ([]TradingVolume, error) {
	q := s.client.From("trades").Select("traded_at,symbol,exchange,quantity,price", "", false).
		Eq("user_id", s.userID).
		Gte("traded_at", startTime.Format(time.RFC3339Nano)).
		Order("traded_at", &postgrest.OrderOpts{Ascending: true}).
		Limit(10000, "")

	result, _, err := q.Execute()
	if err != nil {
		return nil, fmt.Errorf("supabase query trading volume: %w", err)
	}

	var rows []volumeTradeRow
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("supabase unmarshal volume rows: %w", err)
	}

	return aggregateTradingVolume(rows, options), nil
}

func (s *SupabaseService) LoadProfit(id int64) (*types.Trade, error) {
	return s.LoadTrade(id)
}

func (s *SupabaseService) DeleteProfits(_ context.Context, options ProfitQueryOptions) error {
	q := s.client.From("profits").Delete("", "").Eq("user_id", s.userID)

	if options.Strategy != "" {
		q = q.Eq("strategy", options.Strategy)
	}
	if options.StrategyInstanceID != "" {
		q = q.Eq("strategy_instance_id", options.StrategyInstanceID)
	}
	if options.Symbol != "" {
		q = q.Eq("symbol", options.Symbol)
	}
	if !options.StartTime.IsZero() {
		q = q.Gte("traded_at", options.StartTime.Format(time.RFC3339Nano))
	}
	if !options.EndTime.IsZero() {
		q = q.Lte("traded_at", options.EndTime.Format(time.RFC3339Nano))
	}

	_, _, err := q.Execute()
	if err != nil {
		return fmt.Errorf("supabase delete profits: %w", err)
	}
	return nil
}

func (s *SupabaseService) LoadPosition(id int64) (*types.Position, error) {
	result, _, err := s.client.From("positions").
		Select("*", "", false).
		Eq("user_id", s.userID).
		Eq("trade_id", strconv.FormatInt(id, 10)).
		Limit(1, "").
		Execute()
	if err != nil {
		return nil, fmt.Errorf("supabase load position: %w", err)
	}

	var rows []supabasetypes.PublicPositionsSelect
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("supabase unmarshal position: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("position id:%d not found: %w", id, ErrTradeNotFound)
	}

	row := rows[0]
	pos := &types.Position{
		Symbol:             row.Symbol,
		QuoteCurrency:      row.QuoteCurrency,
		BaseCurrency:       row.BaseCurrency,
		AverageCost:        parseFixedPoint(row.AverageCost),
		Base:               parseFixedPoint(row.Base),
		Quote:              parseFixedPoint(row.Quote),
		Strategy:           row.Strategy,
		StrategyInstanceID: row.StrategyInstanceId,
	}
	return pos, nil
}

func (s *SupabaseService) DeletePositions(_ context.Context, options PositionQueryOptions) error {
	q := s.client.From("positions").Delete("", "").Eq("user_id", s.userID)

	if options.Strategy != "" {
		q = q.Eq("strategy", options.Strategy)
	}
	if options.StrategyInstanceID != "" {
		q = q.Eq("strategy_instance_id", options.StrategyInstanceID)
	}
	if options.Symbol != "" {
		q = q.Eq("symbol", options.Symbol)
	}
	if !options.StartTime.IsZero() {
		q = q.Gte("traded_at", options.StartTime.Format(time.RFC3339Nano))
	}
	if !options.EndTime.IsZero() {
		q = q.Lte("traded_at", options.EndTime.Format(time.RFC3339Nano))
	}

	_, _, err := q.Execute()
	if err != nil {
		return fmt.Errorf("supabase delete positions: %w", err)
	}
	return nil
}

// --- Conversion helpers ---

func supabaseOrderToOrder(row supabasetypes.PublicOrdersSelect) (types.Order, error) {
	orderID, err := strconv.ParseUint(row.OrderId, 10, 64)
	if err != nil {
		return types.Order{}, fmt.Errorf("parse order_id: %w", err)
	}

	var creationTime time.Time
	if row.CreatedAt != "" {
		creationTime, _ = time.Parse(time.RFC3339Nano, row.CreatedAt)
	}

	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: row.ClientOrderId,
			Symbol:        row.Symbol,
			Side:          types.SideType(strings.ToUpper(row.Side)),
			Type:          types.OrderType(strings.ToUpper(row.OrderType)),
			Price:         parseFixedPoint(row.Price),
			StopPrice:     parseFixedPoint(row.StopPrice),
			Quantity:      parseFixedPoint(row.Quantity),
			TimeInForce:   types.TimeInForce(strings.ToUpper(row.TimeInForce)),
		},
		Exchange:          types.ExchangeName(strings.ToUpper(row.Exchange)),
		OrderID:           orderID,
		UUID:              row.OrderUuid,
		Status:            types.OrderStatus(strings.ToUpper(row.Status)),
		ExecutedQuantity:  parseFixedPoint(derefStr(row.ExecutedQuantity)),
		IsWorking:         row.IsWorking,
		CreationTime:      types.Time(creationTime),
		IsMargin:          row.IsMargin,
		IsIsolated:        row.IsIsolated,
		IsFutures:         row.IsFutures,
		ActualOrderId:     uint64(row.ActualOrderId),
	}
	return order, nil
}

func supabaseTradeToTrade(row supabasetypes.PublicTradesSelect) (types.Trade, error) {
	tradeID, err := strconv.ParseUint(row.TradeId, 10, 64)
	if err != nil {
		return types.Trade{}, fmt.Errorf("parse trade_id: %w", err)
	}
	orderID, err := strconv.ParseUint(row.OrderId, 10, 64)
	if err != nil {
		return types.Trade{}, fmt.Errorf("parse order_id: %w", err)
	}

	var tradedAt time.Time
	if row.TradedAt != nil && *row.TradedAt != "" {
		tradedAt, _ = time.Parse(time.RFC3339Nano, *row.TradedAt)
	}

	return types.Trade{
		ID:            tradeID,
		OrderID:       orderID,
		OrderUUID:     row.OrderUuid,
		Exchange:      types.ExchangeName(strings.ToUpper(row.Exchange)),
		Price:         parseFixedPoint(row.Price),
		Quantity:      parseFixedPoint(row.Quantity),
		QuoteQuantity: parseFixedPoint(derefStr(row.QuoteQuantity)),
		Symbol:        row.Symbol,
		Side:          types.SideType(strings.ToUpper(row.Side)),
		IsBuyer:       row.IsBuyer,
		IsMaker:       row.IsMaker,
		Time:          types.Time(tradedAt),
		Fee:           parseFixedPoint(row.Fee),
		FeeCurrency:   row.FeeCurrency,
		IsMargin:      row.IsMargin,
		IsFutures:     row.IsFutures,
		IsIsolated:    row.IsIsolated,
	}, nil
}

type volumeTradeRow struct {
	TradedAt string `json:"traded_at"`
	Symbol   string `json:"symbol"`
	Exchange string `json:"exchange"`
	Quantity string `json:"quantity"`
	Price    string `json:"price"`
}

func aggregateTradingVolume(rows []volumeTradeRow, options TradingVolumeQueryOptions) []TradingVolume {
	type groupKey struct {
		year  int
		month int
		day   int
		extra string
	}

	volumes := make(map[groupKey]float64)

	for _, r := range rows {
		t, err := time.Parse(time.RFC3339Nano, r.TradedAt)
		if err != nil {
			continue
		}
		qty, err := strconv.ParseFloat(r.Quantity, 64)
		if err != nil {
			continue
		}
		prc, err := strconv.ParseFloat(r.Price, 64)
		if err != nil {
			continue
		}

		var k groupKey
		switch options.GroupByPeriod {
		case "year":
			k = groupKey{year: t.Year()}
		case "month":
			k = groupKey{year: t.Year(), month: int(t.Month())}
		default:
			k = groupKey{year: t.Year(), month: int(t.Month()), day: t.Day()}
		}

		switch options.SegmentBy {
		case "symbol":
			k.extra = r.Symbol
		case "exchange":
			k.extra = r.Exchange
		}

		volumes[k] += qty * prc
	}

	result := make([]TradingVolume, 0, len(volumes))
	for k, vol := range volumes {
		tv := TradingVolume{
			Year:        k.year,
			Month:       k.month,
			Day:         k.day,
			QuoteVolume: vol,
			Time:        time.Date(k.year, time.Month(k.month), k.day, 0, 0, 0, 0, time.Local),
		}
		switch options.SegmentBy {
		case "symbol":
			tv.Symbol = k.extra
		case "exchange":
			tv.Exchange = k.extra
		}
		result = append(result, tv)
	}
	return result
}

func parseFixedPoint(s string) fixedpoint.Value {
	v, _ := fixedpoint.NewFromString(s)
	return v
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// NetPosition returns the net position for trades matching the given options.
func (s *SupabaseService) NetPosition(opts QueryTradesOptions) (float64, error) {
	q := s.client.From("trades").Select("side,quantity", "", false).
		Eq("user_id", s.userID)

	if opts.Exchange != "" {
		q = q.Eq("exchange", string(opts.Exchange))
	}
	if opts.Symbol != "" {
		q = q.Eq("symbol", opts.Symbol)
	}
	if opts.Until != nil {
		q = q.Lt("traded_at", opts.Until.Format(time.RFC3339Nano))
	}
	if opts.Since != nil {
		q = q.Gte("traded_at", opts.Since.Format(time.RFC3339Nano))
	}

	result, _, err := q.Execute()
	if err != nil {
		return 0, fmt.Errorf("supabase net position: %w", err)
	}

	var rows []struct {
		Side     string `json:"side"`
		Quantity string `json:"quantity"`
	}
	if err := json.Unmarshal(result, &rows); err != nil {
		return 0, fmt.Errorf("supabase unmarshal net position: %w", err)
	}

	var net float64
	for _, r := range rows {
		qty, _ := strconv.ParseFloat(r.Quantity, 64)
		if r.Side == "BUY" {
			net += qty
		} else {
			net -= qty
		}
	}
	return net, nil
}

func ptrStr(s string) *string  { return &s }
func ptrBool(b bool) *bool     { return &b }
func ptrInt64(n int64) *int64  { return &n }
