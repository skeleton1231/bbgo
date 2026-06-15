package bbgo

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// hangTimeout is the upper bound for any single lifecycle operation.
// Every test below wraps the call in a select against this timer so a
// regression that blocks indefinitely fails loudly instead of stalling CI.
const hangTimeout = 3 * time.Second

// modeSetup configures a fresh paper trade exchange for one cell of the
// scenario matrix. Each function returns the configured exchange.
type modeSetup func(e *PaperTradeExchange)

var lifecycleModeMatrix = []struct {
	name  string
	setup modeSetup
}{
	{"spot", func(e *PaperTradeExchange) {}},
	{"futures", func(e *PaperTradeExchange) { e.UseFutures() }},
	{"isolated_futures", func(e *PaperTradeExchange) { e.UseIsolatedFutures("BTCUSDT") }},
	{"margin", func(e *PaperTradeExchange) { e.UseMargin() }},
	{"isolated_margin", func(e *PaperTradeExchange) { e.UseIsolatedMargin("BTC") }},
	{"futures_and_margin", func(e *PaperTradeExchange) {
		e.UseFutures()
		e.UseMargin()
	}},
	{"isolated_futures_and_isolated_margin", func(e *PaperTradeExchange) {
		e.UseIsolatedFutures("BTCUSDT")
		e.UseIsolatedMargin("BTC")
	}},
}

// runWithHangGuard invokes f and fails the test if it doesn't return
// within hangTimeout. The guard is on a separate goroutine so a blocked
// f still produces a useful failure message via t.Fatal.
func runWithHangGuard(t *testing.T, op string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(hangTimeout):
		t.Fatalf("%s did not complete within %s — possible container hang", op, hangTimeout)
	}
}

// TestPaperTradeLifecycle_ModeInit_NoHang walks every mode combination and
// asserts that UseFutures/UseMargin/SetLeverage complete immediately. These
// are called by session.InitExchange before the matching engine ever sees a
// kline; a panic or block here kills the container on startup.
func TestPaperTradeLifecycle_ModeInit_NoHang(t *testing.T) {
	for _, m := range lifecycleModeMatrix {
		t.Run(m.name, func(t *testing.T) {
			e := newTestPaperTradeExchange()
			runWithHangGuard(t, "mode setup", func() {
				m.setup(e)
			})

			runWithHangGuard(t, "SetLeverage", func() {
				if err := e.SetLeverage(context.Background(), "BTCUSDT", 5); err != nil {
					t.Fatalf("SetLeverage failed: %v", err)
				}
			})
		})
	}
}

// TestPaperTradeLifecycle_StartBackgroundServices_NoHang verifies that
// StartBackgroundServices returns for every mode combination. Background
// goroutines spawn here in production; a panic on the first tick would
// crash the container within minutes of startup.
func TestPaperTradeLifecycle_StartBackgroundServices_NoHang(t *testing.T) {
	for _, m := range lifecycleModeMatrix {
		t.Run(m.name, func(t *testing.T) {
			e := newTestPaperTradeExchange()
			m.setup(e)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			runWithHangGuard(t, "StartBackgroundServices", func() {
				e.StartBackgroundServices(ctx)
			})
		})
	}
}

// TestPaperTradeLifecycle_BackgroundServices_StopOnCancel asserts that the
// background goroutines actually terminate when ctx is cancelled. We inspect
// runtime stack traces rather than runtime.NumGoroutine() (which counts
// process-wide goroutines and flaps as other tests run). A leaked
// StartBackgroundServices goroutine keeps a stack frame inside the paper
// trade package; we count those specifically.
func TestPaperTradeLifecycle_BackgroundServices_StopOnCancel(t *testing.T) {
	paperGoroutineCount := func() int {
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		stackDump := string(buf[:n])
		// Each goroutine block in the dump starts with "goroutine N [status]:".
		// We split on that marker and count blocks containing paper trade frames.
		blocks := strings.Split(stackDump, "goroutine ")
		count := 0
		for _, b := range blocks {
			if strings.Contains(b, "paper_trade_futures.go") ||
				strings.Contains(b, "paper_trade_exchange.go") {
				count++
			}
		}
		return count
	}

	for _, m := range lifecycleModeMatrix {
		t.Run(m.name, func(t *testing.T) {
			require.Eventually(t, func() bool {
				return paperGoroutineCount() == 0
			}, 1*time.Second, 10*time.Millisecond, "pre-existing paper trade goroutines before test")

			e := newTestPaperTradeExchange()
			m.setup(e)

			ctx, cancel := context.WithCancel(context.Background())
			e.StartBackgroundServices(ctx)
			cancel()

			require.Eventually(t, func() bool {
				return paperGoroutineCount() == 0
			}, 2*time.Second, 10*time.Millisecond,
				"paper trade goroutines leaked after ctx cancel in mode %s", m.name)
		})
	}
}

