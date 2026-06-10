package bbgo

import (
	"context"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	defaultMaintMarginRate  = "0.005"
	defaultHourlyMarginRate = "0.0001"
)

// paperFuturesState tracks simulated futures state per symbol.
type paperFuturesState struct {
	Leverage       int
	PositionAmount fixedpoint.Value // positive = long, negative = short
	EntryPrice     fixedpoint.Value
	PositionSide   types.PositionType
	MarginAsset    string
	IsolatedSymbol string
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
	defer e.mu.Unlock()

	state := e.getOrCreateMarginState(asset)
	state.Borrowed = state.Borrowed.Add(amount)
	if state.LastAccrual.IsZero() {
		state.LastAccrual = time.Now()
	}

	e.account.AddBalance(asset, amount)
	e.EmitBalanceUpdateFromAccount()

	return nil
}

func (e *PaperTradeExchange) RepayMarginAsset(ctx context.Context, asset string, amount fixedpoint.Value) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, ok := e.marginStates[asset]
	if !ok {
		return fmt.Errorf("paper trade: no margin debt for asset %s", asset)
	}
	if state.Borrowed.Compare(amount) < 0 {
		return fmt.Errorf("paper trade: repay amount %s exceeds borrowed %s for asset %s",
			amount.String(), state.Borrowed.String(), asset)
	}

	state.Borrowed = state.Borrowed.Sub(amount)
	if state.Borrowed.IsZero() {
		state.Interest = fixedpoint.Zero
	}

	e.account.AddBalance(asset, amount.Neg())
	e.EmitBalanceUpdateFromAccount()

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
			Leverage:    20,
			MarginAsset: "USDT",
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

// computePositionRiskLocked calculates simulated position risk from current state.
// Returns a risk with position_amount=0 for closed positions, so bbgo's FuturesService
// can persist the closed state to the database.
// Must be called with e.mu held.
func (e *PaperTradeExchange) computePositionRiskLocked(symbol string) types.PositionRisk {
	state, ok := e.futuresStates[symbol]
	if !ok {
		return types.PositionRisk{
			Symbol: symbol,
		}
	}

	// Closed position: return minimal risk so FuturesService updates DB to amount=0
	if state.PositionAmount.IsZero() {
		return types.PositionRisk{
			Exchange:       e.inner.Name(),
			Symbol:         symbol,
			PositionSide:   state.PositionSide,
			EntryPrice:     state.EntryPrice,
			Leverage:       fixedpoint.NewFromInt(int64(state.Leverage)),
			MarginAsset:    state.MarginAsset,
			PositionAmount: fixedpoint.Zero,
			UpdateTime:     types.MillisecondTimestamp(time.Now()),
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

	maintRate := fixedpoint.MustNewFromString(defaultMaintMarginRate)
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

	positionSide := types.PositionLong
	if state.PositionAmount.Sign() < 0 {
		positionSide = types.PositionShort
	}

	return types.PositionRisk{
		Exchange:               e.inner.Name(),
		Symbol:                 symbol,
		PositionSide:           positionSide,
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
	}
}

// updateFuturesPositionLocked updates the futures state after a fill.
// Must be called with e.mu held (caller acquires it around this call).
func (e *PaperTradeExchange) updateFuturesPositionLocked(symbol string, side types.SideType, price, quantity fixedpoint.Value) {
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

	if state.PositionAmount.Sign() > 0 {
		state.PositionSide = types.PositionLong
	} else if state.PositionAmount.Sign() < 0 {
		state.PositionSide = types.PositionShort
	} else {
		state.PositionSide = types.PositionType("")
	}
}

// updateMarginInterest accrues simulated interest on borrowed assets.
func (e *PaperTradeExchange) updateMarginInterest() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for _, state := range e.marginStates {
		if state.Borrowed.IsZero() || state.LastAccrual.IsZero() {
			continue
		}

		hours := now.Sub(state.LastAccrual).Hours()
		if hours < 1.0 {
			continue
		}

		accruedInterest := state.Borrowed.Mul(state.InterestRate).Mul(fixedpoint.NewFromFloat(hours))
		state.Interest = state.Interest.Add(accruedInterest)
		state.LastAccrual = now
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

// StartBackgroundServices starts background tasks for margin interest accrual.
// Futures position risk is handled by bbgo's FuturesService via trade callbacks
// (see environment.go futuresPositionWriterCreator), not by a separate sync loop.
func (e *PaperTradeExchange) StartBackgroundServices(ctx context.Context) {
	if e.marginSettings.IsMargin {
		e.StartMarginInterestTimer(ctx)
	}
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
