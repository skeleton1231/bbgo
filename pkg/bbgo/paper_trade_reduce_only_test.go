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
