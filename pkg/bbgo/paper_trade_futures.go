package bbgo

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	log "github.com/sirupsen/logrus"
)

const (
	defaultMaintMarginRate  = "0.005"
	defaultHourlyMarginRate = "0.0001"
	defaultFundingRate = "0.0001" // 0.01% per 8h — typical Binance perpetual rate
)

// PositionModeOneWay is the one-way futures position mode (Binance positionSide=BOTH).
// The paper trade engine currently only supports one-way mode; hedge mode is left as
// future work. In one-way mode there is exactly one position slot per symbol and the
// position_amount sign indicates direction (positive=long, negative=short).
const PositionModeOneWay = "BOTH"

// paperFuturesState tracks simulated futures state per symbol.
type paperFuturesState struct {
	Leverage           int
	PositionAmount     fixedpoint.Value // positive = long, negative = short
	EntryPrice         fixedpoint.Value
	PositionSide       types.PositionType // always "BOTH" in one-way mode
	MarginAsset        string
	IsolatedSymbol     string
	StrategyInstanceID string
	LastFundingTime    time.Time
}

// paperMarginState tracks simulated margin borrow/lend state per asset.
type paperMarginState struct {
	Borrowed     fixedpoint.Value
	Interest     fixedpoint.Value
	InterestRate fixedpoint.Value // hourly rate
	LastAccrual  time.Time
}

// --- FuturesExchange interface ---

func (e *PaperTradeExchange) UseFutures() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.futuresSettings.IsFutures = true
}

func (e *PaperTradeExchange) UseIsolatedFutures(symbol string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.futuresSettings.IsFutures = true
	e.futuresSettings.IsIsolatedFutures = true
	e.futuresSettings.IsolatedFuturesSymbol = symbol
}

func (e *PaperTradeExchange) GetFuturesSettings() types.FuturesSettings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.futuresSettings
}

// --- MarginExchange interface ---

func (e *PaperTradeExchange) UseMargin() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.marginSettings.IsMargin = true
}

func (e *PaperTradeExchange) UseIsolatedMargin(symbol string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.marginSettings.IsMargin = true
	e.marginSettings.IsIsolatedMargin = true
	e.marginSettings.IsolatedMarginSymbol = symbol
}

func (e *PaperTradeExchange) GetMarginSettings() types.MarginSettings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.marginSettings
}

// --- ExchangeRiskService interface (futures leverage + position risk) ---

func (e *PaperTradeExchange) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state := e.getOrCreateFuturesState(symbol)
	state.Leverage = leverage
	return nil
}

// QueryPositionRisk returns position risks for the given symbols.
// Closed positions (amount=0) are included so that bbgo's FuturesService
// can update the DB row to reflect the closed state.
func (e *PaperTradeExchange) QueryPositionRisk(ctx context.Context, symbol ...string) ([]types.PositionRisk, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(symbol) == 0 {
		var risks []types.PositionRisk
		for sym := range e.futuresStates {
			risk := e.computePositionRiskLocked(sym)
			risks = append(risks, risk)
		}
		return risks, nil
	}

	var risks []types.PositionRisk
	for _, sym := range symbol {
		if _, ok := e.futuresStates[sym]; !ok {
			continue
		}
		risk := e.computePositionRiskLocked(sym)
		risks = append(risks, risk)
	}
	return risks, nil
}

// --- MarginBorrowRepayService interface ---

func (e *PaperTradeExchange) BorrowMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	e.mu.Lock()

	state := e.getOrCreateMarginState(asset)
	state.Borrowed = state.Borrowed.Add(amount)
	if state.LastAccrual.IsZero() {
		state.LastAccrual = time.Now()
	}

	e.account.AddBalance(asset, amount)
	e.EmitBalanceUpdateFromAccount()

	var loanEvt *types.MarginLoan
	if e.OnMarginLoan != nil {
		loanEvt = &types.MarginLoan{
			TransactionID: nextPaperOrderID(),
			Exchange:      e.inner.Name(),
			Asset:         asset,
			Principle:     amount,
			Time:          types.Time(time.Now()),
		}
	}
	e.mu.Unlock()

	if loanEvt != nil {
		e.OnMarginLoan(*loanEvt)
	}
	return nil
}

