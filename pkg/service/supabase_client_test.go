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

func TestTablePrefix(t *testing.T) {
	svc := &SupabaseService{userID: "test", tablePrefix: ""}
	assert.Equal(t, "orders", svc.table("orders"))
	assert.Equal(t, "trades", svc.table("trades"))

	paperSvc := &SupabaseService{userID: "test", tablePrefix: "paper_"}
	assert.Equal(t, "paper_orders", paperSvc.table("orders"))
	assert.Equal(t, "paper_trades", paperSvc.table("trades"))
	assert.Equal(t, "paper_positions", paperSvc.table("positions"))
	assert.Equal(t, "paper_profits", paperSvc.table("profits"))
	assert.Equal(t, "paper_nav_history_details", paperSvc.table("nav_history_details"))
	assert.Equal(t, "paper_rewards", paperSvc.table("rewards"))
	assert.Equal(t, "paper_withdraws", paperSvc.table("withdraws"))
	assert.Equal(t, "paper_deposits", paperSvc.table("deposits"))
	assert.Equal(t, "paper_margin_loans", paperSvc.table("margin_loans"))
	assert.Equal(t, "paper_margin_repays", paperSvc.table("margin_repays"))
	assert.Equal(t, "paper_margin_interests", paperSvc.table("margin_interests"))
	assert.Equal(t, "paper_margin_liquidations", paperSvc.table("margin_liquidations"))
	assert.Equal(t, "paper_futures_position_risks", paperSvc.table("futures_position_risks"))
}

