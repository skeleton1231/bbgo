package bbgo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TestReduceOnly_RejectWhenNoPosition verifies that a reduceOnly order is
// rejected when there is no open position to reduce.
func TestReduceOnly_RejectWhenNoPosition(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	seedKline(t, e, "BTCUSDT", 60000.0)

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:     "BTCUSDT",
		Side:       types.SideTypeSell,
		Type:       types.OrderTypeMarket,
		Quantity:   fixedpoint.NewFromFloat(0.001),
		Price:      fixedpoint.NewFromFloat(60000.0),
		ReduceOnly: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no open position")
}

// TestReduceOnly_RejectWrongDirection verifies that a reduceOnly BUY is
// rejected when the position is long (BUY would increase, not reduce).
func TestReduceOnly_RejectWrongDirection(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	seedKline(t, e, "BTCUSDT", 60000.0)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:     "BTCUSDT",
		Side:       types.SideTypeBuy,
		Type:       types.OrderTypeMarket,
		Quantity:   fixedpoint.NewFromFloat(0.001),
		Price:      fixedpoint.NewFromFloat(60000.0),
		ReduceOnly: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "would increase position")
}

// TestReduceOnly_CapExcessQuantity verifies that a reduceOnly SELL with
// quantity exceeding the position is capped at the position size.
func TestReduceOnly_CapExcessQuantity(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	seedKline(t, e, "BTCUSDT", 60000.0)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	order, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:     "BTCUSDT",
		Side:       types.SideTypeSell,
		Type:       types.OrderTypeMarket,
		Quantity:   fixedpoint.NewFromFloat(0.003),
		Price:      fixedpoint.NewFromFloat(60000.0),
		ReduceOnly: true,
	})
	require.NoError(t, err)
	assert.InDelta(t, 0.001, order.Quantity.Float64(), 0.0001,
		"quantity should be capped at position size")
}

// TestReduceOnly_AllowCorrectDirection verifies that a reduceOnly SELL on
// a long position succeeds and closes the position.
func TestReduceOnly_AllowCorrectDirection(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	seedKline(t, e, "BTCUSDT", 60000.0)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	order, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:     "BTCUSDT",
		Side:       types.SideTypeSell,
		Type:       types.OrderTypeMarket,
		Quantity:   fixedpoint.NewFromFloat(0.001),
		Price:      fixedpoint.NewFromFloat(60000.0),
		ReduceOnly: true,
	})
	require.NoError(t, err)
	assert.NotNil(t, order)
}

// TestReduceOnly_FillTimeCancelWhenPositionClosed verifies that a ReduceOnly
// limit order is canceled at fill time if the position has already been closed
// by another order between submit and fill. Without this re-check, SaaS users
// testing stop-loss strategies would see the ReduceOnly order open a new
// position in the opposite direction — diverging from real exchange behavior.
func TestReduceOnly_FillTimeCancelWhenPositionClosed(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 50000.0)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(50000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	limitOrder, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:     "BTCUSDT",
		Side:       types.SideTypeSell,
		Type:       types.OrderTypeLimit,
		Quantity:   fixedpoint.NewFromFloat(0.001),
		Price:      fixedpoint.NewFromFloat(60000.0),
		ReduceOnly: true,
	})
	require.NoError(t, err)

	book, ok := e.matchingBook("BTCUSDT")
	require.True(t, ok)

	var lastUpdate types.Order
	book.OnOrderUpdate(func(o types.Order) {
		lastUpdate = o
	})

	state.PositionAmount = fixedpoint.Zero

	book.ProcessKLine(types.KLine{
		Symbol: "BTCUSDT",
		Open:   fixedpoint.NewFromFloat(50000.0),
		High:   fixedpoint.NewFromFloat(61000.0),
		Low:    fixedpoint.NewFromFloat(49900.0),
		Close:  fixedpoint.NewFromFloat(60500.0),
	})

	assert.Equal(t, types.OrderStatusCanceled, lastUpdate.Status,
		"ReduceOnly order should be canceled at fill time when position is closed")

	finalState := e.getOrCreateFuturesState("BTCUSDT")
	assert.True(t, finalState.PositionAmount.IsZero() || finalState.PositionAmount.Sign() > 0,
		"ReduceOnly fill-time check must not open a SHORT position, got PositionAmount=%s",
		finalState.PositionAmount.String())

	assert.Equal(t, limitOrder.OrderID, lastUpdate.OrderID,
		"canceled update should reference the ReduceOnly limit order")
}