func (e *PaperTradeExchange) RepayMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	e.mu.Lock()

	state, ok := e.marginStates[asset]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("paper trade: no margin debt for asset %s", asset)
	}
	if state.Borrowed.Compare(amount) < 0 {
		e.mu.Unlock()
		return fmt.Errorf("paper trade: repay amount %s exceeds borrowed %s for asset %s",
			amount.String(), state.Borrowed.String(), asset)
	}

	now := time.Now()
	// Settle partial interest accrued since the last hourly tick. Without
	// this, a user who repays mid-hour gets the partial period interest-free.
	interestEvt := e.accrueMarginInterestLocked(asset, state, now)

	state.Borrowed = state.Borrowed.Sub(amount)
	if state.Borrowed.IsZero() {
		state.Interest = fixedpoint.Zero
		state.LastAccrual = time.Time{}
	} else {
		state.LastAccrual = now
	}

	e.account.AddBalance(asset, amount.Neg())
	e.EmitBalanceUpdateFromAccount()

	var repayEvt *types.MarginRepay
	if e.OnMarginRepay != nil {
		repayEvt = &types.MarginRepay{
			TransactionID: nextPaperOrderID(),
			Exchange:      e.inner.Name(),
			Asset:         asset,
			Principle:     amount,
			Time:          types.Time(now),
		}
	}
	e.mu.Unlock()

	if interestEvt != nil && e.OnMarginInterest != nil {
		e.OnMarginInterest(*interestEvt)
	}
	if repayEvt != nil {
		e.OnMarginRepay(*repayEvt)
	}
	return nil
}

func (e *PaperTradeExchange) QueryMarginAssetMaxBorrowable(ctx context.Context, asset string) (fixedpoint.Value, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	balances := e.account.Balances()
	if bal, ok := balances[asset]; ok {
		return bal.Available.Mul(fixedpoint.NewFromInt(5)), nil
	}
	return fixedpoint.Zero, nil
}

// --- Internal helpers ---

func (e *PaperTradeExchange) getOrCreateFuturesState(symbol string) *paperFuturesState {
	if e.futuresStates == nil {
		e.futuresStates = make(map[string]*paperFuturesState)
	}
	state, ok := e.futuresStates[symbol]
	if !ok {
		state = &paperFuturesState{
			Leverage:     20,
			MarginAsset:  "USDT",
			PositionSide: types.PositionType(PositionModeOneWay),
			// Treat current slot as already settled so a position opened
			// after a funding boundary doesn't get charged for that slot
			// on the next timer tick. Real exchanges only charge funding
			// at boundaries that occur while the position is open.
			LastFundingTime: lastFundingSlotUTC(time.Now()),
		}
		e.futuresStates[symbol] = state
	}
	return state
}

func (e *PaperTradeExchange) getOrCreateMarginState(asset string) *paperMarginState {
	if e.marginStates == nil {
		e.marginStates = make(map[string]*paperMarginState)
	}
	state, ok := e.marginStates[asset]
	if !ok {
		state = &paperMarginState{
			InterestRate: fixedpoint.MustNewFromString(defaultHourlyMarginRate),
		}
		e.marginStates[asset] = state
	}
	return state
}

// maintMarginTier represents a tier in the tiered maintenance margin schedule.
type maintMarginTier struct {
	NotionalCap fixedpoint.Value // upper bound of this tier (0 = unlimited)
	Rate        fixedpoint.Value // maintenance margin rate for this tier
}

