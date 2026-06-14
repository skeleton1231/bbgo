package bbgo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TestLastFundingSlotUTC verifies the helper snaps an arbitrary time to the
// most recent UTC 00/08/16 funding boundary.
func TestLastFundingSlotUTC(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"before first slot", "2026-06-13T05:59:59Z", "2026-06-13T00:00:00Z"},
		{"just after midnight", "2026-06-13T00:30:00Z", "2026-06-13T00:00:00Z"},
		{"mid morning", "2026-06-13T09:15:00Z", "2026-06-13T08:00:00Z"},
		{"just before 16:00", "2026-06-13T15:59:59Z", "2026-06-13T08:00:00Z"},
		{"evening", "2026-06-13T20:45:00Z", "2026-06-13T16:00:00Z"},
		{"exactly 16:00", "2026-06-13T16:00:00Z", "2026-06-13T16:00:00Z"},
		{"non-utc input stays correct", "2026-06-13T18:00:00+08:00", "2026-06-13T08:00:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, err := time.Parse(time.RFC3339, c.in)
			assert.NoError(t, err)
			want, err := time.Parse(time.RFC3339, c.want)
			assert.NoError(t, err)
			got := lastFundingSlotUTC(in)
			assert.True(t, got.Equal(want), "got %s want %s", got.UTC(), want.UTC())
		})
	}
}

// TestApplyFundingRate_AlignsToUTCSlot verifies that funding is applied and
// LastFundingTime snaps to the UTC slot, not the wall-clock run time.
func TestApplyFundingRate_AlignsToUTCSlot(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)
	state.LastFundingTime = time.Time{} // zero → always fires

	var fired bool
	e.OnFundingPayment = func(_ types.FundingPayment) { fired = true }

	balBefore, _ := e.account.Balance("USDT")
	e.applyFundingRate()

	assert.True(t, fired, "funding should fire when LastFundingTime is zero")
	balAfter, _ := e.account.Balance("USDT")
	assert.True(t, balAfter.Available.Compare(balBefore.Available) < 0, "long balance should decrease after paying funding")
	slot := lastFundingSlotUTC(time.Now())
	assert.True(t, state.LastFundingTime.Equal(slot),
		"LastFundingTime should be UTC slot %s, got %s", slot, state.LastFundingTime)
}

// TestGetOrCreateFuturesState_LastFundingTimeInitialized verifies that a
// freshly-created state has LastFundingTime set to the current UTC funding
// slot. Without this, a position opened at e.g. 09:00 UTC pays funding for
// the 08:00 slot on the next timer tick — a charge for a period when the
// position didn't even exist.
func TestGetOrCreateFuturesState_LastFundingTimeInitialized(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")

	want := lastFundingSlotUTC(time.Now())
	assert.False(t, state.LastFundingTime.IsZero(),
		"LastFundingTime must be non-zero on new state to prevent charging for past slots")
	assert.True(t, state.LastFundingTime.Equal(want),
		"LastFundingTime should be current slot %s, got %s", want, state.LastFundingTime)
}

// TestApplyFundingRate_NewPositionSkipsCurrentSlot verifies the end-to-end
// behavior: a freshly-created position does not pay funding for the slot
// during which it was opened.
func TestApplyFundingRate_NewPositionSkipsCurrentSlot(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)

	var fired bool
	e.OnFundingPayment = func(_ types.FundingPayment) { fired = true }

	balBefore, _ := e.account.Balance("USDT")
	e.applyFundingRate()
	balAfter, _ := e.account.Balance("USDT")

	assert.False(t, fired,
		"funding must not fire for a position created in the current slot")
	assert.True(t, balAfter.Available.Compare(balBefore.Available) == 0,
		"balance must be unchanged when position was just created in current slot")
}

