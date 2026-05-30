package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/supabasetypes"
	"github.com/c9s/bbgo/pkg/types"
)

func TestSupabaseServiceInsertOrderMapping(t *testing.T) {
	svc := &SupabaseService{userID: "test-user-123"}

	order := types.Order{
		Exchange:         types.ExchangeBinance,
		OrderID:          12345,
		Status:           types.OrderStatusFilled,
		ExecutedQuantity: fixedpoint.NewFromFloat(0.1),
		IsWorking:        true,
		IsMargin:         true,
		IsFutures:        false,
		IsIsolated:       false,
		ActualOrderId:    99,
	}
	order.Symbol = "BTCUSDT"
	order.Side = types.SideTypeBuy
	order.Type = types.OrderTypeLimit
	order.Price = fixedpoint.NewFromFloat(50000.0)
	order.Quantity = fixedpoint.NewFromFloat(0.1)
	order.ClientOrderID = "client-123"
	order.StopPrice = fixedpoint.NewFromFloat(49000.0)
	order.TimeInForce = types.TimeInForceGTC
	order.UUID = "order-uuid-123"

	row := supabasetypes.PublicOrdersInsert{
		UserId:           svc.userID,
		OrderId:          "12345",
		Symbol:           order.Symbol,
		Side:             "BUY",
		Price:            "50000",
		Quantity:         "0.1",
		Status:           "filled",
		OrderType:        "limit",
		ExecutedQuantity: ptrStr("0.1"),
		Exchange:         ptrStr("binance"),
		ClientOrderId:    ptrStr("client-123"),
		TimeInForce:      ptrStr("GTC"),
		StopPrice:        ptrStr("49000"),
		IsWorking:        ptrBool(true),
		IsMargin:         ptrBool(true),
		IsFutures:        ptrBool(false),
		IsIsolated:       ptrBool(false),
		OrderUuid:        ptrStr("order-uuid-123"),
		ActualOrderId:    ptrInt64(99),
	}

	b, err := json.Marshal(row)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "test-user-123", m["user_id"])
	assert.Equal(t, "12345", m["order_id"])
	assert.Equal(t, "BTCUSDT", m["symbol"])
	assert.Equal(t, "BUY", m["side"])
	assert.Equal(t, "50000", m["price"])
	assert.Equal(t, "binance", m["exchange"])
	assert.Equal(t, true, m["is_margin"])
	assert.Equal(t, false, m["is_futures"])
}

func TestSupabaseServiceInsertTradeMapping(t *testing.T) {
	svc := &SupabaseService{userID: "test-user-456"}

	trade := types.Trade{
		ID:            999,
		OrderID:       12345,
		Symbol:        "ETHUSDT",
		Side:          types.SideTypeSell,
		Price:         fixedpoint.NewFromFloat(3000.0),
		Quantity:      fixedpoint.NewFromFloat(1.5),
		QuoteQuantity: fixedpoint.NewFromFloat(4500.0),
		Fee:           fixedpoint.NewFromFloat(0.001),
		FeeCurrency:   "USDT",
		Exchange:      types.ExchangeBinance,
		IsBuyer:       false,
		IsMaker:       true,
		IsMargin:      true,
		IsFutures:     true,
		IsIsolated:    false,
		OrderUUID:     "uuid-abc",
	}
	trade.PnL.Float64 = 42.5

	row := supabasetypes.PublicTradesInsert{
		UserId:        svc.userID,
		TradeId:       "999",
		OrderId:       "12345",
		Symbol:        trade.Symbol,
		Side:          "SELL",
		Price:         "3000",
		Quantity:      "1.5",
		Fee:           "0.001",
		FeeCurrency:   "USDT",
		QuoteQuantity: ptrStr("4500"),
		Exchange:      ptrStr("binance"),
		IsBuyer:       ptrBool(false),
		IsMaker:       ptrBool(true),
		IsMargin:      ptrBool(true),
		IsFutures:     ptrBool(true),
		IsIsolated:    ptrBool(false),
		OrderUuid:     ptrStr("uuid-abc"),
		Pnl:           ptrStr("42.5"),
	}

	b, err := json.Marshal(row)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "test-user-456", m["user_id"])
	assert.Equal(t, "999", m["trade_id"])
	assert.Equal(t, "ETHUSDT", m["symbol"])
	assert.Equal(t, "SELL", m["side"])
	assert.Equal(t, "uuid-abc", m["order_uuid"])
	assert.Equal(t, "42.5", m["pnl"])
	assert.Equal(t, true, m["is_margin"])
}

