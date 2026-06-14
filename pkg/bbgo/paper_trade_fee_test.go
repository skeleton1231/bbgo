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

// TestRepayMargin_ResetsLastAccrual verifies that fully repaying a margin
// loan resets LastAccrual so a subsequent borrow doesn't charge interest
// for the idle gap between repay and re-borrow.
func TestRepayMargin_ResetsLastAccrual(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseMargin()

	require.NoError(t, e.BorrowMarginAsset(context.Background(), "USDT", fixedpoint.NewFromFloat(1000)))

	state := e.getOrCreateMarginState("USDT")
	state.LastAccrual = time.Now().Add(-10 * time.Hour)

	require.NoError(t, e.RepayMarginAsset(context.Background(), "USDT", fixedpoint.NewFromFloat(1000)))

	assert.True(t, state.LastAccrual.IsZero(),
		"LastAccrual must be zero after full repayment so re-borrow starts fresh")
	assert.True(t, state.Borrowed.IsZero(), "Borrowed must be zero after full repayment")

	require.NoError(t, e.BorrowMarginAsset(context.Background(), "USDT", fixedpoint.NewFromFloat(500)))
	assert.False(t, state.LastAccrual.IsZero(),
		"LastAccrual must be set on fresh borrow")
	assert.True(t, state.LastAccrual.After(time.Now().Add(-1*time.Minute)),
		"LastAccrual should be ~now after re-borrow, not the stale old value")
}

// TestUpdateMarginInterest_DeductsFromBalance verifies that accrued margin
// interest actually reduces the account balance. Regression guard for the
// updateMarginInterest path that previously only tracked state.Interest in
// memory without ever deducting from the wallet — borrowers paid zero.
func TestUpdateMarginInterest_DeductsFromBalance(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseMargin()

	principal := fixedpoint.NewFromFloat(1000.0)
	require.NoError(t, e.BorrowMarginAsset(context.Background(), "USDT", principal))

	state := e.getOrCreateMarginState("USDT")
	// Force at least 2 hours of accrued interest.
	staleAccrual := time.Now().Add(-2 * time.Hour)
	state.LastAccrual = staleAccrual

	balBefore, _ := e.account.Balance("USDT")
	stream := types.NewStandardStream()
	e.BindUserData(&stream)

	var balanceUpdates int
	stream.OnBalanceUpdate(func(_ types.BalanceMap) { balanceUpdates++ })

	e.updateMarginInterest()

	balAfter, _ := e.account.Balance("USDT")
	delta := balBefore.Available.Sub(balAfter.Available)

	// Expected: principal * 0.0001 * 2h = 1000 * 0.0001 * 2 = 0.2
	expectedInterest := principal.Mul(fixedpoint.MustNewFromString(defaultHourlyMarginRate)).Mul(fixedpoint.NewFromInt(2))
	assert.Truef(t, delta.Compare(expectedInterest) == 0,
		"interest deduction: want delta=%s, got delta=%s (before=%s after=%s)",
		expectedInterest.String(), delta.String(), balBefore.Available.String(), balAfter.Available.String())

	assert.True(t, state.Interest.Compare(expectedInterest) == 0,
		"state.Interest should equal expected accrued interest: want %s, got %s",
		expectedInterest.String(), state.Interest.String())

	assert.Greater(t, balanceUpdates, 0, "balance update should fire after interest deduction")
	assert.True(t, state.LastAccrual.After(staleAccrual),
		"LastAccrual should refresh to now, got %s", state.LastAccrual)
}