// TestApplyFundingRate_SkipsAlreadySettledSlot verifies that once funding has
// been applied for the current UTC slot, a second call in the same slot is a no-op.
func TestApplyFundingRate_SkipsAlreadySettledSlot(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)
	state.LastFundingTime = lastFundingSlotUTC(time.Now()) // already funded this slot

	var fired bool
	e.OnFundingPayment = func(_ types.FundingPayment) { fired = true }

	balBefore, _ := e.account.Balance("USDT")
	e.applyFundingRate()

	assert.False(t, fired, "funding must not fire twice in the same UTC slot")
	balAfter, _ := e.account.Balance("USDT")
	assert.True(t, balAfter.Available.Compare(balBefore.Available) == 0, "balance must be unchanged when slot already settled")
}

// TestApplyFundingRate_ShortReceivesFunding verifies that a short position
// receives funding when the rate is positive (longs pay shorts).
func TestApplyFundingRate_ShortReceivesFunding(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(-0.001) // short
	state.LastFundingTime = time.Time{}                    // force funding to fire this slot

	balBefore, _ := e.account.Balance("USDT")
	e.applyFundingRate()

	balAfter, _ := e.account.Balance("USDT")
	assert.True(t, balAfter.Available.Compare(balBefore.Available) > 0, "short balance should increase when receiving funding")
}

// TestApplyFundingRate_EmitsBalanceUpdate verifies that funding settlement
// emits a balance update so the SaaS frontend sees the adjusted balance
// without waiting for the next trade.
func TestApplyFundingRate_EmitsBalanceUpdate(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	stream := types.NewStandardStream()
	e.BindUserData(&stream)

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)
	state.LastFundingTime = time.Time{} // force funding to fire this slot

	var balanceUpdates int
	stream.OnBalanceUpdate(func(_ types.BalanceMap) { balanceUpdates++ })

	e.applyFundingRate()

	assert.Greater(t, balanceUpdates, 0, "balance update should fire after funding settlement")
}

// openTestSQLiteDB returns an in-memory SQLite DB, skipping the test when
// CGO (and thus go-sqlite3) is unavailable.
func openTestSQLiteDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Connect("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("sqlite3 unavailable (CGO disabled?): %v", err)
	}
	return db
}

