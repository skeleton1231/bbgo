package bbgo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func newTestPaperTradeExchange() *PaperTradeExchange {
	inner := binance.New("key", "secret")
	markets := types.MarketMap{
		"BTCUSDT": {
			BaseCurrency:  "BTC",
			QuoteCurrency: "USDT",
			Symbol:        "BTCUSDT",
		},
		"ETHUSDT": {
			BaseCurrency:  "ETH",
			QuoteCurrency: "USDT",
			Symbol:        "ETHUSDT",
		},
	}
	balances := types.BalanceMap{
		"USDT": {Currency: "USDT", Available: fixedpoint.NewFromFloat(10000.0)},
	}
	return NewPaperTradeExchange(inner, markets, balances)
}

func TestPaperTradeExchange_UseFutures(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	settings := e.GetFuturesSettings()
	assert.True(t, settings.IsFutures)
	assert.False(t, settings.IsIsolatedFutures)
}

func TestPaperTradeExchange_UseIsolatedFutures(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseIsolatedFutures("BTCUSDT")

	settings := e.GetFuturesSettings()
	assert.True(t, settings.IsFutures)
	assert.True(t, settings.IsIsolatedFutures)
	assert.Equal(t, "BTCUSDT", settings.IsolatedFuturesSymbol)
}

func TestPaperTradeExchange_SetLeverage(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	err := e.SetLeverage(context.Background(), "BTCUSDT", 10)
	require.NoError(t, err)

	state := e.getOrCreateFuturesState("BTCUSDT")
	assert.Equal(t, 10, state.Leverage)
}

func TestPaperTradeExchange_QueryPositionRisk_Empty(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	assert.Empty(t, risks)
}

func TestPaperTradeExchange_QueryPositionRisk_AfterTrade(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 20)

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, risks, 1)

	r := risks[0]
	assert.Equal(t, "BTCUSDT", r.Symbol)
	assert.Equal(t, types.PositionLong, r.PositionSide)
	assert.True(t, r.PositionAmount.Compare(fixedpoint.NewFromFloat(1.0)) == 0)
	assert.True(t, r.EntryPrice.Compare(fixedpoint.NewFromFloat(50000.0)) == 0)
	assert.True(t, r.Leverage.Compare(fixedpoint.NewFromInt(20)) == 0)
	assert.True(t, r.Notional.Compare(fixedpoint.NewFromFloat(50000.0)) == 0)
}

func TestPaperTradeExchange_ShortPosition(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 10)

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeSell, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	state := e.futuresStates["BTCUSDT"]
	assert.True(t, state.PositionAmount.Sign() < 0)
	assert.Equal(t, types.PositionShort, state.PositionSide)

	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, risks, 1)
	assert.Equal(t, types.PositionShort, risks[0].PositionSide)
}