func mustMarshalToMap(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

func TestInsertNavHistoryMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-nav"}
	row := supabasetypes.PublicNavHistoryDetailsInsert{
		UserId:         svc.userID,
		Session:        ptrStr("main"),
		Exchange:       ptrStr("binance"),
		Subaccount:     ptrStr("spot"),
		Time:           ptrStr("2026-06-08T10:00:00Z"),
		Currency:       ptrStr("BTC"),
		NetAssetInUsd:  ptrStr("50000.5"),
		NetAssetInBtc:  ptrStr("1.2"),
		Balance:        ptrStr("0.5"),
		Available:      ptrStr("0.4"),
		Locked:         ptrStr("0.1"),
		Borrowed:       ptrStr("0.0"),
		NetAsset:       ptrStr("0.5"),
		PriceInUsd:     ptrStr("100001"),
		Interest:       ptrStr("0.001"),
		IsMargin:       ptrBool(false),
		IsIsolated:     ptrBool(false),
		IsolatedSymbol: ptrStr(""),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-nav", m["user_id"])
	assert.Equal(t, "main", m["session"])
	assert.Equal(t, "BTC", m["currency"])
	assert.Equal(t, "50000.5", m["net_asset_in_usd"])
	assert.Equal(t, false, m["is_margin"])
}

func TestInsertRewardMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-reward"}
	row := supabasetypes.PublicRewardsInsert{
		UserId:     svc.userID,
		Exchange:   ptrStr("binance"),
		Uuid:       ptrStr("reward-uuid-1"),
		RewardType: ptrStr("staking"),
		Currency:   ptrStr("BNB"),
		Quantity:   ptrStr("0.5"),
		State:      ptrStr("confirmed"),
		Note:       ptrStr(""),
		Spent:      ptrBool(false),
		CreatedAt:  ptrStr("2026-06-01T00:00:00Z"),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-reward", m["user_id"])
	assert.Equal(t, "reward-uuid-1", m["uuid"])
	assert.Equal(t, "staking", m["reward_type"])
	assert.Equal(t, "BNB", m["currency"])
	assert.Equal(t, "0.5", m["quantity"])
	assert.Equal(t, false, m["spent"])
}

func TestInsertWithdrawMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-wd"}
	row := supabasetypes.PublicWithdrawsInsert{
		UserId:         svc.userID,
		Exchange:       ptrStr("binance"),
		Asset:          ptrStr("USDT"),
		Network:        ptrStr("TRX"),
		Address:        ptrStr("TXYZ123"),
		Amount:         ptrStr("1000"),
		TxnId:          ptrStr("tx-001"),
		TxnFee:         ptrStr("1"),
		TxnFeeCurrency: ptrStr("USDT"),
		Time:           ptrStr("2026-06-08T10:00:00Z"),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-wd", m["user_id"])
	assert.Equal(t, "USDT", m["asset"])
	assert.Equal(t, "TRX", m["network"])
	assert.Equal(t, "tx-001", m["txn_id"])
	assert.Equal(t, "1", m["txn_fee"])
}

func TestInsertDepositMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-dep"}
	row := supabasetypes.PublicDepositsInsert{
		UserId:   svc.userID,
		Exchange: ptrStr("binance"),
		Asset:    ptrStr("BTC"),
		Address:  ptrStr("bc1abc"),
		Amount:   ptrStr("0.5"),
		TxnId:    ptrStr("tx-dep-001"),
		Time:     ptrStr("2026-06-08T10:00:00Z"),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-dep", m["user_id"])
	assert.Equal(t, "BTC", m["asset"])
	assert.Equal(t, "0.5", m["amount"])
	assert.Equal(t, "tx-dep-001", m["txn_id"])
}

func TestInsertMarginLoanMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-mloan"}
	row := supabasetypes.PublicMarginLoansInsert{
		UserId:         svc.userID,
		Exchange:       ptrStr("binance"),
		TransactionId:  ptrInt64(100001),
		Asset:          ptrStr("BTC"),
		IsolatedSymbol: ptrStr("BTCUSDT"),
		Principle:      ptrStr("0.1"),
		Time:           ptrStr("2026-06-08T10:00:00Z"),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-mloan", m["user_id"])
	assert.Equal(t, float64(100001), m["transaction_id"])
	assert.Equal(t, "BTC", m["asset"])
	assert.Equal(t, "BTCUSDT", m["isolated_symbol"])
	assert.Equal(t, "0.1", m["principle"])
}

func TestInsertMarginRepayMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-mrepay"}
	row := supabasetypes.PublicMarginRepaysInsert{
		UserId:         svc.userID,
		Exchange:       ptrStr("binance"),
		TransactionId:  ptrInt64(200001),
		Asset:          ptrStr("USDT"),
		IsolatedSymbol: ptrStr(""),
		Principle:      ptrStr("5000"),
		Time:           ptrStr("2026-06-08T10:00:00Z"),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-mrepay", m["user_id"])
	assert.Equal(t, float64(200001), m["transaction_id"])
	assert.Equal(t, "USDT", m["asset"])
	assert.Equal(t, "5000", m["principle"])
}

func TestInsertMarginInterestMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-mint"}
	row := supabasetypes.PublicMarginInterestsInsert{
		UserId:         svc.userID,
		Exchange:       ptrStr("binance"),
		Asset:          ptrStr("BTC"),
		IsolatedSymbol: ptrStr("BTCUSDT"),
		Principle:      ptrStr("0.1"),
		Interest:       ptrStr("0.0001"),
		InterestRate:   ptrStr("0.001"),
		Time:           ptrStr("2026-06-08T10:00:00Z"),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-mint", m["user_id"])
	assert.Equal(t, "BTC", m["asset"])
	assert.Equal(t, "0.0001", m["interest"])
	assert.Equal(t, "0.001", m["interest_rate"])
}

func TestInsertMarginLiquidationMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-mliq"}
	row := supabasetypes.PublicMarginLiquidationsInsert{
		UserId:           svc.userID,
		Exchange:         ptrStr("binance"),
		Symbol:           ptrStr("BTCUSDT"),
		Side:             ptrStr("SELL"),
		OrderId:          ptrInt64(300001),
		Price:            ptrStr("50000"),
		Quantity:         ptrStr("0.1"),
		AveragePrice:     ptrStr("49999"),
		ExecutedQuantity: ptrStr("0.1"),
		TimeInForce:      ptrStr("GTC"),
		IsIsolated:       ptrBool(true),
		Time:             ptrStr("2026-06-08T10:00:00Z"),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-mliq", m["user_id"])
	assert.Equal(t, "BTCUSDT", m["symbol"])
	assert.Equal(t, "SELL", m["side"])
	assert.Equal(t, float64(300001), m["order_id"])
	assert.Equal(t, "50000", m["price"])
	assert.Equal(t, true, m["is_isolated"])
}

func TestInsertPositionRiskMapping(t *testing.T) {
	svc := &SupabaseService{userID: "user-risk"}
	row := supabasetypes.PublicFuturesPositionRisksInsert{
		UserId:                 svc.userID,
		Exchange:               ptrStr("binance"),
		Symbol:                 ptrStr("ETHUSDT"),
		PositionSide:           ptrStr("LONG"),
		Leverage:               ptrStr("10"),
		LiquidationPrice:       ptrStr("2500"),
		EntryPrice:             ptrStr("3000"),
		MarkPrice:              ptrStr("3100"),
		BreakEvenPrice:         ptrStr("3050"),
		PositionAmount:         ptrStr("1"),
		UnrealizedPnl:          ptrStr("100"),
		Notional:               ptrStr("3100"),
		InitialMargin:          ptrStr("310"),
		MaintMargin:            ptrStr("155"),
		PositionInitialMargin:  ptrStr("310"),
		OpenOrderInitialMargin: ptrStr("0"),
		Adl:                    ptrStr("1"),
		MarginAsset:            ptrStr("USDT"),
		UpdatedAt:              ptrStr("2026-06-08T10:00:00Z"),
	}
	m := mustMarshalToMap(t, row)
	assert.Equal(t, "user-risk", m["user_id"])
	assert.Equal(t, "ETHUSDT", m["symbol"])
	assert.Equal(t, "LONG", m["position_side"])
	assert.Equal(t, "10", m["leverage"])
	assert.Equal(t, "2500", m["liquidation_price"])
	assert.Equal(t, "100", m["unrealized_pnl"])
	assert.Equal(t, "USDT", m["margin_asset"])
}