// defaultMaintenanceTiers approximates Binance perpetual maintenance margin tiers.
// These are based on BTCUSDT tiers; other symbols may have different schedules.
// For paper trading simulation this is an acceptable approximation.
var defaultMaintenanceTiers = []maintMarginTier{
	{NotionalCap: fixedpoint.MustNewFromString("50000"), Rate: fixedpoint.MustNewFromString("0.004")},
	{NotionalCap: fixedpoint.MustNewFromString("250000"), Rate: fixedpoint.MustNewFromString("0.005")},
	{NotionalCap: fixedpoint.MustNewFromString("1000000"), Rate: fixedpoint.MustNewFromString("0.01")},
	{NotionalCap: fixedpoint.MustNewFromString("5000000"), Rate: fixedpoint.MustNewFromString("0.025")},
	{NotionalCap: fixedpoint.MustNewFromString("10000000"), Rate: fixedpoint.MustNewFromString("0.05")},
	{NotionalCap: fixedpoint.Zero, Rate: fixedpoint.MustNewFromString("0.1")}, // unlimited
}

// getMaintenanceMarginRate returns the effective maintenance margin rate based on notional value.
func getMaintenanceMarginRate(notional fixedpoint.Value) fixedpoint.Value {
	for _, tier := range defaultMaintenanceTiers {
		if tier.NotionalCap.IsZero() || notional.Compare(tier.NotionalCap) <= 0 {
			return tier.Rate
		}
	}
	return defaultMaintenanceTiers[len(defaultMaintenanceTiers)-1].Rate
}

// computePositionRiskLocked calculates simulated position risk from current state.
// Returns a risk with position_amount=0 for closed positions, so bbgo's FuturesService
// can persist the closed state to the database. position_side is always "BOTH" in one-way
// mode, so the closed snapshot lands on the same DB row as the open snapshot and updates
// it in place via the (exchange, symbol, position_side) unique key.
// Must be called with e.mu held.
func (e *PaperTradeExchange) computePositionRiskLocked(symbol string) types.PositionRisk {
	state, ok := e.futuresStates[symbol]
	if !ok {
		return types.PositionRisk{
			Symbol: symbol,
		}
	}

	// Closed position: return minimal risk so FuturesService updates DB to amount=0.
	// position_side stays "BOTH" so the close snapshot hits the same DB row.
	if state.PositionAmount.IsZero() {
		return types.PositionRisk{
			Exchange:           e.inner.Name(),
			Symbol:             symbol,
			PositionSide:       state.PositionSide,
			EntryPrice:         state.EntryPrice,
			Leverage:           fixedpoint.NewFromInt(int64(state.Leverage)),
			MarginAsset:        state.MarginAsset,
			PositionAmount:     fixedpoint.Zero,
			UpdateTime:         types.MillisecondTimestamp(time.Now()),
			StrategyInstanceID: state.StrategyInstanceID,
		}
	}

	var markPrice fixedpoint.Value
	if book, ok := e.matchingBooks[symbol]; ok {
		markPrice = book.lastPrice
	}
	if markPrice.IsZero() {
		markPrice = state.EntryPrice
	}

	notional := state.PositionAmount.Abs().Mul(markPrice)
	leverage := fixedpoint.NewFromInt(int64(state.Leverage))
	initialMargin := notional.Div(leverage)

	maintRate := getMaintenanceMarginRate(notional)
	maintMargin := notional.Mul(maintRate)

	var unrealizedPnL fixedpoint.Value
	if state.PositionAmount.Sign() > 0 {
		unrealizedPnL = markPrice.Sub(state.EntryPrice).Mul(state.PositionAmount)
	} else {
		unrealizedPnL = state.EntryPrice.Sub(markPrice).Mul(state.PositionAmount.Abs())
	}

	var liquidationPrice fixedpoint.Value
	if state.PositionAmount.Sign() > 0 {
		liquidationPrice = state.EntryPrice.Mul(
			fixedpoint.One.Sub(fixedpoint.One.Div(leverage)).Add(maintRate),
		)
	} else {
		liquidationPrice = state.EntryPrice.Mul(
			fixedpoint.One.Add(fixedpoint.One.Div(leverage)).Sub(maintRate),
		)
	}

	return types.PositionRisk{
		Exchange:               e.inner.Name(),
		Symbol:                 symbol,
		PositionSide:           state.PositionSide,
		EntryPrice:             state.EntryPrice,
		Leverage:               leverage,
		LiquidationPrice:       liquidationPrice,
		MarkPrice:              markPrice,
		UnrealizedPnL:          unrealizedPnL,
		Notional:               notional,
		InitialMargin:          initialMargin,
		MaintMargin:            maintMargin,
		PositionInitialMargin:  initialMargin,
		OpenOrderInitialMargin: fixedpoint.Zero,
		MarginAsset:            state.MarginAsset,
		PositionAmount:         state.PositionAmount,
		UpdateTime:             types.MillisecondTimestamp(time.Now()),
		StrategyInstanceID:     state.StrategyInstanceID,
	}
}