func TestSupabaseServiceInsertProfitMapping(t *testing.T) {
	svc := &SupabaseService{userID: "test-user-789"}

	profit := types.Profit{
		Strategy:        "grid2",
		Symbol:          "BTCUSDT",
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC",
		Profit:          fixedpoint.NewFromFloat(100.5),
		NetProfit:       fixedpoint.NewFromFloat(95.0),
		ProfitMargin:    fixedpoint.NewFromFloat(0.05),
		NetProfitMargin: fixedpoint.NewFromFloat(0.0475),
		TradeID:         42,
		IsBuyer:         false,
		IsMaker:         true,
		IsFutures:       false,
		FeeCurrency:     "USDT",
	}
	profit.Side = types.SideTypeSell
	profit.Price = fixedpoint.NewFromFloat(51000)
	profit.Quantity = fixedpoint.NewFromFloat(1.0)
	profit.QuoteQuantity = fixedpoint.NewFromFloat(51000)
	profit.Fee = fixedpoint.NewFromFloat(5.1)

	row := supabasetypes.PublicProfitsInsert{
		UserId:             svc.userID,
		Strategy:           profit.Strategy,
		Symbol:             profit.Symbol,
		Profit:             ptrStr("100.5"),
		NetProfit:          ptrStr("95"),
		TradeId:            int64(profit.TradeID),
		Side:               ptrStr("SELL"),
		Fee:                ptrStr("5.1"),
		FeeCurrency:        ptrStr(profit.FeeCurrency),
		QuoteCurrency:      ptrStr(profit.QuoteCurrency),
		BaseCurrency:       ptrStr(profit.BaseCurrency),
		IsBuyer:            ptrBool(profit.IsBuyer),
		IsMaker:            ptrBool(profit.IsMaker),
		IsFutures:          ptrBool(profit.IsFutures),
	}

	b, err := json.Marshal(row)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "test-user-789", m["user_id"])
	assert.Equal(t, "grid2", m["strategy"])
	assert.Equal(t, "BTCUSDT", m["symbol"])
	assert.Equal(t, "100.5", m["profit"])
	assert.Equal(t, int64(42), int64(m["trade_id"].(float64)))
}

func TestSupabaseServiceInsertPositionMapping(t *testing.T) {
	svc := &SupabaseService{userID: "test-user-pos"}

	pos := &types.Position{
		Strategy:        "xmaker",
		Symbol:          "ETHUSDT",
		QuoteCurrency:   "USDT",
		BaseCurrency:    "ETH",
		AverageCost:     fixedpoint.NewFromFloat(3000.0),
	}
	pos.Base = fixedpoint.NewFromFloat(2.5)
	pos.Quote = fixedpoint.NewFromFloat(7500.0)

	trade := types.Trade{ID: 77}
	trade.Exchange = types.ExchangeBinance
	trade.Side = types.SideTypeBuy

	profit := fixedpoint.NewFromFloat(50.0)
	netProfit := fixedpoint.NewFromFloat(45.0)

	row := supabasetypes.PublicPositionsInsert{
		UserId:             svc.userID,
		Strategy:           pos.Strategy,
		Symbol:             pos.Symbol,
		AverageCost:        ptrStr("3000"),
		Base:               ptrStr("2.5"),
		Quote:              ptrStr("7500"),
		Profit:             ptrStr(profit.String()),
		NetProfit:          ptrStr(netProfit.String()),
		TradeId:            int64(trade.ID),
		Exchange:           ptrStr("binance"),
		Side:               ptrStr("BUY"),
	}

	b, err := json.Marshal(row)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "test-user-pos", m["user_id"])
	assert.Equal(t, "xmaker", m["strategy"])
	assert.Equal(t, "ETHUSDT", m["symbol"])
	assert.Equal(t, "3000", m["average_cost"])
	assert.Equal(t, "50", m["profit"])
	assert.Equal(t, float64(77), m["trade_id"])
}