// TestPaperTradeLifecycle_RestoreFromDB_NilDB verifies RestoreFromDB is a
// no-op when no DB is wired. The session.Init code path can hit this when
// DB_DRIVER is unset; a nil deref here crashes paper-mode containers on
// first restart.
func TestPaperTradeLifecycle_RestoreFromDB_NilDB(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.UseMargin()

	runWithHangGuard(t, "RestoreFromDB(nil)", func() {
		if err := e.RestoreFromDB(context.Background()); err != nil {
			t.Fatalf("RestoreFromDB with nil db returned error: %v", err)
		}
	})
}

// TestPaperTradeLifecycle_RestoreFromDB_EmptySQLite asserts RestoreFromDB
// tolerates an empty SQLite database (no rows, missing tables gracefully
// handled by the query layer). SaaS containers boot from a freshly
// provisioned SQLite file; RestoreFromDB must not panic or hang on it.
//
// go-sqlite3 requires CGO. Without it the driver is a stub; skip so the
// test does not false-positive in CGO_ENABLED=0 builds (e.g. CI without gcc).
func TestPaperTradeLifecycle_RestoreFromDB_EmptySQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "paper.sqlite")

	db, err := sqlx.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Probe the driver — if CGO is disabled, go-sqlite3 returns a stub
	// error on the first real call. Skip in that environment.
	if err := db.Ping(); err != nil {
		t.Skipf("sqlite3 driver unavailable (likely CGO_ENABLED=0): %v", err)
	}

	paperSchema := []string{
		`CREATE TABLE IF NOT EXISTS paper_orders (
			id INTEGER PRIMARY KEY,
			exchange TEXT, order_id INTEGER, client_order_id TEXT,
			order_type TEXT, status TEXT, symbol TEXT,
			price REAL, stop_price REAL, quantity REAL, executed_quantity REAL,
			side TEXT, is_working INTEGER, time_in_force TEXT,
			created_at TEXT, updated_at TEXT,
			is_margin INTEGER, is_futures INTEGER, is_isolated INTEGER,
			order_uuid TEXT, actual_order_id TEXT, strategy_instance_id TEXT,
			user_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS paper_balances (
			currency TEXT PRIMARY KEY,
			available REAL, locked REAL,
			user_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS paper_futures_position_risks (
			id INTEGER PRIMARY KEY,
			symbol TEXT, side TEXT, amount REAL, entry_price REAL,
			leverage INTEGER, unrealized_pnl REAL, liquidation_price REAL,
			notional REAL, margin REAL, margin_ratio REAL,
			is_isolated INTEGER, strategy_instance_id TEXT,
			user_id TEXT, recorded_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS paper_margin_loans (
			id INTEGER PRIMARY KEY, asset TEXT, principle REAL,
			interest REAL, interest_rate REAL, time TEXT,
			user_id TEXT, strategy_instance_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS paper_margin_interests (
			id INTEGER PRIMARY KEY, asset TEXT, principle REAL,
			interest REAL, interest_rate REAL, time TEXT,
			user_id TEXT, strategy_instance_id TEXT
		)`,
	}
	for _, stmt := range paperSchema {
		_, err := db.Exec(stmt)
		require.NoError(t, err, "schema setup failed: %s", stmt)
	}

	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.UseMargin()
	e.SetDB(db, "paper_", "")

	runWithHangGuard(t, "RestoreFromDB(empty sqlite)", func() {
		if err := e.RestoreFromDB(context.Background()); err != nil {
			t.Fatalf("RestoreFromDB on empty sqlite returned error: %v", err)
		}
	})
}