func TestPaperTradeExchange_CloseLongPosition(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeSell, fixedpoint.NewFromFloat(51000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	state := e.futuresStates["BTCUSDT"]
	assert.True(t, state.PositionAmount.IsZero())
}

func TestPaperTradeExchange_FlipLongToShort(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.mu.Lock()
	// Long 0.5 BTC
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(0.5))
	// Sell 1.0 BTC → flips to short 0.5
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeSell, fixedpoint.NewFromFloat(52000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	state := e.futuresStates["BTCUSDT"]
	assert.True(t, state.PositionAmount.Sign() < 0)
	assert.True(t, state.PositionAmount.Abs().Compare(fixedpoint.NewFromFloat(0.5)) == 0)
	assert.Equal(t, types.PositionShort, state.PositionSide)
	assert.True(t, state.EntryPrice.Compare(fixedpoint.NewFromFloat(52000.0)) == 0)
}

func TestPaperTradeExchange_FlipShortToLong(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.mu.Lock()
	// Short 0.5 BTC
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeSell, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(0.5))
	// Buy 1.0 BTC → flips to long 0.5
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(48000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	state := e.futuresStates["BTCUSDT"]
	assert.True(t, state.PositionAmount.Sign() > 0)
	assert.True(t, state.PositionAmount.Compare(fixedpoint.NewFromFloat(0.5)) == 0)
	assert.Equal(t, types.PositionLong, state.PositionSide)
	assert.True(t, state.EntryPrice.Compare(fixedpoint.NewFromFloat(48000.0)) == 0)
}

func TestPaperTradeExchange_WeightedAverageEntry(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.mu.Lock()
	// Buy 1 at 50000
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	// Buy 1 at 52000 → weighted avg = (50000*1 + 52000*1) / 2 = 51000
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(52000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	state := e.futuresStates["BTCUSDT"]
	assert.True(t, state.EntryPrice.Compare(fixedpoint.NewFromFloat(51000.0)) == 0)
	assert.True(t, state.PositionAmount.Compare(fixedpoint.NewFromFloat(2.0)) == 0)
}

func TestPaperTradeExchange_LiquidationPrice_Long(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 10)

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, risks, 1)

	// Long liquidation = entry * (1 - 1/leverage + maintRate)
	// = 50000 * (1 - 0.1 + 0.005) = 50000 * 0.905 = 45250
	expected := fixedpoint.NewFromFloat(45250.0)
	assert.True(t, risks[0].LiquidationPrice.Compare(expected) == 0,
		"expected %s, got %s", expected.String(), risks[0].LiquidationPrice.String())
}

func TestPaperTradeExchange_LiquidationPrice_Short(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 10)

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeSell, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, risks, 1)

	// Short liquidation = entry * (1 + 1/leverage - maintRate)
	// = 50000 * (1 + 0.1 - 0.005) = 50000 * 1.095 = 54750
	expected := fixedpoint.NewFromFloat(54750.0)
	assert.True(t, risks[0].LiquidationPrice.Compare(expected) == 0,
		"expected %s, got %s", expected.String(), risks[0].LiquidationPrice.String())
}

func TestPaperTradeExchange_UnrealizedPnL(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.mu.Lock()
	// Long 1 BTC at 50000
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	// Set mark price to 52000
	book := e.matchingBooks["BTCUSDT"]
	book.lastPrice = fixedpoint.NewFromFloat(52000.0)
	e.mu.Unlock()

	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, risks, 1)

	// Unrealized PnL = (mark - entry) * amount = (52000 - 50000) * 1 = 2000
	expected := fixedpoint.NewFromFloat(2000.0)
	assert.True(t, risks[0].UnrealizedPnL.Compare(expected) == 0,
		"expected %s, got %s", expected.String(), risks[0].UnrealizedPnL.String())
}

func TestPaperTradeExchange_UseMargin(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseMargin()

	settings := e.GetMarginSettings()
	assert.True(t, settings.IsMargin)
	assert.False(t, settings.IsIsolatedMargin)
}

func TestPaperTradeExchange_UseIsolatedMargin(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseIsolatedMargin("BTCUSDT")

	settings := e.GetMarginSettings()
	assert.True(t, settings.IsMargin)
	assert.True(t, settings.IsIsolatedMargin)
	assert.Equal(t, "BTCUSDT", settings.IsolatedMarginSymbol)
}

func TestPaperTradeExchange_BorrowAndRepay(t *testing.T) {
	e := newTestPaperTradeExchange()

	err := e.BorrowMarginAsset(context.Background(), "BTC", fixedpoint.NewFromFloat(1.5))
	require.NoError(t, err)

	borrowed := e.MarginBorrowed("BTC")
	assert.True(t, borrowed.Compare(fixedpoint.NewFromFloat(1.5)) == 0)

	err = e.RepayMarginAsset(context.Background(), "BTC", fixedpoint.NewFromFloat(0.5))
	require.NoError(t, err)

	borrowed = e.MarginBorrowed("BTC")
	assert.True(t, borrowed.Compare(fixedpoint.NewFromFloat(1.0)) == 0)
}