// TestRepayMarginAsset_SettlesPartialInterest verifies that repaying
// between hourly ticks settles the partial interest accrued since the last
// accrual. Regression guard for the repay path that previously zeroed
// LastAccrual on full repay without charging the partial-period interest —
// borrowers who repaid mid-hour got that fraction interest-free.
func TestRepayMarginAsset_SettlesPartialInterest(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseMargin()

	principal := fixedpoint.NewFromFloat(1000.0)
	require.NoError(t, e.BorrowMarginAsset(context.Background(), "USDT", principal))

	state := e.getOrCreateMarginState("USDT")
	// Simulate 30 minutes since the last hourly accrual.
	state.LastAccrual = time.Now().Add(-30 * time.Minute)

	balBefore, _ := e.account.Balance("USDT")

	var interestFired bool
	var interestAmount fixedpoint.Value
	e.OnMarginInterest = func(evt types.MarginInterest) {
		interestFired = true
		interestAmount = evt.Interest
	}

	require.NoError(t, e.RepayMarginAsset(context.Background(), "USDT", principal))

	balAfter, _ := e.account.Balance("USDT")
	totalDelta := balBefore.Available.Sub(balAfter.Available)

	// The partial interest should be > 0 and < 1 full hour's worth.
	fullHourInterest := principal.Mul(fixedpoint.MustNewFromString(defaultHourlyMarginRate))
	assert.True(t, interestFired, "OnMarginInterest should fire for the partial settlement")
	assert.Truef(t, interestAmount.Sign() > 0 && interestAmount.Compare(fullHourInterest) < 0,
		"partial interest should be > 0 and < 1 full hour (%s), got %s",
		fullHourInterest.String(), interestAmount.String())

	// Total deduction = principal + partial interest.
	expectedTotal := principal.Add(interestAmount)
	assert.Truef(t, totalDelta.Compare(expectedTotal) == 0,
		"total deduction should be principal + partial interest: want=%s got=%s",
		expectedTotal.String(), totalDelta.String())

	assert.True(t, state.Borrowed.IsZero(), "Borrowed should be zero after full repay")
	assert.True(t, state.LastAccrual.IsZero(), "LastAccrual should be cleared after full repay")
}

// TestRepayMarginAsset_PartialRepayUpdatesLastAccrual verifies that a
// partial repay updates LastAccrual to now (settling the partial interest)
// rather than leaving the stale timestamp, so subsequent hourly ticks don't
// over-charge the pre-repay period.
func TestRepayMarginAsset_PartialRepayUpdatesLastAccrual(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseMargin()

	principal := fixedpoint.NewFromFloat(1000.0)
	require.NoError(t, e.BorrowMarginAsset(context.Background(), "USDT", principal))

	state := e.getOrCreateMarginState("USDT")
	staleAccrual := time.Now().Add(-30 * time.Minute)
	state.LastAccrual = staleAccrual

	repayAmount := fixedpoint.NewFromFloat(400.0)
	require.NoError(t, e.RepayMarginAsset(context.Background(), "USDT", repayAmount))

	assert.Truef(t, state.Borrowed.Compare(fixedpoint.NewFromFloat(600.0)) == 0,
		"Borrowed should be 600 after 400 partial repay, got %s", state.Borrowed.String())
	assert.True(t, state.LastAccrual.After(staleAccrual),
		"LastAccrual should advance to now after partial repay, got %s", state.LastAccrual)
	assert.False(t, state.LastAccrual.IsZero(),
		"LastAccrual should not be zero for partial repay (still have outstanding debt)")
}

// TestCancelOrder_FuturesUnlocksMarginOnly verifies that canceling a
// futures limit buy unlocks only the locked margin (notional/leverage),
// not the full notional. Regression guard for the CancelOrder path that
// previously unlocked Price*Quantity for futures orders, inflating the
// available balance by (leverage-1)/leverage of the notional on every cancel.
func TestCancelOrder_FuturesUnlocksMarginOnly(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 50000.0)

	qty := fixedpoint.NewFromFloat(0.1)
	price := fixedpoint.NewFromFloat(49000.0) // below mark → maker (resting)

	before, _ := e.account.Balance("USDT")

	order, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol: "BTCUSDT",
		Side:   types.SideTypeBuy,
		Type:   types.OrderTypeLimit,
		Quantity: qty,
		Price:    price,
	})
	require.NoError(t, err)

	afterSubmit, _ := e.account.Balance("USDT")
	expectedLock := price.Mul(qty).Div(fixedpoint.NewFromInt(10)) // margin = notional / leverage
	submitDelta := before.Available.Sub(afterSubmit.Available)
	assert.Truef(t, submitDelta.Compare(expectedLock) == 0,
		"submit should lock margin only: want delta=%s, got delta=%s",
		expectedLock.String(), submitDelta.String())

	require.NoError(t, e.CancelOrders(context.Background(), *order))

	afterCancel, _ := e.account.Balance("USDT")
	cancelReturn := afterCancel.Available.Sub(afterSubmit.Available)
	assert.Truef(t, cancelReturn.Compare(expectedLock) == 0,
		"cancel should release exactly the locked margin: want=%s, got=%s (avail after submit=%s after cancel=%s)",
		expectedLock.String(), cancelReturn.String(),
		afterSubmit.Available.String(), afterCancel.Available.String())

	// Round-trip: available should be restored to pre-submit value.
	assert.Truef(t, afterCancel.Available.Compare(before.Available) == 0,
		"available should equal pre-submit value after cancel: before=%s after=%s",
		before.Available.String(), afterCancel.Available.String())
}