func seedFuturesPositionRisk(t *testing.T, db *sqlx.DB) {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE paper_futures_position_risks (
		symbol TEXT,
		position_side TEXT,
		leverage TEXT,
		entry_price TEXT,
		position_amount TEXT,
		margin_asset TEXT,
		strategy_instance_id TEXT
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO paper_futures_position_risks
		(symbol, position_side, leverage, entry_price, position_amount, margin_asset, strategy_instance_id)
		VALUES ('BTCUSDT', 'BOTH', '20', '60000', '0.001', 'USDT', 'test-instance')`)
	require.NoError(t, err)
}

// TestRestoreFuturesPositions_LastFundingTimeInitialized verifies that a
// position restored from the DB has LastFundingTime set to the most recent
// UTC funding slot. Without this, a container restart mid-window re-applies
// funding for the current slot — a double charge.
func TestRestoreFuturesPositions_LastFundingTimeInitialized(t *testing.T) {
	db := openTestSQLiteDB(t)
	defer db.Close()
	seedFuturesPositionRisk(t, db)

	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetDB(db, "paper_", "test-user")

	require.NoError(t, e.restoreFuturesPositions(context.Background()))

	state, ok := e.futuresStates["BTCUSDT"]
	require.True(t, ok, "position should be restored")

	want := lastFundingSlotUTC(time.Now())
	assert.False(t, state.LastFundingTime.IsZero(),
		"LastFundingTime must be non-zero after restore to prevent double funding")
	assert.True(t, state.LastFundingTime.Equal(want),
		"LastFundingTime should be last funding slot %s, got %s", want, state.LastFundingTime)
}

// TestRestoreFuturesPositions_NoDoubleFunding verifies the end-to-end
// invariant: after restore, applyFundingRate does not charge the current slot.
func TestRestoreFuturesPositions_NoDoubleFunding(t *testing.T) {
	db := openTestSQLiteDB(t)
	defer db.Close()
	seedFuturesPositionRisk(t, db)

	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetDB(db, "paper_", "test-user")

	require.NoError(t, e.restoreFuturesPositions(context.Background()))

	var fired bool
	e.OnFundingPayment = func(_ types.FundingPayment) { fired = true }

	balBefore, _ := e.account.Balance("USDT")
	e.applyFundingRate()
	balAfter, _ := e.account.Balance("USDT")

	assert.False(t, fired, "funding must not fire immediately after restore")
	assert.True(t, balAfter.Available.Compare(balBefore.Available) == 0,
		"balance must be unchanged after restore + immediate funding check")
}

// seedMarginLoansRepays inserts two loans and one repay for USDT and one loan
// for BTC so restoreMarginStates has historical data to sum.
func seedMarginLoansRepays(t *testing.T, db *sqlx.DB) {
	t.Helper()
	mustExec := func(q string) {
		t.Helper()
		if _, err := db.Exec(q); err != nil {
			require.NoError(t, err)
		}
	}
	mustExec(`CREATE TABLE IF NOT EXISTS paper_margin_loans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		transaction_id INTEGER,
		asset TEXT,
		isolated_symbol TEXT DEFAULT '',
		principle TEXT,
		time TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`)
	mustExec(`CREATE TABLE IF NOT EXISTS paper_margin_repays (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		transaction_id INTEGER,
		asset TEXT,
		isolated_symbol TEXT DEFAULT '',
		principle TEXT,
		time TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`)
	mustExec(`INSERT INTO paper_margin_loans (transaction_id, asset, principle) VALUES (1, 'USDT', '1000')`)
	mustExec(`INSERT INTO paper_margin_loans (transaction_id, asset, principle) VALUES (2, 'USDT', '500')`)
	mustExec(`INSERT INTO paper_margin_loans (transaction_id, asset, principle) VALUES (3, 'BTC', '0.5')`)
	mustExec(`INSERT INTO paper_margin_repays (transaction_id, asset, principle) VALUES (101, 'USDT', '300')`)
}

// TestRestoreMarginStates_ReplaysLoansMinusRepays verifies that
// restoreMarginStates correctly sums paper_margin_loans minus
// paper_margin_repays per asset. Regression guard for the restart-time
// amnesia where marginStates started empty and subsequent borrows "forgot"
// prior debt (net effect: free money for users who restart mid-borrow).
func TestRestoreMarginStates_ReplaysLoansMinusRepays(t *testing.T) {
	db := openTestSQLiteDB(t)
	defer db.Close()
	seedMarginLoansRepays(t, db)

	// Create empty interests table so restoreMarginStates can query it.
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS paper_margin_interests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		transaction_id INTEGER,
		asset TEXT,
		isolated_symbol TEXT DEFAULT '',
		principle TEXT,
		interest TEXT,
		interest_rate TEXT,
		time TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`)
	require.NoError(t, err)

	e := newTestPaperTradeExchange()
	e.UseMargin()
	e.SetDB(db, "paper_", "test-user")

	require.NoError(t, e.restoreMarginStates(context.Background()))

	usdtState, ok := e.marginStates["USDT"]
	require.True(t, ok, "USDT margin state should be restored")
	// 1000 + 500 - 300 = 1200
	wantUSDT := fixedpoint.NewFromFloat(1200.0)
	assert.Truef(t, usdtState.Borrowed.Compare(wantUSDT) == 0,
		"USDT Borrowed: want=%s got=%s", wantUSDT.String(), usdtState.Borrowed.String())
	assert.False(t, usdtState.LastAccrual.IsZero(),
		"LastAccrual should be set so the interest clock starts fresh")
	assert.True(t, usdtState.InterestRate.Compare(fixedpoint.MustNewFromString(defaultHourlyMarginRate)) == 0,
		"InterestRate should be restored to default")

	btcState, ok := e.marginStates["BTC"]
	require.True(t, ok, "BTC margin state should be restored")
	wantBTC := fixedpoint.NewFromFloat(0.5)
	assert.Truef(t, btcState.Borrowed.Compare(wantBTC) == 0,
		"BTC Borrowed: want=%s got=%s", wantBTC.String(), btcState.Borrowed.String())
}

// TestRestoreMarginStates_LastAccrualFromInterestRow verifies that
// restoreMarginStates restores LastAccrual from the latest interest row
// rather than time.Now(). Without this, every restart gifts the borrower
// the entire restart gap as interest-free.
func TestRestoreMarginStates_LastAccrualFromInterestRow(t *testing.T) {
	db := openTestSQLiteDB(t)
	defer db.Close()

	mustExec := func(q string) {
		t.Helper()
		if _, err := db.Exec(q); err != nil {
			require.NoError(t, err)
		}
	}
	mustExec(`CREATE TABLE paper_margin_loans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		transaction_id INTEGER,
		asset TEXT,
		isolated_symbol TEXT DEFAULT '',
		principle TEXT,
		time TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`)
	mustExec(`CREATE TABLE paper_margin_repays (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		transaction_id INTEGER,
		asset TEXT,
		isolated_symbol TEXT DEFAULT '',
		principle TEXT,
		time TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`)
	mustExec(`CREATE TABLE paper_margin_interests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		transaction_id INTEGER,
		asset TEXT,
		isolated_symbol TEXT DEFAULT '',
		principle TEXT,
		interest TEXT,
		interest_rate TEXT,
		time TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`)

	// Loan at 3 hours ago.
	loanTime := time.Now().UTC().Add(-3 * time.Hour)
	mustExec(fmt.Sprintf(
		`INSERT INTO paper_margin_loans (transaction_id, asset, principle, time) VALUES (1, 'USDT', '1000', '%s')`,
		loanTime.Format("2006-01-02 15:04:05")))

	// Interest accrued at 1 hour ago (so 2 hours of interest were already charged).
	interestTime := time.Now().UTC().Add(-1 * time.Hour)
	mustExec(fmt.Sprintf(
		`INSERT INTO paper_margin_interests (transaction_id, asset, principle, interest, interest_rate, time) VALUES (10, 'USDT', '1000', '0.2', '0.0001', '%s')`,
		interestTime.Format("2006-01-02 15:04:05")))

	e := newTestPaperTradeExchange()
	e.UseMargin()
	e.SetDB(db, "paper_", "test-user")

	require.NoError(t, e.restoreMarginStates(context.Background()))

	usdtState, ok := e.marginStates["USDT"]
	require.True(t, ok, "USDT margin state should be restored")

	// LastAccrual should be ~1h ago (the interest row), NOT time.Now().
	// Allow 5-minute skew for sqlite timestamp parsing.
	now := time.Now()
	elapsed := now.Sub(usdtState.LastAccrual)
	assert.Truef(t, elapsed > 30*time.Minute,
		"LastAccrual should be ~1h ago (from interest row), got elapsed=%v (LastAccrual=%s, now=%s)",
		elapsed, usdtState.LastAccrual, now)
	assert.Truef(t, elapsed < 2*time.Hour,
		"LastAccrual should not be older than the interest row (~1h), got elapsed=%v", elapsed)
}

// TestBorrowMarginAsset_PersistsBalancesToDB verifies that borrow/repay
// operations persist balance changes to paper_balances via
// EmitBalanceUpdateFromAccount -> syncBalances. Without this, a restart after
// a borrow loses the borrowed funds from the wallet while the debt is still
// tracked — net effect: debt with no money.
func TestBorrowMarginAsset_PersistsBalancesToDB(t *testing.T) {
	db := openTestSQLiteDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE paper_balances (
		currency TEXT PRIMARY KEY,
		total TEXT,
		available TEXT,
		locked TEXT)`)
	require.NoError(t, err)

	e := newTestPaperTradeExchange()
	e.UseMargin()
	e.SetDB(db, "paper_", "test-user")

	borrowAmount := fixedpoint.NewFromFloat(1000.0)
	require.NoError(t, e.BorrowMarginAsset(context.Background(), "USDT", borrowAmount))

	var dbTotal, dbAvailable string
	err = db.QueryRow(`SELECT total, available FROM paper_balances WHERE currency = 'USDT'`).Scan(&dbTotal, &dbAvailable)
	require.NoError(t, err, "borrow should persist USDT balance to paper_balances")

	dbAvail, _ := fixedpoint.NewFromString(dbAvailable)
	assert.Truef(t, dbAvail.Compare(borrowAmount) == 0,
		"paper_balances USDT available should be %s after borrow, got %s",
		borrowAmount.String(), dbAvailable)
}

// TestUpsertBalances_PersistsZeroBalance verifies that a balance going to
// zero is persisted (not skipped). Without this, a user who sells all their
// BTC has the stale non-zero BTC row in paper_balances, which restores as
// phantom BTC after a restart.
func TestUpsertBalances_PersistsZeroBalance(t *testing.T) {
	db := openTestSQLiteDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE paper_balances (
		currency TEXT PRIMARY KEY,
		total TEXT,
		available TEXT,
		locked TEXT)`)
	require.NoError(t, err)

	e := newTestPaperTradeExchange()
	e.SetDB(db, "paper_", "test-user")

	// Add BTC, sync, verify it's persisted.
	e.account.AddBalance("BTC", fixedpoint.NewFromFloat(1.0))
	e.syncBalances()

	var btcAvail string
	err = db.QueryRow(`SELECT available FROM paper_balances WHERE currency = 'BTC'`).Scan(&btcAvail)
	require.NoError(t, err)
	assert.Equal(t, "1", btcAvail, "BTC should be persisted as 1.0")

	// Sell all BTC → balance goes to zero.
	e.account.UseLockedBalance("BTC", fixedpoint.NewFromFloat(1.0))
	e.syncBalances()

	err = db.QueryRow(`SELECT available FROM paper_balances WHERE currency = 'BTC'`).Scan(&btcAvail)
	require.NoError(t, err, "BTC row should still exist after zeroing")
	assert.Equal(t, "0", btcAvail,
		"BTC available should be 0 in DB after selling all — stale value means phantom BTC on restart")
}