func TestPaperTradeExchange_RepayExceedsBorrowed(t *testing.T) {
	e := newTestPaperTradeExchange()

	err := e.BorrowMarginAsset(context.Background(), "BTC", fixedpoint.NewFromFloat(1.0))
	require.NoError(t, err)

	err = e.RepayMarginAsset(context.Background(), "BTC", fixedpoint.NewFromFloat(2.0))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds borrowed")
}

func TestPaperTradeExchange_RepayUnknownAsset(t *testing.T) {
	e := newTestPaperTradeExchange()

	err := e.RepayMarginAsset(context.Background(), "ETH", fixedpoint.NewFromFloat(1.0))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no margin debt")
}

func TestPaperTradeExchange_MaxBorrowable(t *testing.T) {
	e := newTestPaperTradeExchange()

	max, err := e.QueryMarginAssetMaxBorrowable(context.Background(), "USDT")
	require.NoError(t, err)
	// Default balance is 10000 USDT, max = 5x
	assert.True(t, max.Compare(fixedpoint.NewFromFloat(50000.0)) == 0)
}

func TestPaperTradeExchange_MaxBorrowable_NoBalance(t *testing.T) {
	e := newTestPaperTradeExchange()

	max, err := e.QueryMarginAssetMaxBorrowable(context.Background(), "XYZ")
	require.NoError(t, err)
	assert.True(t, max.IsZero())
}

func TestPaperTradeExchange_SupportsShortSell(t *testing.T) {
	e := newTestPaperTradeExchange()
	assert.False(t, e.SupportsShortSell())

	e.UseFutures()
	assert.True(t, e.SupportsShortSell())

	e2 := newTestPaperTradeExchange()
	e2.UseMargin()
	assert.True(t, e2.SupportsShortSell())
}

func TestPaperTradeExchange_QueryAllPositionRisks(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.updateFuturesPositionLocked("ETHUSDT", types.SideTypeSell, fixedpoint.NewFromFloat(3000.0), fixedpoint.NewFromFloat(2.0))
	e.mu.Unlock()

	risks, err := e.QueryPositionRisk(context.Background())
	require.NoError(t, err)
	assert.Len(t, risks, 2)

	symbols := map[string]bool{}
	for _, r := range risks {
		symbols[r.Symbol] = true
	}
	assert.True(t, symbols["BTCUSDT"])
	assert.True(t, symbols["ETHUSDT"])
}

func TestPaperTradeExchange_GetOrCreateFuturesState_Defaults(t *testing.T) {
	e := newTestPaperTradeExchange()
	state := e.getOrCreateFuturesState("BTCUSDT")
	assert.Equal(t, 20, state.Leverage)
	assert.Equal(t, "USDT", state.MarginAsset)
}

func TestPaperTradeExchange_MarginBorrowed_UnknownAsset(t *testing.T) {
	e := newTestPaperTradeExchange()
	borrowed := e.MarginBorrowed("XYZ")
	assert.True(t, borrowed.IsZero())
}

func TestPaperTradeExchange_FullRepayClearsInterest(t *testing.T) {
	e := newTestPaperTradeExchange()

	err := e.BorrowMarginAsset(context.Background(), "BTC", fixedpoint.NewFromFloat(1.0))
	require.NoError(t, err)

	// Simulate accrued interest
	e.mu.Lock()
	e.marginStates["BTC"].Interest = fixedpoint.NewFromFloat(0.01)
	e.mu.Unlock()

	err = e.RepayMarginAsset(context.Background(), "BTC", fixedpoint.NewFromFloat(1.0))
	require.NoError(t, err)

	// Interest should be cleared when borrowed goes to zero
	e.mu.Lock()
	interest := e.marginStates["BTC"].Interest
	e.mu.Unlock()
	assert.True(t, interest.IsZero())
}

