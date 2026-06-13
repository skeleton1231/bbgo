package bbgo

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TestFuturesRealizedPnL_LongClose verifies that a SELL closing a long
// position computes PnL = (sellPrice - entryPrice) * qty.
func TestFuturesRealizedPnL_LongClose(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 3)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.002)

	pnl := e.computeRealizedPnLLocked("BTCUSDT", types.SideTypeSell,
		fixedpoint.NewFromFloat(61000.0), fixedpoint.NewFromFloat(0.002))

	// (61000 - 60000) * 0.002 = 2.0
	assert.InDelta(t, 2.0, pnl, 0.0001)
}

// TestFuturesRealizedPnL_ShortClose verifies that a BUY closing a short
// position computes PnL = (entryPrice - buyPrice) * qty.
func TestFuturesRealizedPnL_ShortClose(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(65000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(-0.001)

	pnl := e.computeRealizedPnLLocked("BTCUSDT", types.SideTypeBuy,
		fixedpoint.NewFromFloat(64000.0), fixedpoint.NewFromFloat(0.001))

	// (65000 - 64000) * 0.001 = 1.0
	assert.InDelta(t, 1.0, pnl, 0.0001)
}

// TestFuturesRealizedPnL_OpeningTrade verifies that opening/adding trades
// return zero PnL.
func TestFuturesRealizedPnL_OpeningTrade(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	pnl := e.computeRealizedPnLLocked("BTCUSDT", types.SideTypeBuy,
		fixedpoint.NewFromFloat(60000.0), fixedpoint.NewFromFloat(0.001))
	assert.InDelta(t, 0.0, pnl, 0.0001)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)
	pnl = e.computeRealizedPnLLocked("BTCUSDT", types.SideTypeBuy,
		fixedpoint.NewFromFloat(61000.0), fixedpoint.NewFromFloat(0.001))
	assert.InDelta(t, 0.0, pnl, 0.0001)
}

// TestFuturesRealizedPnL_PartialClose verifies PnL is proportional to the
// reduced quantity, not the full trade quantity.
func TestFuturesRealizedPnL_PartialClose(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	pnl := e.computeRealizedPnLLocked("BTCUSDT", types.SideTypeSell,
		fixedpoint.NewFromFloat(63000.0), fixedpoint.NewFromFloat(0.003))

	// (63000 - 60000) * 0.001 = 3.0 (only the reducing portion, not 0.003)
	assert.InDelta(t, 3.0, pnl, 0.0001)
}

// TestFuturesTrade_SetsPnLOnFill verifies the end-to-end flow: a SELL that
// closes an existing long position sets trade.PnL via the trade callback.
func TestFuturesTrade_SetsPnLOnFill(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetLeverage(context.Background(), "BTCUSDT", 3)
	seedKline(t, e, "BTCUSDT", 61000.0)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	var capturedPnL atomic.Value
	book, _ := e.matchingBook("BTCUSDT")
	book.OnTradeUpdate(func(trade types.Trade) {
		capturedPnL.Store(trade.PnL)
	})

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeMarket,
		Quantity: fixedpoint.NewFromFloat(0.001),
		Price:    fixedpoint.NewFromFloat(61000.0),
	})
	assert.NoError(t, err)

	stored, ok := capturedPnL.Load().(sql.NullFloat64)
	if assert.True(t, ok, "trade callback should fire") {
		assert.True(t, stored.Valid, "trade.PnL should be set for closing trade")
		assert.InDelta(t, 1.0, stored.Float64, 0.01,
			"PnL = (61000-60000)*0.001 = 1.0")
	}
}

// TestFuturesRealizedPnL_PositionFlip verifies that when a SELL quantity exceeds
// the long position, PnL is computed only on the reducing portion — not on the
// portion that opens a new short. Regression guard for the flip branch where
// reducingQty must be capped at |position|.
func TestFuturesRealizedPnL_PositionFlip(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	// SELL 0.003 @ 61000 closes 0.001 long, then opens 0.002 short.
	pnl := e.computeRealizedPnLLocked("BTCUSDT", types.SideTypeSell,
		fixedpoint.NewFromFloat(61000.0), fixedpoint.NewFromFloat(0.003))

	// Only the reducing portion earns PnL: (61000-60000)*0.001 = 1.0
	assert.InDelta(t, 1.0, pnl, 0.0001)
}

// TestFuturesRealizedPnL_AddThenClose verifies PnL uses the weighted-average
// entry price after adding to a position, not the first fill price.
func TestFuturesRealizedPnL_AddThenClose(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy,
		fixedpoint.NewFromFloat(60000.0), fixedpoint.NewFromFloat(0.001), "test")
	e.updateFuturesPositionLocked("BTCUSDT", types.SideTypeBuy,
		fixedpoint.NewFromFloat(62000.0), fixedpoint.NewFromFloat(0.001), "test")

	state := e.getOrCreateFuturesState("BTCUSDT")
	assert.InDelta(t, 61000.0, state.EntryPrice.Float64(), 0.01,
		"weighted avg entry = (60000*0.001 + 62000*0.001) / 0.002")
	assert.InDelta(t, 0.002, state.PositionAmount.Float64(), 0.0001)

	pnl := e.computeRealizedPnLLocked("BTCUSDT", types.SideTypeSell,
		fixedpoint.NewFromFloat(63000.0), fixedpoint.NewFromFloat(0.002))

	assert.InDelta(t, 4.0, pnl, 0.0001, "PnL = (63000-61000)*0.002 = 4.0")
}

// TestSpotMode_NoPnLOnFill verifies that spot-mode trades never set trade.PnL.
// Only futures reducing trades should carry PnL.
func TestSpotMode_NoPnLOnFill(t *testing.T) {
	e := newTestPaperTradeExchange()
	seedKline(t, e, "BTCUSDT", 50000.0)

	var capturedPnL atomic.Value
	book, _ := e.matchingBook("BTCUSDT")
	book.OnTradeUpdate(func(trade types.Trade) {
		capturedPnL.Store(trade.PnL)
	})

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeMarket,
		Quantity: fixedpoint.NewFromFloat(0.001),
		Price:    fixedpoint.NewFromFloat(50000.0),
	})
	require.NoError(t, err)

	stored, ok := capturedPnL.Load().(sql.NullFloat64)
	require.True(t, ok, "trade callback should fire")
	assert.False(t, stored.Valid, "spot trade must not set PnL (futures-only field)")
}