// TestPaperTradeLifecycle_ConcurrentKLineFeed_NoDeadlock drives OnKLineClosed
// from many goroutines at once under -race. The matching engine takes locks
// in a specific order (book.mu → PaperTradeExchange.mu); a regression in
// that ordering would deadlock under load and stall the container.
func TestPaperTradeLifecycle_ConcurrentKLineFeed_NoDeadlock(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.UseMargin()

	seedKline(t, e, "BTCUSDT", 50000.0)
	seedKline(t, e, "ETHUSDT", 3000.0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.StartBackgroundServices(ctx)

	const feeders = 8
	const klinesPerFeeder = 50

	// Track fills via the matching book's OnTradeUpdate rather than the
	// exchange-level OnKLineClosed (which is the entry point, not a hook).
	btcBook, _ := e.matchingBook("BTCUSDT")
	ethBook, _ := e.matchingBook("ETHUSDT")
	btcBook.OnTradeUpdate(func(_ types.Trade) {})
	ethBook.OnTradeUpdate(func(_ types.Trade) {})

	runWithHangGuard(t, "concurrent kline feed", func() {
		var wg sync.WaitGroup
		wg.Add(feeders)
		for i := 0; i < feeders; i++ {
			go func(feederID int) {
				defer wg.Done()
				for j := 0; j < klinesPerFeeder; j++ {
					sym := "BTCUSDT"
					price := 50000.0 + float64(j)
					if feederID%2 == 1 {
						sym = "ETHUSDT"
						price = 3000.0 + float64(j)
					}
					now := time.Now()
					kline := types.KLine{
						Symbol:      sym,
						Interval:    "1m",
						Open:        fixedpoint.NewFromFloat(price),
						Close:       fixedpoint.NewFromFloat(price),
						High:        fixedpoint.NewFromFloat(price),
						Low:         fixedpoint.NewFromFloat(price),
						Volume:      fixedpoint.NewFromFloat(1.0),
						QuoteVolume: fixedpoint.NewFromFloat(price),
						EndTime:     types.Time(now),
						StartTime:   types.Time(now),
						Closed:      true,
					}
					e.OnKLineClosed(kline)
				}
			}(i)
		}
		wg.Wait()
	})

	// filled may be 0 if no resting orders cross — the point of this test
	// is no deadlock under concurrent feed, not that fills happened.
}

// TestPaperTradeLifecycle_BlockingNavCallback_DoesNotDeadlockOthers checks
// that a slow OnPeriodicNAVRecord (e.g. behind a stalled Supabase REST
// call) cannot block applyFundingRate or updateMarginInterest. Production
// wires these callbacks to network calls; if they shared a lock with the
// timer work, a network stall would freeze the whole paper engine.
func TestPaperTradeLifecycle_BlockingNavCallback_DoesNotDeadlockOthers(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.UseMargin()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)
	state.EntryPrice = fixedpoint.NewFromFloat(50000.0)
	state.LastFundingTime = lastFundingSlotUTC(time.Now())

	marginState := e.getOrCreateMarginState("USDT")
	marginState.Borrowed = fixedpoint.NewFromFloat(1000.0)
	marginState.InterestRate = fixedpoint.NewFromFloat(0.0001)
	marginState.LastAccrual = time.Now().Add(-2 * time.Hour)

	navBlocked := atomic.Bool{}
	e.OnPeriodicNAVRecord = func(time.Time) {
		navBlocked.Store(true)
		// Hold the callback for the entire test window. This simulates a
		// stuck Supabase insert that never returns.
		time.Sleep(2 * time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				e.OnPeriodicNAVRecord(t)
			}
		}
	}()

	require.Eventually(t, func() bool { return navBlocked.Load() },
		1*time.Second, 10*time.Millisecond, "NAV callback should have started by now")

	runWithHangGuard(t, "applyFundingRate (with blocked NAV)", e.applyFundingRate)
	runWithHangGuard(t, "updateMarginInterest (with blocked NAV)", e.updateMarginInterest)

	cancel()
}

// TestPaperTradeLifecycle_ApplyFundingRate_CallbackTakesLock_NoDeadlock
// simulates a misbehaving OnFundingPayment callback that takes e.mu again.
// The current implementation emits callbacks outside the lock so this should
// not deadlock — but it's exactly the kind of regression a refactor could
// silently introduce.
func TestPaperTradeLifecycle_ApplyFundingRate_CallbackTakesLock_NoDeadlock(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)
	state.EntryPrice = fixedpoint.NewFromFloat(50000.0)
	state.LastFundingTime = time.Time{}

	e.OnFundingPayment = func(_ types.FundingPayment) {
		e.mu.Lock()
		_ = e.futuresSettings
		e.mu.Unlock()
	}

	runWithHangGuard(t, "applyFundingRate with reentrant callback", e.applyFundingRate)
}

// TestGracefulShutdown_Empty_ReturnsImmediately is a regression guard for
// the empty-shutdown path. The Trader wires Shutdown(ctx) into the SIGTERM
// handler; an empty-shutdown panic would prevent the container from ever
// receiving SIGTERM cleanly.
func TestGracefulShutdown_Empty_ReturnsImmediately(t *testing.T) {
	g := &GracefulShutdown{}
	runWithHangGuard(t, "empty GracefulShutdown.Shutdown", func() {
		g.Shutdown(context.Background())
	})
}