func TestPaperTradeExchange_QueryPositionRisk_WithOpenPosition(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	risks, err := e.QueryPositionRisk(context.Background())
	require.NoError(t, err)
	require.Len(t, risks, 1)
	assert.Equal(t, "BTCUSDT", risks[0].Symbol)
	assert.True(t, risks[0].PositionAmount.Compare(fixedpoint.NewFromFloat(1.0)) == 0)
}

func TestPaperTradeExchange_QueryPositionRisk_ClosedPositionReturnsZero(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	// Open and close a position
	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeSell, fixedpoint.NewFromFloat(51000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	// Should still return a risk row with position_amount=0 so FuturesService can update DB
	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, risks, 1)
	assert.True(t, risks[0].PositionAmount.IsZero())
}

func TestPaperTradeExchange_PartialClosePosition(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.mu.Lock()
	// Long 1 BTC at 50000
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	// Partial close: sell 0.4 at 52000
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeSell, fixedpoint.NewFromFloat(52000.0), fixedpoint.NewFromFloat(0.4))
	e.mu.Unlock()

	state := e.futuresStates["BTCUSDT"]
	// Remaining: 0.6 BTC
	assert.True(t, state.PositionAmount.Compare(fixedpoint.NewFromFloat(0.6)) == 0)
	// Entry should remain 50000 for partial close
	assert.True(t, state.EntryPrice.Compare(fixedpoint.NewFromFloat(50000.0)) == 0)
}

func TestPaperTradeExchange_UpdateFuturesPosition_NonFutures(t *testing.T) {
	e := newTestPaperTradeExchange()
	// Don't call UseFutures()

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(1.0))
	e.mu.Unlock()

	// Should not create state when not in futures mode
	_, exists := e.futuresStates["BTCUSDT"]
	assert.False(t, exists)
}

func TestPaperTradeExchange_MarginAsset_FromMarket(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.MarginAsset = "" // reset to test auto-detection

	e.mu.Lock()
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy, fixedpoint.NewFromFloat(50000.0), fixedpoint.NewFromFloat(0.1))
	e.mu.Unlock()

	assert.Equal(t, "USDT", state.MarginAsset)
}

func TestPaperTradeExchange_MarginInterestAccrual(t *testing.T) {
	e := newTestPaperTradeExchange()

	err := e.BorrowMarginAsset(context.Background(), "BTC", fixedpoint.NewFromFloat(10.0))
	require.NoError(t, err)

	// Simulate 5 hours of interest
	e.mu.Lock()
	e.marginStates["BTC"].LastAccrual = e.marginStates["BTC"].LastAccrual.Add(-5 * time.Hour)
	e.mu.Unlock()

	e.updateMarginInterest()

	e.mu.Lock()
	interest := e.marginStates["BTC"].Interest
	e.mu.Unlock()

	// Interest = 10 * 0.0001 * 5 = 0.005
	expected := fixedpoint.NewFromFloat(0.005)
	assert.True(t, interest.Compare(expected) == 0,
		"expected interest %s, got %s", expected.String(), interest.String())
}

func TestPaperTradeExchange_MarginInterest_NoAccrualUnderOneHour(t *testing.T) {
	e := newTestPaperTradeExchange()

	err := e.BorrowMarginAsset(context.Background(), "BTC", fixedpoint.NewFromFloat(10.0))
	require.NoError(t, err)

	// Only 30 minutes since last accrual
	e.mu.Lock()
	e.marginStates["BTC"].LastAccrual = e.marginStates["BTC"].LastAccrual.Add(-30 * time.Minute)
	e.mu.Unlock()

	e.updateMarginInterest()

	e.mu.Lock()
	interest := e.marginStates["BTC"].Interest
	e.mu.Unlock()

	assert.True(t, interest.IsZero(), "no interest should accrue under 1 hour")
}

// --- Integration tests: full SubmitOrder → kline fill → position tracking path ---