// computeRealizedPnLLocked returns the realized PnL for a fill that reduces
// an existing position. Opening/adding trades return zero.
// Long reduced by SELL: (fillPrice - entryPrice) * min(posQty, sellQty)
// Short reduced by BUY: (entryPrice - fillPrice) * min(|posQty|, buyQty)
// Must be called with e.mu held.
func (e *PaperTradeExchange) computeRealizedPnLLocked(symbol string, side types.SideType, fillPrice, quantity fixedpoint.Value) float64 {
	state, ok := e.futuresStates[symbol]
	if !ok || state.PositionAmount.IsZero() {
		return 0.0
	}

	switch {
	case state.PositionAmount.Sign() > 0 && side == types.SideTypeSell:
		reducingQty := fixedpoint.Min(state.PositionAmount, quantity)
		return fillPrice.Sub(state.EntryPrice).Mul(reducingQty).Float64()
	case state.PositionAmount.Sign() < 0 && side == types.SideTypeBuy:
		reducingQty := fixedpoint.Min(state.PositionAmount.Abs(), quantity)
		return state.EntryPrice.Sub(fillPrice).Mul(reducingQty).Float64()
	default:
		return 0.0
	}
}

// updateFuturesPositionLocked updates the futures state after a fill.
// Must be called with e.mu held (caller acquires it around this call).
func (e *PaperTradeExchange) updateFuturesPositionLocked(symbol string, side types.SideType, price, quantity fixedpoint.Value, strategyInstanceID string) {
	if !e.futuresSettings.IsFutures {
		return
	}

	state := e.getOrCreateFuturesState(symbol)

	switch side {
	case types.SideTypeBuy:
		if state.PositionAmount.Sign() < 0 {
			if state.PositionAmount.Abs().Compare(quantity) >= 0 {
				state.PositionAmount = state.PositionAmount.Add(quantity)
			} else {
				remainingQty := quantity.Add(state.PositionAmount)
				state.EntryPrice = price
				state.PositionAmount = remainingQty
			}
		} else {
			if state.PositionAmount.IsZero() {
				state.EntryPrice = price
				state.PositionAmount = quantity
			} else {
				totalCost := state.EntryPrice.Mul(state.PositionAmount).Add(price.Mul(quantity))
				newAmount := state.PositionAmount.Add(quantity)
				state.EntryPrice = totalCost.Div(newAmount)
				state.PositionAmount = newAmount
			}
		}

	case types.SideTypeSell:
		if state.PositionAmount.Sign() > 0 {
			if state.PositionAmount.Compare(quantity) >= 0 {
				state.PositionAmount = state.PositionAmount.Sub(quantity)
			} else {
				remainingQty := quantity.Sub(state.PositionAmount)
				state.EntryPrice = price
				state.PositionAmount = remainingQty.Neg()
			}
		} else {
			if state.PositionAmount.IsZero() {
				state.EntryPrice = price
				state.PositionAmount = quantity.Neg()
			} else {
				totalCost := state.EntryPrice.Mul(state.PositionAmount.Abs()).Add(price.Mul(quantity))
				newAmount := state.PositionAmount.Abs().Add(quantity)
				state.EntryPrice = totalCost.Div(newAmount)
				state.PositionAmount = state.PositionAmount.Sub(quantity)
			}
		}
	}

	if state.MarginAsset == "" {
		if market, ok := e.markets[symbol]; ok {
			state.MarginAsset = market.QuoteCurrency
		} else {
			state.MarginAsset = "USDT"
		}
	}

	// One-way mode: position_side is always "BOTH". Direction is encoded in the sign
	// of PositionAmount (positive=long, negative=short). Keeping side constant across
	// open/flip/close means each (symbol, strategy_instance_id) lands on a single DB
	// row and close/flip snapshots correctly overwrite the open snapshot.
	state.PositionSide = types.PositionType(PositionModeOneWay)

	if strategyInstanceID != "" {
		state.StrategyInstanceID = strategyInstanceID
	}

}