// TestRestoreFuturesPositions_FiltersByStrategyInstance verifies that when
// BBGO_STRATEGY_INSTANCE_ID is set, restoreFuturesPositions only pulls
// positions belonging to that instance. Without this filter, a SaaS user
// running multiple paper bot containers would have a restart pull in
// positions from their OTHER containers — corrupting position state.
func TestRestoreFuturesPositions_FiltersByStrategyInstance(t *testing.T) {
	db := openTestSQLiteDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE paper_futures_position_risks (
		symbol TEXT,
		position_side TEXT,
		leverage TEXT,
		entry_price TEXT,
		position_amount TEXT,
		margin_asset TEXT,
		strategy_instance_id TEXT
	)`)
	require.NoError(t, err)

	// Two positions for BTCUSDT under different instances.
	_, err = db.Exec(`INSERT INTO paper_futures_position_risks
		(symbol, position_side, leverage, entry_price, position_amount, margin_asset, strategy_instance_id)
		VALUES ('BTCUSDT', 'BOTH', '20', '60000', '0.001', 'USDT', 'instance-A')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO paper_futures_position_risks
		(symbol, position_side, leverage, entry_price, position_amount, margin_asset, strategy_instance_id)
		VALUES ('ETHUSDT', 'BOTH', '10', '3000', '0.5', 'USDT', 'instance-B')`)
	require.NoError(t, err)

	t.Setenv("BBGO_STRATEGY_INSTANCE_ID", "instance-A")

	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.SetDB(db, "paper_", "test-user")

	require.NoError(t, e.restoreFuturesPositions(context.Background()))

	_, hasBTC := e.futuresStates["BTCUSDT"]
	assert.True(t, hasBTC, "BTCUSDT (instance-A) should be restored")

	_, hasETH := e.futuresStates["ETHUSDT"]
	assert.False(t, hasETH, "ETHUSDT (instance-B) must NOT be restored — it belongs to another container")
}