func TestSupabaseOrderRoundTrip(t *testing.T) {
	row := supabasetypes.PublicOrdersSelect{
		OrderId:          "12345",
		Symbol:           "BTCUSDT",
		Side:             "BUY",
		Price:            "50000",
		Quantity:         "0.1",
		Status:           "filled",
		OrderType:        "limit",
		ExecutedQuantity: ptrStr("0.1"),
		Exchange:         "binance",
		ClientOrderId:    "client-abc",
		TimeInForce:      "GTC",
		StopPrice:        "49000",
		IsWorking:        true,
		CreatedAt:        "2026-01-15T10:30:00Z",
		IsMargin:         true,
		IsFutures:        false,
		IsIsolated:       false,
		OrderUuid:        "uuid-123",
		ActualOrderId:    99,
	}

	got, err := supabaseOrderToOrder(row)
	require.NoError(t, err)
	assert.Equal(t, uint64(12345), got.OrderID)
	assert.Equal(t, "BTCUSDT", got.Symbol)
	assert.Equal(t, types.SideTypeBuy, got.Side)
	assert.Equal(t, types.OrderTypeLimit, got.Type)
	assert.Equal(t, types.OrderStatusFilled, got.Status)
	assert.Equal(t, types.ExchangeName("BINANCE"), got.Exchange)
	assert.Equal(t, "client-abc", got.ClientOrderID)
	assert.Equal(t, types.TimeInForceGTC, got.TimeInForce)
	assert.True(t, got.IsWorking)
	assert.True(t, got.IsMargin)
	assert.False(t, got.IsFutures)
	assert.Equal(t, uint64(99), got.ActualOrderId)
	assert.Equal(t, "50000", got.Price.String())
	assert.Equal(t, "0.1", got.Quantity.String())
	assert.Equal(t, "0.1", got.ExecutedQuantity.String())
	assert.Equal(t, "49000", got.StopPrice.String())
}

func TestSupabaseTradeRoundTrip(t *testing.T) {
	row := supabasetypes.PublicTradesSelect{
		TradeId:       "999",
		OrderId:       "12345",
		Symbol:        "ETHUSDT",
		Side:          "SELL",
		Price:         "3000",
		Quantity:      "1.5",
		QuoteQuantity: ptrStr("4500"),
		Exchange:      "binance",
		IsBuyer:       false,
		IsMaker:       true,
		TradedAt:      ptrStr("2026-01-15T12:00:00Z"),
		Fee:           "0.001",
		FeeCurrency:   "USDT",
		IsMargin:      true,
		IsFutures:     true,
		IsIsolated:    false,
		OrderUuid:     "uuid-abc",
	}

	got, err := supabaseTradeToTrade(row)
	require.NoError(t, err)
	assert.Equal(t, uint64(999), got.ID)
	assert.Equal(t, uint64(12345), got.OrderID)
	assert.Equal(t, "ETHUSDT", got.Symbol)
	assert.Equal(t, types.SideTypeSell, got.Side)
	assert.Equal(t, types.ExchangeName("BINANCE"), got.Exchange)
	assert.Equal(t, "3000", got.Price.String())
	assert.Equal(t, "1.5", got.Quantity.String())
	assert.Equal(t, "4500", got.QuoteQuantity.String())
	assert.False(t, got.IsBuyer)
	assert.True(t, got.IsMaker)
	assert.True(t, got.IsMargin)
	assert.True(t, got.IsFutures)
	assert.Equal(t, "uuid-abc", got.OrderUUID)
}

func TestAggregateTradingVolume(t *testing.T) {
	rows := []volumeTradeRow{
		{TradedAt: "2026-01-15T10:00:00Z", Symbol: "BTCUSDT", Exchange: "binance", Quantity: "1", Price: "50000"},
		{TradedAt: "2026-01-15T11:00:00Z", Symbol: "BTCUSDT", Exchange: "binance", Quantity: "0.5", Price: "51000"},
		{TradedAt: "2026-01-16T10:00:00Z", Symbol: "ETHUSDT", Exchange: "binance", Quantity: "10", Price: "3000"},
	}

	result := aggregateTradingVolume(rows, TradingVolumeQueryOptions{
		GroupByPeriod: "day",
		SegmentBy:     "symbol",
	})

	assert.Len(t, result, 2)

	var btcVol, ethVol float64
	for _, v := range result {
		if v.Symbol == "BTCUSDT" {
			btcVol += v.QuoteVolume
		}
		if v.Symbol == "ETHUSDT" {
			ethVol += v.QuoteVolume
		}
	}
	assert.InDelta(t, 75500.0, btcVol, 0.01)
	assert.InDelta(t, 30000.0, ethVol, 0.01)
}

func TestNewSupabaseServiceValidation(t *testing.T) {
	_, err := NewSupabaseService("", "key", "user")
	assert.Error(t, err)

	_, err = NewSupabaseService("https://example.supabase.co", "", "user")
	assert.Error(t, err)
}