// SyncStrategyPositionFromFuturesState copies the authoritative futures position
// (PositionAmount, EntryPrice) from paperFuturesState into the given strategy
// position. Returns true if a sync happened.
//
// This bridges the gap between the futures position tracker
// (paperFuturesState, restored from paper_futures_position_risks on restart)
// and the strategy's own *types.Position (loaded from JSON persistence,
// which can be stale or zero after a container restart). Without this
// sync, a SPOT strategy running on a FUTURES paper session starts each
// session with Base=0/AverageCost=0, causing AddTrade's spot semantics
// to compute wrong profits and accumulate wrong position state — even
// though paperFuturesState itself is correct.
//
// Once Base/AverageCost are aligned, spot AddTrade produces results that
// match updateFuturesPositionLocked: weighted-average entry on adds,
// realized profit on reduces, and correct flip handling.
func (e *PaperTradeExchange) SyncStrategyPositionFromFuturesState(symbol string, pos *types.Position) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, ok := e.futuresStates[symbol]
	if !ok || state.PositionAmount.IsZero() {
		return false
	}

	pos.Lock()
	defer pos.Unlock()
	pos.Base = state.PositionAmount
	pos.AverageCost = state.EntryPrice
	return true
}

// accrueMarginInterestLocked applies interest accrued on state.Borrowed between
// state.LastAccrual and `until`. Mutates state.Interest and the account balance.
// Returns a MarginInterest event if any interest was accrued, nil otherwise.
// Caller must hold e.mu and is responsible for updating state.LastAccrual.
func (e *PaperTradeExchange) accrueMarginInterestLocked(asset string, state *paperMarginState, until time.Time) *types.MarginInterest {
	if state.LastAccrual.IsZero() || !until.After(state.LastAccrual) {
		return nil
	}
	hours := until.Sub(state.LastAccrual).Hours()
	if hours <= 0 {
		return nil
	}
	accruedInterest := state.Borrowed.Mul(state.InterestRate).Mul(fixedpoint.NewFromFloat(hours))
	if accruedInterest.Sign() <= 0 {
		return nil
	}
	state.Interest = state.Interest.Add(accruedInterest)
	e.account.AddBalance(asset, accruedInterest.Neg())
	return &types.MarginInterest{
		Exchange:     e.inner.Name(),
		Asset:        asset,
		Principle:    state.Borrowed,
		Interest:     accruedInterest,
		InterestRate: state.InterestRate,
		Time:         types.Time(until),
	}
}

// updateMarginInterest accrues simulated interest on borrowed assets.
func (e *PaperTradeExchange) updateMarginInterest() {
	var events []types.MarginInterest
	var balanceChanged bool
	func() {
		e.mu.Lock()
		defer e.mu.Unlock()

		now := time.Now()
		for asset, state := range e.marginStates {
			if state.Borrowed.IsZero() || state.LastAccrual.IsZero() {
				continue
			}

			// updateMarginInterest only charges on full-hour boundaries
			// (vs. RepayMarginAsset which settles partial hours at repay time).
			if now.Sub(state.LastAccrual).Hours() < 1.0 {
				continue
			}

			evt := e.accrueMarginInterestLocked(asset, state, now)
			if evt == nil {
				continue
			}
			state.LastAccrual = now
			balanceChanged = true

			if e.OnMarginInterest != nil {
				events = append(events, *evt)
			}
		}
	}()

	if balanceChanged {
		e.EmitBalanceUpdateFromAccount()
	}

	if e.OnMarginInterest != nil {
		for _, evt := range events {
			e.OnMarginInterest(evt)
		}
	}
}

