package bbgo

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

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

func (e *PaperTradeExchange) QueryPositionRisk(ctx context.Context, symbol ...string) ([]types.PositionRisk, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(symbol) == 0 {
		var risks []types.PositionRisk
		for sym, state := range e.futuresStates {
			if state.PositionAmount.IsZero() {
				continue
			}
			risk := e.computePositionRiskLocked(sym)
			risks = append(risks, risk)
		}
		return risks, nil
	}

	var risks []types.PositionRisk
	for _, sym := range symbol {
		state, ok := e.futuresStates[sym]
		if !ok || state.PositionAmount.IsZero() {
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
// Must be called with e.mu held.
func (e *PaperTradeExchange) computePositionRiskLocked(symbol string) types.PositionRisk {
	state, ok := e.futuresStates[symbol]
	if !ok || state.PositionAmount.IsZero() {
		return types.PositionRisk{}
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

// EmitPositionRiskSnapshots computes and returns all non-zero futures position risks.
func (e *PaperTradeExchange) EmitPositionRiskSnapshots() []types.PositionRisk {
	e.mu.Lock()
	defer e.mu.Unlock()

	var risks []types.PositionRisk
	for symbol, state := range e.futuresStates {
		if state.PositionAmount.IsZero() {
			continue
		}
		risk := e.computePositionRiskLocked(symbol)
		risks = append(risks, risk)
	}
	return risks
}

// SyncPositionRisksToDB writes current position risk snapshots to the database.
func (e *PaperTradeExchange) SyncPositionRisksToDB() error {
	risks := e.EmitPositionRiskSnapshots()
	if e.db == nil || len(risks) == 0 {
		return nil
	}

	tableName := e.tableName("futures_position_risks")
	for _, risk := range risks {
		_, err := e.db.NamedExec(`INSERT INTO "`+tableName+`" (
			exchange, symbol, position_side, entry_price, leverage, liquidation_price,
			mark_price, break_even_price, unrealized_pnl, notional, initial_margin, maint_margin,
			position_initial_margin, open_order_initial_margin, adl, margin_asset,
			position_amount, updated_at, user_id
		) VALUES (
			:exchange, :symbol, :position_side, :entry_price, :leverage, :liquidation_price,
			:mark_price, :break_even_price, :unrealized_pnl, :notional, :initial_margin, :maint_margin,
			:position_initial_margin, :open_order_initial_margin, :adl, :margin_asset,
			:position_amount, :updated_at, :user_id
		) ON CONFLICT (user_id, exchange, symbol, position_side) DO UPDATE SET
			entry_price=:entry_price, leverage=:leverage, liquidation_price=:liquidation_price,
			mark_price=:mark_price, unrealized_pnl=:unrealized_pnl, notional=:notional,
			initial_margin=:initial_margin, maint_margin=:maint_margin,
			position_initial_margin=:position_initial_margin,
			position_amount=:position_amount, updated_at=:updated_at`,
			map[string]interface{}{
				"exchange":                  risk.Exchange,
				"symbol":                    risk.Symbol,
				"position_side":             risk.PositionSide,
				"entry_price":               risk.EntryPrice,
				"leverage":                  risk.Leverage,
				"liquidation_price":         risk.LiquidationPrice,
				"mark_price":                risk.MarkPrice,
				"break_even_price":          risk.BreakEvenPrice,
				"unrealized_pnl":            risk.UnrealizedPnL,
				"notional":                  risk.Notional,
				"initial_margin":            risk.InitialMargin,
				"maint_margin":              risk.MaintMargin,
				"position_initial_margin":   risk.PositionInitialMargin,
				"open_order_initial_margin": risk.OpenOrderInitialMargin,
				"adl":                       risk.Adl,
				"margin_asset":              risk.MarginAsset,
				"position_amount":           risk.PositionAmount,
				"updated_at":                risk.UpdateTime.Time(),
				"user_id":                   e.userID,
			})
		if err != nil {
			return err
		}
	}
	return nil
}

// StartPositionRiskSync starts periodic position risk snapshots to DB.
func (e *PaperTradeExchange) StartPositionRiskSync(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := e.SyncPositionRisksToDB(); err != nil {
					log.WithError(err).Warn("paper trade: failed to sync position risks to DB")
				}
			}
		}
	}()
}

// StartBackgroundServices starts all background sync tasks based on configured mode.
func (e *PaperTradeExchange) StartBackgroundServices(ctx context.Context) {
	if e.futuresSettings.IsFutures {
		e.StartPositionRiskSync(ctx)
	}
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