// TestTakerLimitBuy_FuturesRefundScalesByLeverage verifies that a futures
// taker limit buy refunds the excess locked margin scaled by 1/leverage,
// not the full notional difference. Regression guard for the refund path
// that previously unlocked (limitPrice-fillPrice)*qty for futures, when
// only (limitPrice-fillPrice)*qty/leverage was the actual excess margin.
func TestTakerLimitBuy_FuturesRefundScalesByLeverage(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 50000.0)

	qty := fixedpoint.NewFromFloat(0.1)
	limitPrice := fixedpoint.NewFromFloat(50100.0) // above mark → taker

	before, _ := e.account.Balance("USDT")

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeLimit,
		Quantity: qty,
		Price:    limitPrice,
	})
	require.NoError(t, err)

	after, _ := e.account.Balance("USDT")

	// Submit locks limitPrice*qty/leverage = 50100*0.1/10 = 501 USDT (margin).
	// Fill consumes the locked margin (silently fails to consume full notional
	// because only margin was locked — pre-existing opening-side quirk).
	// Refund should release (limitPrice-fillPrice)*qty/leverage = 100*0.1/10 = 1 USDT.
	// Net available delta: locked 501, refunded 1 → -500, minus fee on fillPrice*qty.
	fillPrice := fixedpoint.NewFromFloat(50000.0)
	fee := fillPrice.Mul(qty).Mul(fixedpoint.NewFromFloat(paperFuturesTakerFeeRate))
	expectedDelta := fixedpoint.NewFromFloat(-500.0).Sub(fee)
	actualDelta := after.Available.Sub(before.Available)
	assert.Truef(t, actualDelta.Compare(expectedDelta) == 0,
		"futures taker limit buy: want delta=%s, got delta=%s (before=%s after=%s)",
		expectedDelta.String(), actualDelta.String(),
		before.Available.String(), after.Available.String())

	// The incorrect pre-fix behavior would refund 10 USDT (full notional diff),
	// producing expectedDelta = -500 - fee + 9 = roughly -491 - fee.
}

// TestStopTrigger_FuturesReleasesMargin verifies that when a futures stop
// order triggers and fills, the originally-locked margin is released back
// to available. Regression guard for the checkStopTriggers path that
// previously consumed the locked margin via the (silently failing)
// UseLockedBalance call in buildFillLocked, leaving margin permanently
// locked after every triggered stop.
func TestStopTrigger_FuturesReleasesMargin(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 49000.0)

	qty := fixedpoint.NewFromFloat(0.1)
	stopPrice := fixedpoint.NewFromFloat(50000.0)

	before, _ := e.account.Balance("USDT")

	// Stop-market buy: triggers when high >= stopPrice.
	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:    "BTCUSDT",
		Side:      types.SideTypeBuy,
		Type:      types.OrderTypeStopMarket,
		Quantity:  qty,
		StopPrice: stopPrice,
	})
	require.NoError(t, err)

	afterSubmit, _ := e.account.Balance("USDT")
	expectedLock := stopPrice.Mul(qty).Div(fixedpoint.NewFromInt(10)) // 50000*0.1/10 = 500
	submitDelta := before.Available.Sub(afterSubmit.Available)
	assert.Truef(t, submitDelta.Compare(expectedLock) == 0,
		"stop submit should lock margin only: want delta=%s, got=%s",
		expectedLock.String(), submitDelta.String())

	// Push a kline whose High crosses the stop price → triggers the stop.
	book, _ := e.matchingBook("BTCUSDT")
	book.ProcessKLine(types.KLine{
		Symbol: "BTCUSDT",
		Open:   fixedpoint.NewFromFloat(49500.0),
		High:   fixedpoint.NewFromFloat(50500.0),
		Low:    fixedpoint.NewFromFloat(49400.0),
		Close:  fixedpoint.NewFromFloat(50200.0),
	})

	afterFill, _ := e.account.Balance("USDT")

	// After trigger+fill, the originally-locked 500 USDT margin must be
	// back in Available. If it's still Locked, the bug is present.
	assert.Truef(t, afterFill.Locked.IsZero(),
		"locked margin should be released after stop fills: locked=%s (before=%s afterSubmit=%s afterFill=%s)",
		afterFill.Locked.String(), before.Locked.String(),
		afterSubmit.Locked.String(), afterFill.Locked.String())
}