// StartMarginInterestTimer starts a goroutine that periodically accrues margin interest.
func (e *PaperTradeExchange) StartMarginInterestTimer(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.updateMarginInterest()
			}
		}
	}()
}

// StartBackgroundServices starts background tasks for margin interest accrual,
// futures funding rate simulation, and periodic NAV snapshot recording.
func (e *PaperTradeExchange) StartBackgroundServices(ctx context.Context) {
	if e.marginSettings.IsMargin {
		e.StartMarginInterestTimer(ctx)
	}
	if e.futuresSettings.IsFutures {
		e.StartFundingRateTimer(ctx)
	}
	if e.OnPeriodicNAVRecord != nil {
		go e.runNAVTicker(ctx)
	}
}

// runNAVTicker fires OnPeriodicNAVRecord every minute so the environment
// can persist a NAV snapshot for equity curve reconstruction.
func (e *PaperTradeExchange) runNAVTicker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			e.OnPeriodicNAVRecord(t)
		}
	}
}

// fundingScheduleHours are the UTC hours at which perpetual exchanges settle funding.
var fundingScheduleHours = []int{0, 8, 16}

// lastFundingSlotUTC returns the most recent funding settlement time at or before t.
// Real perpetual exchanges (Binance) settle funding at UTC 00:00, 08:00, 16:00.
func lastFundingSlotUTC(t time.Time) time.Time {
	utc := t.UTC()
	h := utc.Hour()
	slot := (h / 8) * 8
	return time.Date(utc.Year(), utc.Month(), utc.Day(), slot, 0, 0, 0, time.UTC)
}

// applyFundingRate applies funding rate payments to all open futures positions.
// Positive rate: longs pay shorts. Negative rate: shorts pay longs.
// Funding = position_amount × mark_price × funding_rate
// Settled on UTC 00/08/16 boundaries to match real exchange behavior.
//
// Catch-up: if a position's LastFundingTime is several slots behind the
// current slot (e.g. container was down for 21h, missing 2-3 settlements),
// we settle ONE missed slot per call. The 1-hourly timer drives subsequent
// slots until caught up. Each missed slot is paid at the current funding
// rate — historical rates are not fetched, so a long downtime produces
// approximate (not exact) backfill.
func (e *PaperTradeExchange) applyFundingRate() {
	var events []types.FundingPayment
	var balanceChanged bool
	func() {
		e.mu.Lock()
		defer e.mu.Unlock()

		now := time.Now()
		currentSlot := lastFundingSlotUTC(now)

		// Refresh the per-slot funding rate cache once. Real Binance rates
		// vary slot-to-slot (typically -0.05%..+0.10% for BTC perp); using
		// a hardcoded 0.01% silently misprices PnL on every position.
		if !currentSlot.Equal(e.fundingRateSlot) {
			e.fundingRateCache = e.fetchFundingRates(currentSlot)
			e.fundingRateSlot = currentSlot
		}

		for symbol, state := range e.futuresStates {
			if state.PositionAmount.IsZero() {
				continue
			}

			// Pick the slot to settle: the next 8h slot after LastFundingTime,
			// capped at the current slot. When LastFundingTime is zero (fresh
			// position never funded), settle the current slot.
			var slot time.Time
			if state.LastFundingTime.IsZero() {
				slot = currentSlot
			} else {
				slot = state.LastFundingTime.Add(8 * time.Hour)
				if slot.After(currentSlot) {
					continue // up to date
				}
			}

			rate, ok := e.fundingRateCache[symbol]
			if !ok {
				rate = fixedpoint.MustNewFromString(defaultFundingRate)
			}

			var markPrice fixedpoint.Value
			if book, ok := e.matchingBooks[symbol]; ok {
				markPrice = book.lastPrice
			}
			if markPrice.IsZero() {
				markPrice = state.EntryPrice
			}

			notional := state.PositionAmount.Abs().Mul(markPrice)
			fundingAmount := notional.Mul(rate)

			asset := state.MarginAsset
			if asset == "" {
				asset = "USDT"
			}

			// Longs pay (negative), shorts receive (positive) when rate > 0
			var signedFunding fixedpoint.Value
			if state.PositionAmount.Sign() > 0 {
				signedFunding = fundingAmount.Neg()
				e.account.AddBalance(asset, signedFunding)
			} else {
				signedFunding = fundingAmount
				e.account.AddBalance(asset, signedFunding)
			}
			balanceChanged = true

			missed := ""
			if !state.LastFundingTime.IsZero() && slot.Sub(state.LastFundingTime) > 8*time.Hour {
				remaining := currentSlot.Sub(slot) / (8 * time.Hour)
				missed = fmt.Sprintf(" (catch-up slot %s, %d slot(s) remaining)", slot.Format(time.RFC3339), remaining)
			}
			state.LastFundingTime = slot

			if e.OnFundingPayment != nil {
				events = append(events, types.FundingPayment{
					Exchange: e.inner.Name(),
					Symbol:   symbol,
					Asset:    asset,
					Amount:   signedFunding,
					Rate:     rate,
					Time:     types.Time(slot),
				})
			}
			log.Infof("paper trade: funding rate applied for %s — notional=%s rate=%s funding=%s %s (position_side=%s)%s",
				symbol, notional.String(), rate.String(), fundingAmount.String(), asset, state.PositionSide, missed)
		}
	}()

	if balanceChanged {
		e.EmitBalanceUpdateFromAccount()
	}

	if e.OnFundingPayment != nil {
		for _, evt := range events {
			e.OnFundingPayment(evt)
		}
	}
}