// TestGracefulShutdown_RunsCallbacksInParallel verifies that N slow
// callbacks finish in roughly the time of one, not N times as long. The
// current implementation launches each callback on its own goroutine; if a
// refactor ever turned it into a serial loop the container's SIGTERM window
// would expire on shutdown.
func TestGracefulShutdown_RunsCallbacksInParallel(t *testing.T) {
	g := &GracefulShutdown{}

	const callbacks = 5
	const eachDelay = 200 * time.Millisecond

	for i := 0; i < callbacks; i++ {
		cb := func(_ context.Context, wg *sync.WaitGroup) {
			defer wg.Done()
			time.Sleep(eachDelay)
		}
		g.shutdownCallbacks = append(g.shutdownCallbacks, cb)
	}

	start := time.Now()
	runWithHangGuard(t, "GracefulShutdown with slow callbacks", func() {
		g.Shutdown(context.Background())
	})
	elapsed := time.Since(start)

	maxExpected := eachDelay + 100*time.Millisecond
	assert.Less(t, elapsed, maxExpected,
		"shutdown of %d parallel callbacks took %s; expected ≤ %s — callbacks not running in parallel",
		callbacks, elapsed, maxExpected)
}

// TestPaperTradeLifecycle_GoroutineCountStable_AfterManyCycles runs the
// full init → start → cancel cycle 20 times per mode and asserts the
// paper-trade goroutine count returns to zero. This catches slow leaks
// that single-iteration tests miss. Uses stack-trace inspection rather
// than runtime.NumGoroutine() because the global count is noisy.
func TestPaperTradeLifecycle_GoroutineCountStable_AfterManyCycles(t *testing.T) {
	paperGoroutineCount := func() int {
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		stackDump := string(buf[:n])
		blocks := strings.Split(stackDump, "goroutine ")
		count := 0
		for _, b := range blocks {
			if strings.Contains(b, "paper_trade_futures.go") ||
				strings.Contains(b, "paper_trade_exchange.go") {
				count++
			}
		}
		return count
	}

	for _, m := range lifecycleModeMatrix {
		t.Run(m.name, func(t *testing.T) {
			for cycle := 0; cycle < 20; cycle++ {
				e := newTestPaperTradeExchange()
				m.setup(e)

				ctx, cancel := context.WithCancel(context.Background())
				e.StartBackgroundServices(ctx)
				cancel()
			}

			require.Eventually(t, func() bool {
				return paperGoroutineCount() == 0
			}, 2*time.Second, 10*time.Millisecond,
				"paper trade goroutines leaked after 20 start/cancel cycles in mode %s",
				m.name)
		})
	}
}

// TestPaperTradeLifecycle_FuturesPositionRisk_AfterFundingAndInterest verifies
// that QueryPositionRisk still returns cleanly after funding + interest fire.
// A bug that leaves e.mu locked after the timer tick would make this block
// forever in production.
func TestPaperTradeLifecycle_FuturesPositionRisk_AfterFundingAndInterest(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()
	e.UseMargin()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.PositionAmount = fixedpoint.NewFromFloat(0.5)
	state.EntryPrice = fixedpoint.NewFromFloat(50000.0)
	state.LastFundingTime = time.Time{}

	runWithHangGuard(t, "applyFundingRate", e.applyFundingRate)
	runWithHangGuard(t, "QueryPositionRisk post-funding", func() {
		_, err := e.QueryPositionRisk(context.Background(), "BTCUSDT")
		require.NoError(t, err)
	})
}

// TestPaperTradeLifecycle_ConcurrentSubmitAndCancel_AllModes drives the
// matching engine with submit + cancel under all mode combinations. The
// matching engine is the lock-hotspot most likely to deadlock; exercising
// it concurrently across modes is the strongest signal we can get without
// spinning up a full Trader.
func TestPaperTradeLifecycle_ConcurrentSubmitAndCancel_AllModes(t *testing.T) {
	for _, m := range lifecycleModeMatrix {
		t.Run(m.name, func(t *testing.T) {
			e := newTestPaperTradeExchange()
			m.setup(e)
			seedKline(t, e, "BTCUSDT", 50000.0)

			if e.GetFuturesSettings().IsFutures {
				require.NoError(t, e.SetLeverage(context.Background(), "BTCUSDT", 5))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e.StartBackgroundServices(ctx)

			const workers = 6
			const itersPerWorker = 30

			runWithHangGuard(t, "concurrent submit/cancel", func() {
				var wg sync.WaitGroup
				wg.Add(workers)
				for i := 0; i < workers; i++ {
					go func(workerID int) {
						defer wg.Done()
						for j := 0; j < itersPerWorker; j++ {
							price := fixedpoint.NewFromFloat(40000.0 + float64(workerID*100))
							submit := types.SubmitOrder{
								Symbol:   "BTCUSDT",
								Side:     types.SideTypeBuy,
								Type:     types.OrderTypeLimit,
								Quantity: fixedpoint.NewFromFloat(0.01),
								Price:    price,
							}
							order, err := e.SubmitOrder(ctx, submit)
							if err != nil {
								continue
							}
							_ = e.CancelOrders(ctx, *order)
						}
					}(i)
				}
				wg.Wait()
			})
		})
	}
}
