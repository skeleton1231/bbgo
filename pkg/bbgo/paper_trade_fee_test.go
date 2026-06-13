package bbgo

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// seedKline sets lastPrice on the matching book so market orders can resolve
// a fill price. Called as the first kline so no resting orders are touched.
func seedKline(t *testing.T, e *PaperTradeExchange, symbol string, price float64) {
	t.Helper()
	book, ok := e.matchingBook(symbol)
	require.Truef(t, ok, "matching book missing for %s", symbol)
	book.mu.Lock()
	book.lastPrice = fixedpoint.NewFromFloat(price)
	book.mu.Unlock()
}

// TestPaperTradeFee_FuturesTakerVsMaker verifies the futures fee tier:
// taker 0.04% must equal exactly 2x maker 0.02%. Regression guard for the
// buildFillLocked branch that picks feeRate by isFutures && isTaker.
func TestPaperTradeFee_FuturesTakerVsMaker(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 50000.0)

	qty := fixedpoint.NewFromFloat(0.1)
	price := fixedpoint.NewFromFloat(50000.0)

	var sawTrade atomic.Bool
	var capturedFee fixedpoint.Value
	book, _ := e.matchingBook("BTCUSDT")
	book.OnTradeUpdate(func(trade types.Trade) {
		capturedFee = trade.Fee
		sawTrade.Store(true)
	})

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeMarket,
		Quantity: qty,
		Price:    price,
	})
	require.NoError(t, err)

	expectedTakerFee := qty.Mul(price).Mul(fixedpoint.NewFromFloat(paperFuturesTakerFeeRate))
	require.Eventually(t, func() bool { return sawTrade.Load() }, time.Second, 10*time.Millisecond,
		"taker trade callback should fire")
	assert.Truef(t, capturedFee.Compare(expectedTakerFee) == 0,
		"futures taker fee: want %s, got %s", expectedTakerFee.String(), capturedFee.String())

	// Maker fee must be exactly half the taker fee for the same notional.
	expectedMakerFee := qty.Mul(price).Mul(fixedpoint.NewFromFloat(paperFuturesMakerFeeRate))
	assert.Truef(t, expectedTakerFee.Compare(expectedMakerFee.Mul(fixedpoint.NewFromInt(2))) == 0,
		"futures taker (%s) must equal 2x maker (%s)", expectedTakerFee.String(), expectedMakerFee.String())
}

// TestPaperTradeFee_SpotRates verifies the spot fee tier is 0.1% (10 bps).
// Regression guard against accidental divergence when futures/margin rates
// are tuned.
func TestPaperTradeFee_SpotRates(t *testing.T) {
	e := newTestPaperTradeExchange() // spot mode by default
	seedKline(t, e, "BTCUSDT", 50000.0)

	qty := fixedpoint.NewFromFloat(0.2)
	price := fixedpoint.NewFromFloat(50000.0)

	var sawTrade atomic.Bool
	var capturedFee fixedpoint.Value
	book, _ := e.matchingBook("BTCUSDT")
	book.OnTradeUpdate(func(trade types.Trade) {
		capturedFee = trade.Fee
		sawTrade.Store(true)
	})

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeMarket,
		Quantity: qty,
		Price:    price,
	})
	require.NoError(t, err)

	expected := qty.Mul(price).Mul(fixedpoint.NewFromFloat(paperSpotTakerFeeRate))
	require.Eventually(t, func() bool { return sawTrade.Load() }, time.Second, 10*time.Millisecond,
		"spot taker trade callback should fire")
	assert.Truef(t, capturedFee.Compare(expected) == 0,
		"spot taker fee: want %s, got %s", expected.String(), capturedFee.String())
}

// TestTakerLimitBuy_RefundsExcessLocked verifies that a taker limit buy
// (limit price above current mark) refunds the excess locked quote back to
// available. Regression guard for the refund path in SubmitOrder.
func TestTakerLimitBuy_RefundsExcessLocked(t *testing.T) {
	e := newTestPaperTradeExchange()
	seedKline(t, e, "BTCUSDT", 50000.0)

	qty := fixedpoint.NewFromFloat(0.1)
	limitPrice := fixedpoint.NewFromFloat(50100.0) // above mark -> taker

	// Snapshot available quote before submit so we can compute the refund delta.
	before, _ := e.account.Balance("USDT")
	availBefore := before.Available

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Quantity: qty,
		Price:    limitPrice,
	})
	require.NoError(t, err)

	after, _ := e.account.Balance("USDT")
	availAfter := after.Available

	// Locked = limitPrice * qty; spent at fill = 50000 * qty.
	// After refund, available should equal:
	//   availBefore - limitPrice*qty (locked)
	//                + (limitPrice - fillPrice) * qty (refund)
	//                - fillPrice * qty (used by fill)
	//   = availBefore - fillPrice * qty - fee
	// Net delta: -(50000 * 0.1) - (50000 * 0.1 * 0.001) = -5000 - 5 = -5005
	fillPrice := fixedpoint.NewFromFloat(50000.0)
	expectedDelta := fillPrice.Mul(qty).Neg().Sub(
		fillPrice.Mul(qty).Mul(fixedpoint.NewFromFloat(paperSpotTakerFeeRate)),
	)
	actualDelta := availAfter.Sub(availBefore)
	assert.Truef(t, actualDelta.Compare(expectedDelta) == 0,
		"taker limit buy refund: want delta=%s, got delta=%s (avail before=%s after=%s)",
		expectedDelta.String(), actualDelta.String(), availBefore.String(), availAfter.String())
}

// TestMarginCallbacks_FireAndNoDeadlock verifies that BorrowMarginAsset fires
// OnMarginLoan outside the exchange mutex and returns within a sane deadline.
// Regression guard for the M1 lock-scope fix that moved callback emission
// after e.mu.Unlock().
func TestMarginCallbacks_FireAndNoDeadlock(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseMargin()

	var loanFired atomic.Bool
	e.OnMarginLoan = func(types.MarginLoan) { loanFired.Store(true) }

	amount := fixedpoint.NewFromFloat(500.0)
	done := make(chan error, 1)
	go func() {
		done <- e.BorrowMarginAsset(context.Background(), "USDT", amount)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("BorrowMarginAsset deadlocked — callback held under lock")
	}
	require.Eventually(t, func() bool { return loanFired.Load() }, time.Second, 10*time.Millisecond,
		"OnMarginLoan callback should fire after BorrowMarginAsset returns")

	assert.Equal(t, amount.String(), e.MarginBorrowed("USDT").String())
	bal, _ := e.account.Balance("USDT")
	assert.Truef(t, bal.Available.Compare(amount) >= 0,
		"borrow should credit available balance; got %s", bal.Available.String())
}