func TestPaperTradeExchange_EffectiveLeverage(t *testing.T) {
	e := newTestPaperTradeExchange()
	// No futures state → returns 1
	assert.True(t, e.effectiveLeverage("BTCUSDT").Compare(fixedpoint.One) == 0)

	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 10)
	assert.True(t, e.effectiveLeverage("BTCUSDT").Compare(fixedpoint.NewFromInt(10)) == 0)
}

func TestPaperTradeExchange_FuturesMarginLocking(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 10)

	// Feed initial kline so matching engine has a lastPrice
	e.OnKLineClosed(types.KLine{
		Symbol: "BTCUSDT",
		Open:   fixedpoint.NewFromFloat(50000.0),
		Close:  fixedpoint.NewFromFloat(50000.0),
		High:   fixedpoint.NewFromFloat(50000.0),
		Low:    fixedpoint.NewFromFloat(50000.0),
	})

	// With 10x leverage, buying 1 BTC at ~50000 should only lock ~5000 USDT margin
	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Quantity: fixedpoint.NewFromFloat(1.0),
		Price:    fixedpoint.NewFromFloat(50000.0),
	})
	require.NoError(t, err)

	// Locked = notional / leverage = 50000 / 10 = 5000
	bal, _ := e.account.Balance("USDT")
	expected := fixedpoint.NewFromFloat(5000.0)
	assert.True(t, bal.Locked.Compare(expected) == 0,
		"expected locked %s, got %s", expected.String(), bal.Locked.String())
}

func TestPaperTradeExchange_KlineFill_TracksFuturesPosition(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 20)

	// Place a limit buy order
	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Quantity: fixedpoint.NewFromFloat(1.0),
		Price:    fixedpoint.NewFromFloat(50000.0),
	})
	require.NoError(t, err)

	// Feed a kline that fills the order (high >= 50000)
	e.OnKLineClosed(types.KLine{
		Symbol: "BTCUSDT",
		Open:   fixedpoint.NewFromFloat(49900.0),
		Close:  fixedpoint.NewFromFloat(50100.0),
		High:   fixedpoint.NewFromFloat(50200.0),
		Low:    fixedpoint.NewFromFloat(49800.0),
	})

	// Verify position was tracked
	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, risks, 1)
	assert.Equal(t, types.PositionLong, risks[0].PositionSide)
	assert.True(t, risks[0].PositionAmount.Compare(fixedpoint.NewFromFloat(1.0)) == 0)
}

func TestPaperTradeExchange_KlineFill_ShortSell(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	// Place a limit sell (short) — should succeed without holding BTC
	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Quantity: fixedpoint.NewFromFloat(0.5),
		Price:    fixedpoint.NewFromFloat(50000.0),
	})
	require.NoError(t, err)

	// Feed a kline that fills (low <= 50000)
	e.OnKLineClosed(types.KLine{
		Symbol: "BTCUSDT",
		Open:   fixedpoint.NewFromFloat(50100.0),
		Close:  fixedpoint.NewFromFloat(49900.0),
		High:   fixedpoint.NewFromFloat(50200.0),
		Low:    fixedpoint.NewFromFloat(49800.0),
	})

	// Verify short position tracked
	risks, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
	require.NoError(t, err)
	require.Len(t, risks, 1)
	assert.Equal(t, types.PositionShort, risks[0].PositionSide)
	assert.True(t, risks[0].PositionAmount.Abs().Compare(fixedpoint.NewFromFloat(0.5)) == 0)
}

func TestPaperTradeExchange_SpotSellRejectsWithoutBalance(t *testing.T) {
	e := newTestPaperTradeExchange()
	// No futures/margin — spot mode

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Quantity: fixedpoint.NewFromFloat(1.0),
		Price:    fixedpoint.NewFromFloat(50000.0),
	})
	assert.Error(t, err, "spot sell without BTC balance should fail")
}
