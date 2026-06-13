package bbgo

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func klineForTest(symbol string, open, high, low, close float64) types.KLine {
	return types.KLine{
		Symbol:    symbol,
		StartTime: types.Time(time.Now().Add(-time.Minute)),
		EndTime:   types.Time(time.Now()),
		Interval:  types.Interval1m,
		Open:      fixedpoint.NewFromFloat(open),
		High:      fixedpoint.NewFromFloat(high),
		Low:       fixedpoint.NewFromFloat(low),
		Close:     fixedpoint.NewFromFloat(close),
		Volume:    fixedpoint.NewFromFloat(1.0),
		Closed:    true,
	}
}

// TestFuturesLiquidation_LongForceClosed verifies that a long position is
// force-closed when a kline's low crosses the liquidation price.
// Entry=60000, leverage=10, maintRate=0.004 → liqPrice = 60000*(1-0.1+0.004) = 54240.
func TestFuturesLiquidation_LongForceClosed(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 60000.0)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	var sawTrade atomic.Bool
	var capturedPnL sql.NullFloat64
	var capturedSide types.SideType
	book, _ := e.matchingBook("BTCUSDT")
	book.OnTradeUpdate(func(trade types.Trade) {
		capturedSide = trade.Side
		capturedPnL = trade.PnL
		sawTrade.Store(true)
	})

	book.ProcessKLine(klineForTest("BTCUSDT", 55000, 55500, 54000, 54500))

	require.True(t, sawTrade.Load(), "liquidation trade should fire")
	assert.Equal(t, types.SideTypeSell, capturedSide, "long liquidation is a SELL")

	state = e.getOrCreateFuturesState("BTCUSDT")
	assert.True(t, state.PositionAmount.IsZero(), "position should be force-closed")
	assert.True(t, capturedPnL.Valid, "liquidation trade must have PnL")
	assert.InDelta(t, -5.76, capturedPnL.Float64, 0.1,
		"PnL = (liqPrice-entry)*qty = (54240-60000)*0.001 = -5.76")
}

// TestFuturesLiquidation_ShortForceClosed verifies that a short position is
// force-closed when a kline's high crosses the liquidation price.
// Entry=60000, leverage=10, maintRate=0.004 → liqPrice = 60000*(1+0.1-0.004) = 65760.
func TestFuturesLiquidation_ShortForceClosed(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 60000.0)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(-0.001)

	var sawTrade atomic.Bool
	var capturedPnL sql.NullFloat64
	var capturedSide types.SideType
	book, _ := e.matchingBook("BTCUSDT")
	book.OnTradeUpdate(func(trade types.Trade) {
		capturedSide = trade.Side
		capturedPnL = trade.PnL
		sawTrade.Store(true)
	})

	book.ProcessKLine(klineForTest("BTCUSDT", 65000, 66000, 64500, 65500))

	require.True(t, sawTrade.Load(), "liquidation trade should fire")
	assert.Equal(t, types.SideTypeBuy, capturedSide, "short liquidation is a BUY")

	state = e.getOrCreateFuturesState("BTCUSDT")
	assert.True(t, state.PositionAmount.IsZero(), "position should be force-closed")
	assert.True(t, capturedPnL.Valid, "liquidation trade must have PnL")
	assert.InDelta(t, -5.76, capturedPnL.Float64, 0.1,
		"PnL = (entry-liqPrice)*qty = (60000-65760)*0.001 = -5.76")
}

// TestFuturesLiquidation_NoTrigger verifies that a position is NOT liquidated
// when the kline's price range doesn't cross the liquidation price.
func TestFuturesLiquidation_NoTrigger(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 60000.0)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	var sawTrade atomic.Bool
	book, _ := e.matchingBook("BTCUSDT")
	book.OnTradeUpdate(func(trade types.Trade) {
		sawTrade.Store(true)
	})

	// liqPrice ≈ 54240; kline low=55000 stays above → no liquidation
	book.ProcessKLine(klineForTest("BTCUSDT", 59000, 59500, 55000, 58500))

	assert.False(t, sawTrade.Load(), "no trade should fire when liq price not crossed")

	state = e.getOrCreateFuturesState("BTCUSDT")
	assert.InDelta(t, 0.001, state.PositionAmount.Float64(), 0.0001,
		"position should remain open")
}