// TestCancelOrder_FuturesMarginSellWithBaseUnlocksBase verifies that canceling
// a futures/margin SELL unlocks base when the user had base at submit time.
// Regression guard for the cancel-askOrders path that previously unlocked quote
// (silently failing because nothing was locked in quote), leaving base
// permanently stranded.
func TestCancelOrder_FuturesMarginSellWithBaseUnlocksBase(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseMargin() // margin allows holding the base; futures typically does not
	seedKline(t, e, "BTCUSDT", 50000.0)

	// Seed the account with BTC so the submit path takes the "lock base" branch.
	e.account.AddBalance("BTC", fixedpoint.NewFromFloat(1.0))

	qty := fixedpoint.NewFromFloat(0.1)
	price := fixedpoint.NewFromFloat(51000.0) // above mark → maker (resting)

	btcBefore, _ := e.account.Balance("BTC")
	usdtBefore, _ := e.account.Balance("USDT")

	order, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Quantity: qty,
		Price:    price,
	})
	require.NoError(t, err)

	afterSubmitBTC, _ := e.account.Balance("BTC")
	afterSubmitUSDT, _ := e.account.Balance("USDT")
	assert.Truef(t, afterSubmitBTC.Locked.Compare(qty) == 0,
		"submit should lock base BTC: locked=%s, want=%s", afterSubmitBTC.Locked.String(), qty.String())
	assert.Truef(t, afterSubmitUSDT.Locked.IsZero(),
		"submit should not lock any USDT margin: locked=%s", afterSubmitUSDT.Locked.String())

	require.NoError(t, e.CancelOrders(context.Background(), *order))

	afterCancelBTC, _ := e.account.Balance("BTC")
	afterCancelUSDT, _ := e.account.Balance("USDT")

	assert.Truef(t, afterCancelBTC.Locked.IsZero(),
		"cancel should unlock base BTC: locked=%s", afterCancelBTC.Locked.String())
	assert.Truef(t, afterCancelBTC.Available.Compare(btcBefore.Available) == 0,
		"available BTC should be restored: before=%s after=%s",
		btcBefore.Available.String(), afterCancelBTC.Available.String())
	assert.Truef(t, afterCancelUSDT.Available.Compare(usdtBefore.Available) == 0,
		"USDT available should be untouched: before=%s after=%s",
		usdtBefore.Available.String(), afterCancelUSDT.Available.String())
}

// TestFuturesFill_SetsPositionAction verifies that futures fills tag each
// trade with PositionAction (OPEN_LONG, CLOSE_LONG, etc.) based on the
// position state BEFORE the fill. Regression guard for the SaaS frontend
// which categorizes trades by position_action; without this, paper-mode
// trades had an empty string and could not be distinguished.
func TestFuturesFill_SetsPositionAction(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 50000.0)

	cases := []struct {
		name   string
		side   types.SideType
		pre    float64 // position amount before this fill (set via direct state manipulation)
		want   string
	}{
		{"open_long", types.SideTypeBuy, 0.0, types.PositionActionOpenLong},
		{"add_long", types.SideTypeBuy, 0.05, types.PositionActionAddLong},
		{"close_long", types.SideTypeSell, 0.1, types.PositionActionCloseLong},
		{"reduce_long", types.SideTypeSell, 0.2, types.PositionActionReduceLong},
		{"open_short", types.SideTypeSell, 0.0, types.PositionActionOpenShort},
		{"close_short", types.SideTypeBuy, -0.1, types.PositionActionCloseShort},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Fresh exchange per case for isolation.
			ex := newTestPaperTradeExchange()
			ex.UseFutures()
			require.NoError(t, ex.SetLeverage(context.Background(), "BTCUSDT", 10))
			seedKline(t, ex, "BTCUSDT", 50000.0)

			// Set up pre-position state directly.
			state := ex.getOrCreateFuturesState("BTCUSDT")
			state.PositionAmount = fixedpoint.NewFromFloat(c.pre)
			state.EntryPrice = fixedpoint.NewFromFloat(50000.0)

			var captured types.Trade
			book, _ := ex.matchingBook("BTCUSDT")
			book.OnTradeUpdate(func(trade types.Trade) { captured = trade })

			_, err := ex.SubmitOrder(context.Background(), types.SubmitOrder{
				Symbol:   "BTCUSDT",
				Side:     c.side,
				Type:     types.OrderTypeMarket,
				Quantity: fixedpoint.NewFromFloat(0.1),
				Price:    fixedpoint.NewFromFloat(50000.0),
			})
			require.NoError(t, err)
			require.Eventually(t, func() bool { return captured.OrderID != 0 }, time.Second, 10*time.Millisecond,
				"trade callback should fire for %s", c.name)
			assert.Equalf(t, c.want, captured.PositionAction,
				"trade position action for %s: want=%s got=%s", c.name, c.want, captured.PositionAction)
		})
	}
}

