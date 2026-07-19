# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

BBGO is a Go-based cryptocurrency trading framework supporting 8 exchanges (Binance, OKEx, KuCoin, MAX, Bitget, Bybit, Coinbase, Bitfinex) with 40+ built-in strategies, backtesting, and a web dashboard.

This repo (`skeleton1231/bbgo`, forked from [c9s/bbgo](https://github.com/c9s/bbgo)) is the upstream for the **SaaS deployment** (`saas/` directory). Key SaaS-specific additions:

- **Supabase direct write** (`DB_DRIVER=supabase`) — orders/trades/positions/profits written to Supabase via REST API; paper mode writes to `paper_*` tables via `SUPABASE_TABLE_PREFIX`
- **Paper trade engine** (`pkg/bbgo/paper_trade_exchange.go` + `paper_trade_futures.go`) — kline-driven matching engine for simulation mode with full futures/margin support: short selling, leverage-based margin locking, position tracking (weighted average entry, unrealized PnL, liquidation price), margin borrow/repay with hourly interest accrual, deadlock-free callback emission
- **Strategy instance IDs** (`pkg/instanceid/`) — deterministic instance IDs computed from strategy + symbol + config; propagated to orders/trades for per-instance data isolation
- **gRPC market data service** — shared `MarketDataService` with 3-layer kline cache (memory → SQLite → API)
- **Backtest isolation** — per-job config/database to prevent concurrent overwrite
- **Strategy hardening** — defaults, validation, and nil guards for backtest-crashing strategies
- **Supabase service parity** — full SQL-mode service alignment (NAV history, rewards, deposits, margin, futures position risks)

Module path: `github.com/c9s/bbgo`

## Build Commands

```bash
# Fast local build (no web dashboard — preferred for development)
make bbgo-slim          # → build/bbgo/bbgo-slim

# Full build with embedded web dashboard (requires frontend assets)
make bbgo               # → build/bbgo/bbgo

# Build with high-precision decimal math
make bbgo-slim-dnum

# Rebuild embedded frontend assets (only needed for full build)
make static
```

Build tags: `web` (embed dashboard), `release` (version stamp), `dnum` (decimal math).

## Testing

```bash
# Run all tests
go test ./pkg/...

# Run a specific package/test (preferred during iteration)
go test ./pkg/bbgo -v -run TestLoadConfig

# With race detector and coverage (what CI runs)
go test -count 3 -race -coverprofile coverage.txt -covermode atomic ./pkg/...

# Run with dnum tag (separate test set)
go test -race -tags dnum ./pkg/...
```

Integration tests require exchange API credentials via env vars (e.g., `BINANCE_API_KEY`). Most unit tests run without credentials.

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `KLINE_DB_PATH` | SQLite database path for the 3-layer kline cache (memory → SQLite → API) in the gRPC server. When set, the gRPC `MarketDataService` caches klines to disk. Leave unset for memory-only caching. |

CI also requires MySQL and Redis for full test runs (see `.github/workflows/go.yml`).

### Test Helpers (`pkg/testing/`)

Two sub-packages provide reusable test utilities:

**`testhelper`** — Domain-specific builders and assertions for trading types:

- `testhelper.Market("BTCUSDT")` / `testhelper.Ticker("ETHUSDT")` — pre-defined market and ticker fixtures (BTCUSDT, ETHUSDT, USDCUSDT, USDTTWD, BTCTWD)
- `testhelper.Number(99.5)` — flexible conversion (string/int/float) to `fixedpoint.Value`
- `testhelper.Balance("BTC", Number(10))` — create a `types.Balance` with zeroed metadata fields
- `testhelper.BalancesFromText("BTC, 10.5\nETH, 100")` — parse text into `types.BalanceMap`
- `testhelper.PriceVolumeSliceFromText("100, 10\n105, 20")` — parse price/volume pairs (supports `//` comments)
- `testhelper.AssertOrdersPriceSideQuantityFromText(t, "BUY, 100, 10\nSELL, 105, 20", orders)` — assert order slice against text spec
- `testhelper.MatchOrder(order)` / `testhelper.Catch(func(x any){...})` — testify matchers for use with mocks

**`httptesting`** — HTTP client mocking and record/replay:

- `httptesting.HttpClientWithJson(data)` — client that returns JSON for any request
- `httptesting.HttpClientSaver(&req, content)` — client that captures the request for inspection
- `MockTransport` — register handlers per method/path: `transport.GET("/api/v1/orders", handlerFunc)`
- `Recorder` — records live HTTP interactions, saves to JSON, replays later via `MockTransport.LoadFromRecorder()`. Automatically strips credential headers. Control with `TEST_HTTP_RECORD=1` env var.
- `BuildResponseJson(code, payload)` / `SetHeader(resp, k, v)` — chainable response builders

### Paper-mode strategy gate (`pkg/cmd/strategy/paper_smoke_test.go`)

`TestPaperSmoke_AllStrategies_RunWithoutPanic` is the **runtime** counterpart to `registry_lifecycle_test.go`. For every registered single-exchange strategy it wires the strategy to a paper-backed `ExchangeSession`, runs `Defaults` → `Initialize` → `Subscribe` → `Run`, feeds 220 deterministic synthetic klines through a controllable `StandardStream` (which drives both the strategy callbacks and the paper matching engine), and asserts no panic, no hang, clean ctx-cancel exit, and **no negative balance**. Coverage is a **denylist** (`paperSmokeSkip`) — a new strategy is covered by default; adding a skip requires a concrete reason and is Phase 2/3 backlog. Run with `go test ./pkg/cmd/strategy/ -run TestPaperSmoke -timeout 600s`. See `saas/docs/paper-strategy-audit.md` for the strategy-by-strategy status matrix and the skip backlog.

## Linting

CI uses **revive** for linting and **golangci-lint** with: staticcheck, bodyclose, contextcheck, dupword, decorder, goconst, govet, gosec, misspell. Format with `gofmt -s -w`.

## Code Generation

Several generators produce committed files — re-run and commit when touching their inputs:

```bash
# SQL migrations (after changing migrations/*.sql)
make migrations         # uses rockhopper → pkg/migrations/{mysql,sqlite3}/

# gRPC protobuf (after changing pkg/pb/*.proto)
make grpc-go            # requires protoc; install deps: make install-grpc-tools

# Go generate (requestgen, callbackgen) — run in the relevant package
go generate ./pkg/exchange/max/...
```

- **requestgen**: generates HTTP API request builders from `//go:generate requestgen` directives. Found in exchange API packages and data sources.
- **callbackgen**: generates `OnXxx()` callback registrations from `//go:generate callbackgen -type TypeName` directives. Used for event-driven types like `TradeCollector`, `ActiveOrderBook`, `GracefulShutdown`, etc. Touch the source type, then run `go generate` in that package to regenerate the `*_callbacks.go` file.

## Architecture

### Core Flow

```
CLI (cmd/bbgo → pkg/cmd)
  → Environment (pkg/bbgo/environment.go) — manages exchange sessions & services
    → Trader (pkg/bbgo/trader.go) — loads strategies, manages lifecycle
      → Strategies implement SingleExchangeStrategy.Run() or CrossExchangeStrategy.CrossRun()
```

### Startup Sequence

1. Load config from YAML
2. Allocate and initialize `ExchangeSession` objects (one per exchange account)
3. Add sessions to `Environment`
4. `Environment` initializes `Trader`
5. `Trader` calls strategy `Subscribe()` to register market data interests, then opens WebSocket connections
6. `Trader` calls strategy `Run()` — strategies set up callbacks on stream/book events and start trading

### Key Packages

| Package | Purpose |
|---------|---------|
| `pkg/bbgo/` | Core engine: Environment, Trader, Config, strategy registry, OrderExecutor, ExchangeSession, paper trade engine (spot + futures + margin) |
| `pkg/types/` | Shared types: Exchange interface, Order, Trade, KLine, Balance, Stream, fixedpoint |
| `pkg/exchange/` | Exchange adapters (REST + WebSocket); factory in `factory.go`. Each exchange has its own sub-package |
| `pkg/strategy/` | Built-in strategies (grid2, xmaker, bollmaker, supertrend, etc.) |
| `pkg/indicator/` | Technical indicators (SMA, EMA, MACD, RSI, Bollinger, etc.) |
| `pkg/service/` | Persistence and business logic (database, backtest, orders, trades). Includes `supabase_client.go` |
| `pkg/supabasetypes/` | Auto-generated Go types for Supabase tables (regenerated via `pnpm sb go-types`) |
| `pkg/instanceid/` | Deterministic strategy instance ID computation (shared by bbgo strategies and SaaS manager) |
| `pkg/core/` | TradeCollector, order store, KLine driver |
| `pkg/backtest/` | Backtesting engine |
| `pkg/server/` | HTTP API and web dashboard server |
| `pkg/pb/` | gRPC protobuf definitions + query extensions for market data service |

### Key Interfaces

- `types.Exchange` (`pkg/types/exchange.go`) — unified exchange interface. Composes `ExchangeMinimal`, `ExchangeMarketDataService`, `ExchangeAccountService`, `ExchangeTradeService`. Mocks generated via mockgen in `pkg/types/mocks/`.
- `bbgo.SingleExchangeStrategy` / `bbgo.CrossExchangeStrategy` — strategy contracts in `pkg/bbgo/trader.go`
- `bbgo.OrderExecutor` — `SubmitOrders` + `CancelOrders`
- Strategies are registered via `bbgo.RegisterStrategy()` and configured through YAML

### Strategy Interface Hierarchy

Strategies can implement optional interfaces for lifecycle hooks (all defined in `pkg/bbgo/trader.go`):

- `ExchangeSessionSubscriber` — `Subscribe(session)` called before WebSocket connection, used to register channel subscriptions (klines, orderbook, etc.)
- `StrategyInitializer` — `Initialize()` called before Subscribe
- `StrategyDefaulter` — `Defaults()` sets default config values
- `StrategyValidator` — `Validate()` validates config
- `StrategyShutdown` — `Shutdown(ctx, wg)` for graceful cleanup

### Dynamic Injection

BBGO auto-injects fields into strategy structs by type before `Run()`:
- `*bbgo.ExchangeSession` — the exchange session the strategy runs on
- `bbgo.OrderExecutor` — for submitting/canceling orders
- `types.Market` — injected if the strategy has a `Symbol string` field

### Configuration

YAML config files (e.g., `bbgo.yaml`) define exchange sessions and strategy parameters. API credentials come from `.env.local` environment files. Config parsing is in `pkg/bbgo/config.go`.

### Database

Supports MySQL, SQLite, and Supabase via rockhopper migrations. Migration SQL files are in `migrations/` and compiled Go packages in `pkg/migrations/`.

#### Supabase Direct Write (`DB_DRIVER=supabase`)

When `DB_DRIVER=supabase` is set (along with `SUPABASE_URL`, `SUPABASE_SERVICE_KEY`, `DB_USER_ID`), bbgo writes orders, trades, positions, and profits directly to Supabase via REST API instead of SQLite. This is the mode used in the SaaS deployment.

- **Types**: `pkg/supabasetypes/database_types.go` — auto-generated from Supabase schema
- **Service**: `pkg/service/supabase_client.go` — InsertOrder/InsertTrade/InsertPosition/InsertProfit + query methods
- **Tables**: `orders`, `trades`, `positions`, `profits` (match bbgo's original SQLite table design)
- **Paper tables**: When `SUPABASE_TABLE_PREFIX=paper_` is set, writes go to `paper_orders`, `paper_trades`, etc.
- **Service parity**: Full SQL-mode alignment includes `nav_history_details`, `rewards`, `withdraws`, `deposits`, `margin_loans`, `margin_repays`, `margin_interests`, `margin_liquidations`, `futures_position_risks`
- Orders and trades include `strategy_instance_id` for per-instance data isolation
- Orders use `order_type` column (not `type`) to match bbgo's original schema
- Multi-tenant via `user_id` column on all tables

**Type generation**: All Supabase Go types and TypeScript types are regenerated from the live database using `pnpm sb` from the `saas/web/` directory:
```bash
cd saas/web
pnpm sb push          # push migrations to remote database
pnpm sb go-types      # regenerate → manager/supabase_types.go + pkg/supabasetypes/database_types.go
pnpm sb types         # regenerate → web/src/lib/supabase/types.ts
```

### Paper Trade Engine

The paper trade engine (`pkg/bbgo/paper_trade_exchange.go` + `paper_trade_futures.go`) wraps a real exchange for market data and simulates order fills locally via kline-driven matching. It implements the full `types.Exchange` interface plus futures and margin extensions.

**Spot mode** (`PAPER_TRADE=true`): Virtual balances, kline-driven limit/market order matching, balance locking/unlocking, open order restoration from DB.

**Futures mode** (`session.Futures=true`):
- `PaperTradeExchange` implements `FuturesExchange` and `ExchangeRiskService` interfaces
- Short selling allowed without holding base currency; margin locked instead of full notional (`notional / leverage`)
- Position tracking: `paperFuturesState` per symbol — weighted average entry price, position amount (positive=long, negative=short), flip detection (long→short, short→long)
- `QueryPositionRisk()` computes unrealized PnL, liquidation price, initial/maintenance margin, notional value
- Position risk persistence: trade-event driven via `environment.go` `OnTradeUpdate` callback (only registered when config `userDataStream.futuresPosition: true`, which SaaS manager auto-sets when any session has `Futures=true`). Each futures trade triggers `FuturesService.QueryPositionsAndInsert` → `paper_futures_position_risks`. `PositionRiskUpdateInterval` throttles repeated writes for the same symbol. No background ticker.
- Liquidation price formula: Long = entry × (1 - 1/leverage + maintRate), Short = entry × (1 + 1/leverage - maintRate)
- Maintenance margin rate: 0.5% (`defaultMaintMarginRate`)

**Margin mode** (`session.Margin=true`):
- `PaperTradeExchange` implements `MarginExchange` and `MarginBorrowRepayService` interfaces
- `BorrowMarginAsset()` adds asset to account balance and tracks borrowed amount
- `RepayMarginAsset()` deducts from account; clears interest when fully repaid
- Hourly interest accrual at 0.01% rate (`defaultHourlyMarginRate`) via background goroutine
- `QueryMarginAssetMaxBorrowable()` returns 5× available balance

**Concurrency**: All futures/margin state protected by `PaperTradeExchange.mu`. Lock ordering in matching engine: `paperMatchingBook.mu` → `PaperTradeExchange.mu` (no reverse path). Fill callbacks emitted outside locks to prevent deadlock.

**Session wiring** (`session.go:InitExchange`): When `PAPER_TRADE=true` and session has `Futures` or `Margin` enabled, the corresponding `UseFutures()`/`UseMargin()` methods are called on the paper exchange before it replaces the real exchange. `StartBackgroundServices(ctx)` starts position risk sync and interest accrual goroutines.

**Key files**:
- `pkg/bbgo/paper_trade_exchange.go` — matching engine, balance management, order submission
- `pkg/bbgo/paper_trade_futures.go` — futures position tracking, margin borrow/repay, risk computation, DB sync
- `pkg/bbgo/paper_trade_futures_test.go` — 32 tests covering position lifecycle, liquidation, margin, interest accrual

## Strategy Development

1. Create package under `pkg/strategy/<name>/`
2. Implement `SingleExchangeStrategy` or `CrossExchangeStrategy` interface
3. Optionally implement `ExchangeSessionSubscriber` for market data subscriptions
4. Register with `bbgo.RegisterStrategy("<name>", &Strategy{})` in an `init()` function
5. Add config tests similar to `pkg/bbgo/config_test.go`
6. Use `testify` for assertions (already in go.mod)

### Strategy `Defaults()` and `Validate()` (required reading)

A strategy that starts successfully but computes wrong values is **worse than one that crashes** — silent semantic failures can run for days placing wrong orders. Two discipline rules prevent this class of bug:

**1. Fill defaults at the field level, not the pointer level.** User config often supplies a nested struct partially. Pattern:

```go
// WRONG — only handles "user omitted entire nested object"
if s.Thing == nil { s.Thing = &DefaultThing }

// RIGHT — also handles "user supplied Thing but left RequiredField zero"
if s.Thing == nil { s.Thing = &Thing{} }
if s.Thing.RequiredField == 0 { s.Thing.RequiredField = defaultRequiredField }
if s.Thing.Interval == "" { s.Thing.Interval = s.Interval }
```

This matters for every strategy with nested config (bollmaker, rsmaker, pivotshort, etc.). See `pkg/strategy/bollmaker/strategy.go` `Defaults()` for the reference pattern and `pkg/strategy/bollmaker/defaults_test.go` for the regression test.

**2. `Validate()` checks semantics, not just presence.** A struct field that exists but holds a zero value where a positive value is required (e.g. `BandWidth: 0` for BOLL where the formula is `sma ± k×stdDev`) will silently collapse the indicator. Refuse to start:

```go
if s.DefaultBollinger.BandWidth <= 0 {
    return fmt.Errorf("defaultBollinger.bandWidth must be > 0, got %v", s.DefaultBollinger.BandWidth)
}
```

When adding a new strategy: write `Defaults()` defensively (rule 1), write `Validate()` strictly (rule 2), and write a meta-test that calls `Defaults()` on an empty `Strategy{}` and asserts no semantically-required field remains zero.

### SaaS-Exposed Strategies (50 of 55+ registered)

The SaaS frontend (`saas/web/src/lib/bbgo/strategies.ts`) exposes 50 strategies across 10 categories. Strategy metadata is centralized in the `strategy_registry` Supabase table (defaults, fields, liveOnly, requiresFutures). A code generation script (`saas/web/scripts/gen_strategy_types.mjs`) produces `saas/manager/strategy_types.go` from the frontend definitions. Strategies not in the SaaS frontend (etf, liquditycorr, marketcap, tradingdesk, tri, example/*) are intentionally excluded. 22 strategies are `liveOnly` (blocked from paper/simulation mode). 10 are cross-exchange strategies requiring multiple exchange sessions.

When adding a new strategy to the SaaS:
1. Register in bbgo via `bbgo.RegisterStrategy(ID, &Strategy{})`
2. Add `StrategySchema` entry to `STRATEGY_SCHEMAS` in `saas/web/src/lib/bbgo/strategies.ts`
3. Regenerate Go types: `cd saas/web && pnpm gen-strategy-types`
4. Insert into `strategy_registry` Supabase table with defaults and field definitions
5. If live-only, set `live_only = true` in `strategy_registry`
6. If cross-exchange, define `sessionRoles` in the schema entry
7. If renaming an old ID, add alias to `legacyStrategyAliases` in `saas/manager/user.go`

## Build Tag Constraints

Some code and tests use `//go:build dnum` or `//go:build !dnum` to conditionally compile between standard `float64` and high-precision `dnum` decimal math. When adding dnum-specific code paths, mirror the build constraints in tests.

## SaaS Deployment

The `saas/` directory contains the multi-tenant deployment layer. See `saas/CLAUDE.md` for full architecture, build commands, and API documentation. Key components:

| Component | Location | Purpose |
|-----------|----------|---------|
| Manager | `saas/manager/` | Go HTTP server for per-instance container orchestration, data sync, backtests |
| Web | `saas/web/` | Next.js 16 dashboard frontend |
| Docker | `saas/docker/` | Dockerfiles and docker-compose for backend services |
| Migrations | `saas/web/supabase/migrations/` | Supabase schema migrations (23 migrations covering live/paper tables, strategy registry, realtime) |