// fetchFundingRates pulls the live funding rate for every symbol with an
// open position. Returns an empty map (caller falls back to
// defaultFundingRate) when the wrapped exchange doesn't implement
// QueryPremiumIndex (only Binance does today) or any fetch fails. Errors
// are logged at debug level — funding settlement is best-effort, not a
// hard dependency.
func (e *PaperTradeExchange) fetchFundingRates(slot time.Time) map[string]fixedpoint.Value {
	fetcher, ok := e.inner.(premiumIndexFetcher)
	if !ok {
		return nil
	}
	out := make(map[string]fixedpoint.Value)
	for symbol, state := range e.futuresStates {
		if state.PositionAmount.IsZero() {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		idx, err := fetcher.QueryPremiumIndex(ctx, symbol)
		cancel()
		if err != nil || idx == nil {
			log.WithError(err).Debugf("paper trade: live funding rate fetch failed for %s, falling back to default", symbol)
			continue
		}
		out[symbol] = idx.LastFundingRate
		log.Infof("paper trade: fetched live funding rate for %s slot %s — rate=%s", symbol, slot.Format(time.RFC3339), idx.LastFundingRate.String())
	}
	return out
}

// StartFundingRateTimer starts a goroutine that applies funding rate every 8 hours.
func (e *PaperTradeExchange) StartFundingRateTimer(ctx context.Context) {
	// Check every hour to see if 8h has elapsed since last funding
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.applyFundingRate()
			}
		}
	}()
}

// MarginBorrowed returns the current borrowed amount for a given asset.
func (e *PaperTradeExchange) MarginBorrowed(asset string) fixedpoint.Value {
	e.mu.Lock()
	defer e.mu.Unlock()
	if state, ok := e.marginStates[asset]; ok {
		return state.Borrowed
	}
	return fixedpoint.Zero
}

// SupportsShortSell returns true if the exchange supports short selling.
func (e *PaperTradeExchange) SupportsShortSell() bool {
	return e.futuresSettings.IsFutures || e.marginSettings.IsMargin
}