// TestFuturesFill_OrderCarriesPositionAction verifies that the filled Order
// emitted via OnOrderUpdate also carries PositionAction. The order is what the
// SaaS orderWriter persists to paper_orders; without this field, the orders
// table had empty position_action even though trades had it (Loop 11 regression).
func TestFuturesFill_OrderCarriesPositionAction(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 10))
	seedKline(t, e, "BTCUSDT", 50000.0)

	var capturedOrder types.Order
	book, _ := e.matchingBook("BTCUSDT")
	book.OnOrderUpdate(func(o types.Order) {
		if o.Status == types.OrderStatusFilled {
			capturedOrder = o
		}
	})

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeBuy,
		Type:     types.OrderTypeMarket,
		Quantity: fixedpoint.NewFromFloat(0.1),
		Price:    fixedpoint.NewFromFloat(50000.0),
	})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return capturedOrder.OrderID != 0 }, time.Second, 10*time.Millisecond,
		"filled order callback should fire")
	assert.Equal(t, types.PositionActionOpenLong, capturedOrder.PositionAction,
		"filled order should carry PositionAction: got=%s", capturedOrder.PositionAction)
}

// TestTakerLimitSell_NoDoubleCountedSurplus verifies that a taker limit SELL
// (limit price below mark) does not double-count the surplus. The fill path
// already credits fillPrice*qty to quote; the old refund path ADDED
// (fillPrice-limitPrice)*qty on top, inflating quote by the surplus twice.
// Regression guard for the SELL-branch removal in the taker-limit refund block.
func TestTakerLimitSell_NoDoubleCountedSurplus(t *testing.T) {
	e := newTestPaperTradeExchange()
	seedKline(t, e, "BTCUSDT", 50000.0)

	// Seed base so the SELL submit path can lock it.
	e.account.AddBalance("BTC", fixedpoint.NewFromFloat(1.0))

	qty := fixedpoint.NewFromFloat(0.1)
	limitPrice := fixedpoint.NewFromFloat(49900.0) // below mark → taker

	usdtBefore, _ := e.account.Balance("USDT")
	btcBefore, _ := e.account.Balance("BTC")

	_, err := e.SubmitOrder(context.Background(), types.SubmitOrder{
		Symbol:   "BTCUSDT",
		Side:     types.SideTypeSell,
		Type:     types.OrderTypeLimit,
		Quantity: qty,
		Price:    limitPrice,
	})
	require.NoError(t, err)

	usdtAfter, _ := e.account.Balance("USDT")
	btcAfter, _ := e.account.Balance("BTC")

	// Expected: sold qty BTC at fillPrice=50000 (mark), minus taker fee.
	// BTC.Available drops by qty (locked + consumed at fill).
	// USDT.Available gains fillPrice*qty - fee.
	// The pre-fix bug added an extra (fillPrice-limitPrice)*qty = 10 USDT.
	fillPrice := fixedpoint.NewFromFloat(50000.0)
	expectedUSDTDelta := fillPrice.Mul(qty).Sub(
		fillPrice.Mul(qty).Mul(fixedpoint.NewFromFloat(paperSpotTakerFeeRate)),
	)
	actualUSDTDelta := usdtAfter.Available.Sub(usdtBefore.Available)
	assert.Truef(t, actualUSDTDelta.Compare(expectedUSDTDelta) == 0,
		"taker limit SELL surplus double-count: want delta=%s, got delta=%s (before=%s after=%s)",
		expectedUSDTDelta.String(), actualUSDTDelta.String(),
		usdtBefore.Available.String(), usdtAfter.Available.String())

	expectedBTCDelta := qty.Neg()
	actualBTCDelta := btcAfter.Available.Sub(btcBefore.Available)
	assert.Truef(t, actualBTCDelta.Compare(expectedBTCDelta) == 0,
		"taker limit SELL base delta: want=%s got=%s",
		expectedBTCDelta.String(), actualBTCDelta.String())
}
