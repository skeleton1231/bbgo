package bbgo

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// paperOrderIDHashMask occupies the top 15 bits of paperOrderID/paperTradeID.
// Bottom 48 bits hold the ms-timestamp-seeded counter — plenty of headroom
// (2^48 ms ≈ 8.9M years). Bits 48-62 carry a per-container hash of
// BBGO_STRATEGY_INSTANCE_ID so that two paper bots sharing one Supabase
// paper_orders table cannot generate overlapping order_id ranges.
// Bit 63 is left clear so every generated ID fits in a PostgreSQL BIGINT
// (signed int64 max = 9223372036854775807).
const paperOrderIDHashShift = 48
const paperOrderIDHashMask = uint64(0x7FFF) << paperOrderIDHashShift

var paperOrderIDHashOffset uint64

var paperOrderID uint64
var paperTradeID uint64

func init() {
	base := uint64(time.Now().UnixNano()) / 1e6
	if id := os.Getenv("BBGO_STRATEGY_INSTANCE_ID"); id != "" {
		h := fnv.New64a()
		_, _ = h.Write([]byte(id))
		paperOrderIDHashOffset = (h.Sum64() & 0x7FFF) << paperOrderIDHashShift
		base &^= paperOrderIDHashMask
	}
	paperOrderID = base | paperOrderIDHashOffset
	paperTradeID = paperOrderID
}

func nextPaperOrderID() uint64 {
	return atomic.AddUint64(&paperOrderID, 1)
}

func nextPaperTradeID() uint64 {
	return atomic.AddUint64(&paperTradeID, 1)
}

// Fee rates approximate Binance retail tiers. Spot is 0.1% for both maker and
// taker. USD-M futures are 0.02% maker / 0.04% taker. Without this split,
// paper-trade PnL overstates futures costs by ~25× and produces strategies
// that look unprofitable in paper but viable live.
const (
	paperSpotMakerFeeRate   = 0.001
	paperSpotTakerFeeRate   = 0.001
	paperFuturesMakerFeeRate = 0.0002
	paperFuturesTakerFeeRate = 0.0004
)

// isPaperLimitTaker checks if a limit order would immediately match on a real exchange.
// Buy limit at or above market price, or sell limit at or below market price → taker.
func isLimitType(t types.OrderType) bool {
	return t == types.OrderTypeLimit || t == types.OrderTypeLimitMaker
}

func isPaperLimitTaker(o types.SubmitOrder, currentPrice fixedpoint.Value) bool {
	if currentPrice.IsZero() || !isLimitType(o.Type) {
		return false
	}
	return (o.Side == types.SideTypeBuy && o.Price.Compare(currentPrice) >= 0) ||
		(o.Side == types.SideTypeSell && o.Price.Compare(currentPrice) <= 0)
}

// paperMatchingBook implements kline-driven order matching for paper trading,
// adapted from backtest.SimplePriceMatching.
type paperMatchingBook struct {
	Symbol  string
	Market  types.Market
	Account *types.Account
	Parent  *PaperTradeExchange // back-reference for futures/margin mode

	mu          sync.Mutex
	bidOrders   []types.Order
	askOrders   []types.Order
	stopOrders  []types.Order // stop orders waiting for price trigger
	lastPrice   fixedpoint.Value
	currentTime time.Time

	tradeUpdateCallbacks   []func(trade types.Trade)
	orderUpdateCallbacks   []func(order types.Order)
	balanceUpdateCallbacks []func(balances types.BalanceMap)
}

func (m *paperMatchingBook) OnTradeUpdate(cb func(trade types.Trade)) {
	m.tradeUpdateCallbacks = append(m.tradeUpdateCallbacks, cb)
}

func (m *paperMatchingBook) EmitTradeUpdate(trade types.Trade) {
	for _, cb := range m.tradeUpdateCallbacks {
		cb(trade)
	}
}

func (m *paperMatchingBook) OnOrderUpdate(cb func(order types.Order)) {
	m.orderUpdateCallbacks = append(m.orderUpdateCallbacks, cb)
}

func (m *paperMatchingBook) EmitOrderUpdate(order types.Order) {
	for _, cb := range m.orderUpdateCallbacks {
		cb(order)
	}
}

func (m *paperMatchingBook) OnBalanceUpdate(cb func(balances types.BalanceMap)) {
	m.balanceUpdateCallbacks = append(m.balanceUpdateCallbacks, cb)
}

func (m *paperMatchingBook) EmitBalanceUpdate(balances types.BalanceMap) {
	for _, cb := range m.balanceUpdateCallbacks {
		cb(balances)
	}
}

func (m *paperMatchingBook) LastPrice() fixedpoint.Value {
	return m.lastPrice
}

func (m *paperMatchingBook) OpenOrders() []types.Order {
	m.mu.Lock()
	defer m.mu.Unlock()
	orders := make([]types.Order, 0, len(m.bidOrders)+len(m.askOrders)+len(m.stopOrders))
	orders = append(orders, m.bidOrders...)
	orders = append(orders, m.askOrders...)
	orders = append(orders, m.stopOrders...)
	return orders
}

// ProcessKLine processes a closed kline and fills matching orders.
func (m *paperMatchingBook) ProcessKLine(kline types.KLine) {
	m.currentTime = kline.EndTime.Time()

	var fills []paperFill

	// Check stop order triggers first — kline high/low determines if stop price was crossed.
	fills = append(fills, m.checkStopTriggers(kline.High, kline.Low)...)

	if m.lastPrice.IsZero() {
		m.lastPrice = kline.Open
	} else {
		if m.lastPrice.Compare(kline.Open) > 0 {
			fills = append(fills, m.sellToPrice(kline.Open)...)
		} else {
			fills = append(fills, m.buyToPrice(kline.Open)...)
		}
	}

	switch kline.Direction() {
	case types.DirectionDown:
		if kline.High.Compare(kline.Open) >= 0 {
			fills = append(fills, m.buyToPrice(kline.High)...)
		}
		if kline.Low.Compare(kline.Close) < 0 {
			fills = append(fills, m.sellToPrice(kline.Low)...)
			fills = append(fills, m.buyToPrice(kline.Close)...)
		} else {
			fills = append(fills, m.sellToPrice(kline.Close)...)
		}

	case types.DirectionUp:
		if kline.Low.Compare(kline.Open) <= 0 {
			fills = append(fills, m.sellToPrice(kline.Low)...)
		}
		if kline.High.Compare(kline.Close) > 0 {
			fills = append(fills, m.buyToPrice(kline.High)...)
			fills = append(fills, m.sellToPrice(kline.Close)...)
		} else {
			fills = append(fills, m.buyToPrice(kline.Close)...)
		}

	default:
		if m.lastPrice.IsZero() {
			fills = append(fills, m.buyToPrice(kline.Close)...)
		}
	}

	if liqFill := m.checkLiquidation(kline); liqFill != nil {
		fills = append(fills, *liqFill)
	}

	m.emitFills(fills)
}

// paperFill records a fill to be emitted after the lock is released.
// This prevents deadlock when callbacks trigger new order submissions.
type paperFill struct {
	Trade    types.Trade
	Order    types.Order
	Balances types.BalanceMap
	// Canceled marks a cancel-only fill (e.g. ReduceOnly order canceled at fill
	// time). emitFills skips TradeUpdate emission for these so the SaaS pipeline
	// doesn't see phantom zero-quantity trades.
	Canceled bool
}

// checkLiquidation force-closes a futures position when the kline's price
// range crosses the liquidation price. Longs liquidate when Low <= liqPrice,
// shorts when High >= liqPrice. The entire position is closed at liqPrice.
func (m *paperMatchingBook) checkLiquidation(kline types.KLine) *paperFill {
	if m.Parent == nil || !m.Parent.futuresSettings.IsFutures {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Parent.mu.Lock()
	defer m.Parent.mu.Unlock()

	state, ok := m.Parent.futuresStates[kline.Symbol]
	if !ok || state.PositionAmount.IsZero() {
		return nil
	}

	risk := m.Parent.computePositionRiskLocked(kline.Symbol)
	liqPrice := risk.LiquidationPrice
	if liqPrice.IsZero() {
		return nil
	}

	var shouldLiquidate bool
	var closeSide types.SideType
	var positionAction string
	if state.PositionAmount.Sign() > 0 {
		shouldLiquidate = kline.Low.Compare(liqPrice) <= 0
		closeSide = types.SideTypeSell
		positionAction = types.PositionActionCloseLong
	} else {
		shouldLiquidate = kline.High.Compare(liqPrice) >= 0
		closeSide = types.SideTypeBuy
		positionAction = types.PositionActionCloseShort
	}

	if !shouldLiquidate {
		return nil
	}

	closeQty := state.PositionAmount.Abs()
	entryPrice := state.EntryPrice

	realizedPnL := m.Parent.computeRealizedPnLLocked(kline.Symbol, closeSide, liqPrice, closeQty)
	m.Parent.updateFuturesPositionLocked(kline.Symbol, closeSide, liqPrice, closeQty, state.StrategyInstanceID)

	now := types.Time(m.currentTime)
	if time.Time(now).IsZero() {
		now = types.Time(time.Now())
	}

	quoteQty := closeQty.Mul(liqPrice)
	fee := quoteQty.Mul(fixedpoint.NewFromFloat(paperFuturesTakerFeeRate))

	// Apply the position-closing effect to the wallet. Without this, the base
	// currency held/owed and the quote currency paid/received at liquidation
	// never hit the balance — the SaaS dashboard would show a stale wallet
	// alongside a zero position, and the originally-locked margin stays locked
	// forever.
	asset := state.MarginAsset
	if asset == "" {
		asset = m.Market.QuoteCurrency
	}
	if closeSide == types.SideTypeSell {
		// Long liquidated → sell the held base back at liqPrice.
		m.Account.AddBalance(m.Market.BaseCurrency, closeQty.Neg())
		m.Account.AddBalance(asset, quoteQty.Sub(fee))
	} else {
		// Short liquidated → buy back the owed base at liqPrice.
		m.Account.AddBalance(m.Market.BaseCurrency, closeQty)
		m.Account.AddBalance(asset, quoteQty.Neg().Sub(fee))
	}
	// Release the initial margin locked when the position was opened.
	leverage := state.Leverage
	if leverage <= 0 {
		leverage = 1
	}
	marginLocked := closeQty.Mul(entryPrice).Div(fixedpoint.NewFromInt(int64(leverage)))
	if marginLocked.Sign() > 0 {
		if err := m.Account.UnlockBalance(asset, marginLocked); err != nil {
			log.WithError(err).Errorf("[papertrade] LIQUIDATION unlock margin failed: symbol=%s asset=%s wanted=%s (possible prior balance corruption)",
				kline.Symbol, asset, marginLocked.String())
		}
	}

	orderID := nextPaperOrderID()

	trade := types.Trade{
		ID:                 nextPaperTradeID(),
		OrderID:            orderID,
		Exchange:           m.Parent.inner.Name(),
		Symbol:             kline.Symbol,
		Side:               closeSide,
		Price:              liqPrice,
		Quantity:           closeQty,
		QuoteQuantity:      quoteQty,
		IsBuyer:            closeSide == types.SideTypeBuy,
		IsMaker:            false,
		Fee:                fee,
		FeeCurrency:        m.Market.QuoteCurrency,
		StrategyInstanceID: state.StrategyInstanceID,
		Time:               now,
		IsFutures:          true,
		PositionAction:     positionAction,
	}
	if realizedPnL != 0.0 {
		trade.PnL = sql.NullFloat64{Float64: realizedPnL, Valid: true}
	}

	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:       kline.Symbol,
			Side:         closeSide,
			Type:         types.OrderTypeMarket,
			Price:        liqPrice,
			Quantity:     closeQty,
			AveragePrice: liqPrice,
			PositionAction: positionAction,
		},
		Exchange:           m.Parent.inner.Name(),
		OrderID:            orderID,
		ExecutedQuantity:   closeQty,
		Status:             types.OrderStatusFilled,
		StrategyInstanceID: state.StrategyInstanceID,
		UpdateTime:         now,
		IsFutures:          true,
	}

	log.Infof("paper trade: LIQUIDATION %s %s qty=%s @ %s (entry was %s) PnL=%.4f",
		kline.Symbol, closeSide, closeQty.String(), liqPrice.String(), entryPrice.String(), realizedPnL)

	return &paperFill{
		Trade:    trade,
		Order:    order,
		Balances: m.Account.Balances(),
	}
}

// buyToPrice simulates price going up — limit sell orders get filled.
func (m *paperMatchingBook) buyToPrice(price fixedpoint.Value) []paperFill {
	m.mu.Lock()
	defer m.mu.Unlock()

	var fills []paperFill
	var remainingAsk []types.Order
	for _, o := range m.askOrders {
		if isLimitType(o.Type) && price.Compare(o.Price) >= 0 {
			fillPrice := o.Price
			if o.Price.Compare(m.lastPrice) < 0 {
				fillPrice = m.lastPrice
			}
			fills = append(fills, m.buildFillLocked(o, fillPrice, false))
		} else {
			remainingAsk = append(remainingAsk, o)
		}
	}
	m.askOrders = remainingAsk
	m.lastPrice = price
	return fills
}

// sellToPrice simulates price going down — limit buy orders get filled.
func (m *paperMatchingBook) sellToPrice(price fixedpoint.Value) []paperFill {
	m.mu.Lock()
	defer m.mu.Unlock()

	var fills []paperFill
	var remainingBid []types.Order
	for _, o := range m.bidOrders {
		if isLimitType(o.Type) && price.Compare(o.Price) <= 0 {
			fillPrice := o.Price
			if o.Price.Compare(m.lastPrice) > 0 {
				fillPrice = m.lastPrice
			}
			fills = append(fills, m.buildFillLocked(o, fillPrice, false))
		} else {
			remainingBid = append(remainingBid, o)
		}
	}
	m.bidOrders = remainingBid
	m.lastPrice = price
	return fills
}

// emitFills emits all fill callbacks outside the lock.
// Cancel-only fills (ReduceOnly orders canceled at fill time) are flagged via
// paperFill.Canceled — we skip trade emission for those so the SaaS pipeline
// doesn't see phantom zero-quantity trades.
func (m *paperMatchingBook) emitFills(fills []paperFill) {
	for _, f := range fills {
		if f.Canceled {
			m.EmitOrderUpdate(f.Order)
			m.EmitBalanceUpdate(f.Balances)
			continue
		}
		m.EmitTradeUpdate(f.Trade)
		m.EmitOrderUpdate(f.Order)
		m.EmitBalanceUpdate(f.Balances)
		log.Infof("[papertrade] FILLED: order=%d %s %s @ %s qty=%s",
			f.Order.OrderID, f.Order.Side, f.Order.Symbol, f.Order.Price.String(), f.Order.Quantity.String())
	}
}

// checkStopTriggers processes stop orders whose stop price was crossed by the kline.
// Triggered stops are immediately filled at market price (stop-market) or limit price (stop-limit).
func (m *paperMatchingBook) checkStopTriggers(high, low fixedpoint.Value) []paperFill {
	m.mu.Lock()
	defer m.mu.Unlock()

	var fills []paperFill
	var remaining []types.Order

	for _, order := range m.stopOrders {
		stopPrice := order.StopPrice
		if stopPrice.IsZero() {
			remaining = append(remaining, order)
			continue
		}

		triggered := false
		switch order.Side {
		case types.SideTypeBuy:
			// Buy stop triggers when price rises above stop price
			if high.Compare(stopPrice) >= 0 {
				triggered = true
			}
		case types.SideTypeSell:
			// Sell stop triggers when price falls below stop price
			if low.Compare(stopPrice) <= 0 {
				triggered = true
			}
		}

		if !triggered {
			remaining = append(remaining, order)
			continue
		}

		fillPrice := order.Price
		if order.Type == types.OrderTypeStopMarket || fillPrice.IsZero() {
			fillPrice = m.lastPrice
			if fillPrice.IsZero() {
				fillPrice = stopPrice
			}
		}
		fillPrice = m.Market.TruncatePrice(fillPrice)

		order.Status = types.OrderStatusFilled
		order.ExecutedQuantity = order.Quantity
		order.IsWorking = false
		order.Price = fillPrice
		fills = append(fills, m.buildFillLocked(order, fillPrice, true))
		// Release the margin that was locked when the stop was placed.
		// buildFillLocked's UseLockedBalance call tries to consume the full
		// notional, which silently fails for futures/margin stops (only
		// stopPrice*qty/leverage was locked at submit). Without this unlock,
		// every triggered stop leaves its margin permanently locked.
		if m.Parent != nil && (m.Parent.futuresSettings.IsFutures || m.Parent.marginSettings.IsMargin) {
			if order.Side == types.SideTypeSell {
				m.unlockMarginSell(order, stopPrice)
			} else {
				leverage := m.Parent.effectiveLeverage(order.Symbol)
				if leverage.Sign() > 0 {
					marginLocked := order.Quantity.Mul(stopPrice).Div(leverage)
					if marginLocked.Sign() > 0 {
						m.Account.UnlockBalance(m.Market.QuoteCurrency, marginLocked)
					}
				}
			}
		}
		log.Infof("[papertrade] STOP TRIGGERED: order=%d %s %s stop=%s fill=%s qty=%s",
			order.OrderID, order.Side, order.Symbol, stopPrice.String(), fillPrice.String(), order.Quantity.String())
	}

	m.stopOrders = remaining
	return fills
}

// reduceOnlyCancelFillLocked builds a cancel-only paperFill for a ReduceOnly
// order that cannot be filled (position gone or would flip direction). The
// Canceled flag signals to emitFills that no trade should be emitted.
func (m *paperMatchingBook) reduceOnlyCancelFillLocked(order types.Order, reason string) paperFill {
	canceled := order
	canceled.Status = types.OrderStatusCanceled
	canceled.IsWorking = false
	canceled.UpdateTime = types.Time(time.Now())
	log.Infof("[papertrade] REDUCE-ONLY CANCELED: order=%d %s %s — %s",
		canceled.OrderID, canceled.Side, canceled.Symbol, reason)
	return paperFill{
		Trade:    types.Trade{},
		Order:    canceled,
		Balances: m.Account.Balances(),
		Canceled: true,
	}
}

func (m *paperMatchingBook) buildFillLocked(order types.Order, fillPrice fixedpoint.Value, isTaker bool) paperFill {
	now := types.Time(m.currentTime)
	if time.Time(now).IsZero() {
		now = types.Time(time.Now())
	}

	isFutures := false
	isMargin := false
	if m.Parent != nil {
		isFutures = m.Parent.futuresSettings.IsFutures
		isMargin = m.Parent.marginSettings.IsMargin
	}

	// Re-validate ReduceOnly at fill time. The position may have changed between
	// SubmitOrder and fill (e.g., another order closed it). Real exchanges
	// cancel ReduceOnly orders that would flip the position; without this,
	// SaaS users testing stop-loss strategies in paper mode see different
	// behavior than live.
	//
	// Note: this block and the futures-state update below both acquire
	// m.Parent.mu separately. They cannot be combined because this check
	// may early-return (cancel), skipping the later update. Lock order
	// m.mu -> m.Parent.mu is the documented safe direction; no reverse
	// path exists.
	if m.Parent != nil && order.ReduceOnly && (isFutures || isMargin) {
		m.Parent.mu.Lock()
		state := m.Parent.getOrCreateFuturesState(order.Symbol)
		posAmt := state.PositionAmount
		m.Parent.mu.Unlock()

		if posAmt.IsZero() {
			return m.reduceOnlyCancelFillLocked(order, "position closed before fill")
		}
		canReduce := (posAmt.Sign() > 0 && order.Side == types.SideTypeSell) ||
			(posAmt.Sign() < 0 && order.Side == types.SideTypeBuy)
		if !canReduce {
			return m.reduceOnlyCancelFillLocked(order, "would flip position direction")
		}
		if order.Quantity.Compare(posAmt.Abs()) > 0 {
			order.Quantity = posAmt.Abs()
		}
	}

	var feeRate fixedpoint.Value
	switch {
	case isFutures && isTaker:
		feeRate = fixedpoint.NewFromFloat(paperFuturesTakerFeeRate)
	case isFutures:
		feeRate = fixedpoint.NewFromFloat(paperFuturesMakerFeeRate)
	case isTaker:
		feeRate = fixedpoint.NewFromFloat(paperSpotTakerFeeRate)
	default:
		feeRate = fixedpoint.NewFromFloat(paperSpotMakerFeeRate)
	}
	quoteQty := order.Quantity.Mul(fillPrice)
	fee := quoteQty.Mul(feeRate)

	trade := types.Trade{
		ID:                 nextPaperTradeID(),
		OrderID:            order.OrderID,
		Exchange:           order.Exchange,
		Symbol:             order.Symbol,
		Side:               order.Side,
		Price:              fillPrice,
		Quantity:           order.Quantity,
		QuoteQuantity:      quoteQty,
		IsBuyer:            order.Side == types.SideTypeBuy,
		IsMaker:            !isTaker,
		Fee:                fee,
		FeeCurrency:        m.Market.QuoteCurrency,
		StrategyInstanceID: order.StrategyInstanceID,
		Time:               now,
		IsFutures:          isFutures,
		IsMargin:           isMargin,
		IsIsolated:         (m.Parent != nil && (m.Parent.futuresSettings.IsIsolatedFutures || m.Parent.marginSettings.IsIsolatedMargin)),
	}

	switch order.Side {
	case types.SideTypeBuy:
		m.Account.UseLockedBalance(m.Market.QuoteCurrency, quoteQty)
		m.Account.AddBalance(m.Market.BaseCurrency, order.Quantity)
		m.Account.AddBalance(m.Market.QuoteCurrency, fee.Neg())

	case types.SideTypeSell:
		if isFutures || isMargin {
			baseBal, _ := m.Account.Balance(m.Market.BaseCurrency)
			if baseBal.Locked.Compare(order.Quantity) >= 0 {
				m.Account.UseLockedBalance(m.Market.BaseCurrency, order.Quantity)
			} else if baseBal.Locked.Sign() > 0 {
				m.Account.UseLockedBalance(m.Market.BaseCurrency, baseBal.Locked)
			}
			m.Account.AddBalance(m.Market.QuoteCurrency, quoteQty.Sub(fee))
		} else {
			m.Account.UseLockedBalance(m.Market.BaseCurrency, order.Quantity)
			m.Account.AddBalance(m.Market.QuoteCurrency, quoteQty.Sub(fee))
		}
	}

	// Update futures position tracking under the exchange mutex.
	// Lock ordering: m.mu -> e.mu is safe; no reverse path exists.
	if m.Parent != nil && isFutures {
		m.Parent.mu.Lock()
		state, ok := m.Parent.futuresStates[order.Symbol]
		var baseBefore fixedpoint.Value
		if ok {
			baseBefore = state.PositionAmount
		}
		realizedPnL := m.Parent.computeRealizedPnLLocked(order.Symbol, order.Side, fillPrice, order.Quantity)
		if realizedPnL != 0.0 {
			trade.PnL = sql.NullFloat64{Float64: realizedPnL, Valid: true}
		}
		m.Parent.updateFuturesPositionLocked(order.Symbol, order.Side, fillPrice, order.Quantity, order.StrategyInstanceID)
		m.Parent.mu.Unlock()
		// Compute position action from the state BEFORE this trade so the SaaS
		// frontend and downstream trade collectors can distinguish opens / adds /
		// reduces / closes / flips without re-deriving from position history.
		trade.PositionAction = types.ComputePositionActionFromState(baseBefore, order.Side, order.Quantity, true)
	}

	filled := order
	filled.Status = types.OrderStatusFilled
	filled.ExecutedQuantity = order.Quantity
	filled.AveragePrice = fillPrice
	filled.IsWorking = false
	filled.UpdateTime = now
	filled.PositionAction = trade.PositionAction

	return paperFill{
		Trade:    trade,
		Order:    filled,
		Balances: m.Account.Balances(),
	}
}

// unlockMarginSell releases whatever the submit path locked for a futures/margin
// SELL order. Submit locks base when the user has it; otherwise locks quote
// (margin for a short). Mirroring that branch here prevents stranded balances
// when the user closes a base-held position via SELL.
func (m *paperMatchingBook) unlockMarginSell(order types.Order, price fixedpoint.Value) {
	if m.Parent == nil || !(m.Parent.futuresSettings.IsFutures || m.Parent.marginSettings.IsMargin) {
		return
	}
	baseBal, _ := m.Account.Balance(m.Market.BaseCurrency)
	if baseBal.Locked.Compare(order.Quantity) >= 0 {
		_ = m.Account.UnlockBalance(m.Market.BaseCurrency, order.Quantity)
		return
	}
	leverage := m.Parent.effectiveLeverage(order.Symbol)
	if leverage.Sign() <= 0 {
		return
	}
	marginLocked := order.Quantity.Mul(price).Div(leverage)
	if marginLocked.Sign() > 0 {
		_ = m.Account.UnlockBalance(m.Market.QuoteCurrency, marginLocked)
	}
}

// CancelOrder removes an order from the book and unlocks balance.
func (m *paperMatchingBook) CancelOrder(order types.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	isStop := order.Type == types.OrderTypeStopLimit || order.Type == types.OrderTypeStopMarket ||
		order.Type == types.OrderTypeTakeProfit || order.Type == types.OrderTypeTakeProfitMarket

	if isStop {
		var remaining []types.Order
		for _, o := range m.stopOrders {
			if o.OrderID == order.OrderID {
				// Unlock the margin that was locked when the stop was placed
				if m.Parent != nil && (m.Parent.futuresSettings.IsFutures || m.Parent.marginSettings.IsMargin) {
					if order.Side == types.SideTypeSell {
						m.unlockMarginSell(order, order.StopPrice)
					} else {
						leverage := m.Parent.effectiveLeverage(order.Symbol)
						lockAmt := order.Quantity.Mul(order.StopPrice).Div(leverage)
						m.Account.UnlockBalance(m.Market.QuoteCurrency, lockAmt)
					}
				} else if order.Side == types.SideTypeBuy {
					m.Account.UnlockBalance(m.Market.QuoteCurrency, order.Price.Mul(order.Quantity))
				} else {
					m.Account.UnlockBalance(m.Market.BaseCurrency, order.Quantity)
				}
				continue
			}
			remaining = append(remaining, o)
		}
		m.stopOrders = remaining
	} else {
		switch order.Side {
		case types.SideTypeBuy:
			var remaining []types.Order
			for _, o := range m.bidOrders {
				if o.OrderID == order.OrderID {
					if m.Parent != nil && (m.Parent.futuresSettings.IsFutures || m.Parent.marginSettings.IsMargin) {
						// Futures/margin buy locks notional/leverage at submit; unlocking the full
						// notional here would inflate available balance by (leverage-1)/leverage.
						leverage := m.Parent.effectiveLeverage(o.Symbol)
						if leverage > 0 {
							m.Account.UnlockBalance(m.Market.QuoteCurrency, o.Price.Mul(o.Quantity).Div(leverage))
						}
					} else {
						m.Account.UnlockBalance(m.Market.QuoteCurrency, o.Price.Mul(o.Quantity))
					}
					continue
				}
				remaining = append(remaining, o)
			}
			m.bidOrders = remaining

		case types.SideTypeSell:
			var remaining []types.Order
			for _, o := range m.askOrders {
				if o.OrderID == order.OrderID {
					if m.Parent != nil && (m.Parent.futuresSettings.IsFutures || m.Parent.marginSettings.IsMargin) {
						m.unlockMarginSell(o, o.Price)
					} else {
						m.Account.UnlockBalance(m.Market.BaseCurrency, o.Quantity)
					}
					continue
				}
				remaining = append(remaining, o)
			}
			m.askOrders = remaining
		}
	}

	canceled := order
	canceled.Status = types.OrderStatusCanceled
	canceled.IsWorking = false
	canceled.UpdateTime = types.Time(time.Now())
	m.EmitOrderUpdate(canceled)
	m.EmitBalanceUpdate(m.Account.Balances())
}

// PaperTradeExchange wraps a real exchange for market data and simulates order fills
// locally using a kline-driven matching engine. Follows the same pattern as
// backtest.Exchange — uses types.Account for balance management with
// LockBalance/UnlockBalance/UseLockedBalance/AddBalance.
type PaperTradeExchange struct {
	inner   types.Exchange
	markets types.MarketMap

	account *types.Account

	matchingBooks   map[string]*paperMatchingBook
	mu              sync.Mutex
	userDataEmitter types.StandardStreamEmitter
	db               *sqlx.DB // nil when not in DB mode
	tablePrefix      string
	userID           string
	strategyInstance string // isolates per-container queries (open-order restore) in shared DB

	// Futures state per symbol
	futuresSettings types.FuturesSettings
	futuresStates   map[string]*paperFuturesState

	// Margin state per asset
	marginSettings types.MarginSettings
	marginStates   map[string]*paperMarginState

	// OnPeriodicNAVRecord is fired every minute so the environment layer
	// (which owns PriceSolver + AccountService) can persist a NAV snapshot.
	// PaperTradeExchange itself cannot compute USD NAV — it lacks the
	// price solver and the session reference. Without this ticker,
	// nav_history_details stays empty for paper sessions and the equity
	// curve has to be reconstructed from trades/profits only.
	OnPeriodicNAVRecord func(time.Time)

	// Persistence callbacks for margin/funding events. Set by the
	// environment layer which owns MarginService. Without these,
	// margin_loans/margin_repays/margin_interests tables stay empty
	// for paper sessions and funding payments are invisible to PnL.
	OnMarginLoan     func(types.MarginLoan)
	OnMarginRepay    func(types.MarginRepay)
	OnMarginInterest func(types.MarginInterest)
	OnFundingPayment func(types.FundingPayment)

	// Real funding-rate fetcher (optional). When the wrapped exchange
	// supports QueryPremiumIndex (Binance does), paper mode pulls the
	// live funding rate per 8h slot instead of using defaultFundingRate.
	// Falls back to the hardcoded rate when nil or fetch fails.
	fundingRateSlot  time.Time
	fundingRateCache map[string]fixedpoint.Value
}

// premiumIndexFetcher is the optional interface the wrapped exchange can
// implement to provide live funding rates. Binance's *exchange.Exchange
// satisfies this.
type premiumIndexFetcher interface {
	QueryPremiumIndex(ctx context.Context, symbol string) (*types.PremiumIndex, error)
}

func NewPaperTradeExchange(inner types.Exchange, markets types.MarketMap, balances types.BalanceMap) *PaperTradeExchange {
	account := &types.Account{
		MakerFeeRate: fixedpoint.MustNewFromString("0.001"),
		TakerFeeRate: fixedpoint.MustNewFromString("0.001"),
		AccountType:  types.AccountTypeSpot,
		CanTrade:     true,
		CanDeposit:   true,
		CanWithdraw:  true,
	}
	account.UpdateBalances(balances)

	e := &PaperTradeExchange{
		inner:            inner,
		markets:          markets,
		account:          account,
		matchingBooks:    make(map[string]*paperMatchingBook),
		futuresStates:    make(map[string]*paperFuturesState),
		marginStates:     make(map[string]*paperMarginState),
		fundingRateCache: make(map[string]fixedpoint.Value),
	}

	for symbol, market := range markets {
		e.matchingBooks[symbol] = &paperMatchingBook{
			Symbol:  symbol,
			Market:  market,
			Account: account,
			Parent:  e,
		}
	}

	return e
}

// OnKLineClosed processes a closed kline through the matching engine.
func (e *PaperTradeExchange) OnKLineClosed(kline types.KLine) {
	matching, ok := e.matchingBook(kline.Symbol)
	if !ok {
		return
	}
	matching.ProcessKLine(kline)
}

// BindUserData connects matching engine callbacks to the UserDataStream.
// Also stores the emitter so lazily-created matching books get bound automatically.
// When DB is configured, adds a balance sync callback to persist every change.
func (e *PaperTradeExchange) BindUserData(userDataStream types.StandardStreamEmitter) {
	e.mu.Lock()
	e.userDataEmitter = userDataStream
	for _, matching := range e.matchingBooks {
		matching.OnTradeUpdate(userDataStream.EmitTradeUpdate)
		matching.OnOrderUpdate(userDataStream.EmitOrderUpdate)
		matching.OnBalanceUpdate(userDataStream.EmitBalanceUpdate)
		if e.db != nil {
			matching.OnBalanceUpdate(func(types.BalanceMap) { e.syncBalances() })
		}
	}
	e.mu.Unlock()
}

// SetDB enables DB persistence for balance sync.
func (e *PaperTradeExchange) SetDB(db *sqlx.DB, tablePrefix string, userID string) {
	e.db = db
	e.tablePrefix = tablePrefix
	e.userID = userID
	e.strategyInstance = os.Getenv("BBGO_STRATEGY_INSTANCE_ID")
}

// EmitBalanceUpdateFromAccount emits a balance update using the current account state.
// Also persists balances to the DB so that margin borrow/repay, interest accrual,
// and funding settlement survive a restart. The matching-book fill path already
// syncs via its own OnBalanceUpdate callback; this covers the non-fill paths.
func (e *PaperTradeExchange) EmitBalanceUpdateFromAccount() {
	if e.userDataEmitter != nil {
		e.userDataEmitter.EmitBalanceUpdate(e.account.Balances())
	}
	e.syncBalances()
}

// RestoreFromDB loads open orders and balances from the database into
// the in-memory paper trade engine. Must be called before strategies run
// so that QueryOpenOrders() returns restored orders and strategies skip
// re-placing existing grids.
func (e *PaperTradeExchange) RestoreFromDB(ctx context.Context) error {
	if e.db == nil {
		return nil
	}

	// 1. Restore balances
	balances, err := e.queryBalances(ctx)
	if err != nil {
		log.WithError(err).Warn("paper trade: failed to restore balances from DB, using config defaults")
	} else if len(balances) > 0 {
		e.account.UpdateBalances(balances)
		log.Infof("paper trade: restored %d balances from DB", len(balances))
	}

	// Seed paper_balances on startup so the dashboard shows balances before
	// the first trade fills. Without this, the table stays empty until an
	// order triggers OnBalanceUpdate.
	e.syncBalances()

	// 2. Restore open orders.
	orders, err := e.queryOpenOrders(ctx, "")
	if err != nil {
		log.WithError(err).Warn("paper trade: failed to restore open orders from DB")
		return nil
	}
	if len(orders) == 0 {
		log.Info("paper trade: no open orders to restore from DB")
		return nil
	}

	var maxOrderID uint64
	for _, order := range orders {
		matching, ok := e.matchingBook(order.Symbol)
		if !ok {
			continue
		}
		matching.mu.Lock()
		isStop := order.Type == types.OrderTypeStopLimit || order.Type == types.OrderTypeStopMarket ||
			order.Type == types.OrderTypeTakeProfit || order.Type == types.OrderTypeTakeProfitMarket
		if isStop {
			matching.stopOrders = append(matching.stopOrders, order)
		} else {
			switch order.Side {
			case types.SideTypeBuy:
				matching.bidOrders = append(matching.bidOrders, order)
			case types.SideTypeSell:
				matching.askOrders = append(matching.askOrders, order)
			}
		}
		matching.mu.Unlock()

		if order.OrderID > maxOrderID {
			maxOrderID = order.OrderID
		}
	}

	// Bump the counter portion to maxOrderID, but preserve this container's
	// hash offset. maxOrderID may come from pre-fix rows (no hash bits); strip
	// any high bits it carries so we don't pick up another strategy's namespace.
	maxCounter := maxOrderID &^ paperOrderIDHashMask
	bumped := maxCounter | paperOrderIDHashOffset
	for {
		current := atomic.LoadUint64(&paperOrderID)
		if bumped <= current || atomic.CompareAndSwapUint64(&paperOrderID, current, bumped) {
			break
		}
	}

	log.Infof("paper trade: restored %d open orders from DB (max order ID: %d)", len(orders), maxOrderID)

	// 3. Restore futures positions if futures mode is enabled.
	if e.futuresSettings.IsFutures {
		if err := e.restoreFuturesPositions(ctx); err != nil {
			log.WithError(err).Warn("paper trade: failed to restore futures positions from DB")
		}
	}

	// 4. Restore margin borrow state if margin mode is enabled.
	if e.marginSettings.IsMargin {
		if err := e.restoreMarginStates(ctx); err != nil {
			log.WithError(err).Warn("paper trade: failed to restore margin states from DB")
		}
	}

	return nil
}

func (e *PaperTradeExchange) tableName(base string) string {
	return e.tablePrefix + base
}

func (e *PaperTradeExchange) effectiveLeverage(symbol string) fixedpoint.Value {
	if state, ok := e.futuresStates[symbol]; ok && state.Leverage > 0 {
		return fixedpoint.NewFromInt(int64(state.Leverage))
	}
	return fixedpoint.One
}

func (e *PaperTradeExchange) queryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	tableName := e.tableName("orders")
	var query string
	var args []interface{}
	if e.db.DriverName() == "postgres" {
		query = `SELECT exchange, CAST(order_id AS BIGINT) as order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at, updated_at, is_margin, is_futures, is_isolated, order_uuid as uuid, actual_order_id, strategy_instance_id FROM "` + tableName + `" WHERE user_id = $1 AND status IN ('NEW', 'PARTIALLY_FILLED')`
		args = append(args, e.userID)
		// In multi-tenant shared tables, restrict to this container's own orders so
		// restart-time restore does not pull in orders belonging to other strategies.
		if e.strategyInstance != "" {
			query += " AND strategy_instance_id = $2"
			args = append(args, e.strategyInstance)
		}
	} else {
		query = "SELECT * FROM " + tableName + " WHERE status IN ('NEW', 'PARTIALLY_FILLED')"
	}
	if symbol != "" {
		if e.db.DriverName() == "postgres" {
			nextIdx := len(args) + 1
			query += " AND symbol = $" + strconv.Itoa(nextIdx)
		} else {
			query += " AND symbol = ?"
		}
		args = append(args, symbol)
	}
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []types.Order
	for rows.Next() {
		var order types.Order
		if err := rows.StructScan(&order); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (e *PaperTradeExchange) queryBalances(ctx context.Context) (types.BalanceMap, error) {
	tableName := e.tableName("balances")
	var query string
	var args []interface{}
	if e.db.DriverName() == "postgres" {
		query = "SELECT currency, available, locked FROM \"" + tableName + "\" WHERE user_id = $1 AND strategy_instance_id = $2"
		args = append(args, e.userID, e.strategyInstance)
	} else {
		query = "SELECT currency, available, locked FROM " + tableName
	}
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	balances := make(types.BalanceMap)
	for rows.Next() {
		var b types.Balance
		if err := rows.StructScan(&b); err != nil {
			return nil, err
		}
		balances[b.Currency] = b
	}
	return balances, rows.Err()
}

// restoreFuturesPositions loads open futures positions from the latest
// futures_position_risks snapshot and restores paperFuturesState so the
// paper engine continues existing positions after a container restart.
func (e *PaperTradeExchange) restoreFuturesPositions(ctx context.Context) error {
	if e.db == nil {
		return nil
	}

	tableName := e.tableName("futures_position_risks")
	var query string
	var args []interface{}

	if e.db.DriverName() == "postgres" {
		// Get the latest row per symbol where position_amount != '0'.
		// Filter by strategy_instance_id when set so a container restart in
		// multi-instance SaaS mode doesn't pull in positions owned by other
		// containers under the same user_id. updated_at is read so
		// LastFundingTime can be reconstructed to the slot the position was
		// last active in — without this, restart drops catch-up state and
		// any funding slots that elapsed during the downtime are silently
		// never charged.
		query = `SELECT DISTINCT ON (symbol) symbol, position_side, leverage, entry_price, position_amount, margin_asset, strategy_instance_id, updated_at
			FROM "` + tableName + `"
			WHERE user_id = $1`
		args = append(args, e.userID)
		if e.strategyInstance != "" {
			query += " AND strategy_instance_id = $2"
			args = append(args, e.strategyInstance)
		}
		query += " AND position_amount != '0' ORDER BY symbol, updated_at DESC"
	} else {
		query = `SELECT symbol, position_side, leverage, entry_price, position_amount, margin_asset, strategy_instance_id, '' AS updated_at
			FROM ` + tableName + `
			WHERE position_amount != 0 AND rowid IN (
				SELECT MAX(rowid) FROM ` + tableName + ` WHERE position_amount != 0 GROUP BY symbol
			)`
	}

	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query futures positions: %w", err)
	}
	defer rows.Close()

	e.mu.Lock()
	defer e.mu.Unlock()

	var restored int
	for rows.Next() {
		var r struct {
			Symbol             string           `db:"symbol"`
			PositionSide       string           `db:"position_side"`
			Leverage           fixedpoint.Value `db:"leverage"`
			EntryPrice         fixedpoint.Value `db:"entry_price"`
			PositionAmount     fixedpoint.Value `db:"position_amount"`
			MarginAsset        string           `db:"margin_asset"`
			StrategyInstanceID string           `db:"strategy_instance_id"`
			UpdatedAt          time.Time        `db:"updated_at"`
		}
		if err := rows.StructScan(&r); err != nil {
			return fmt.Errorf("scan futures position: %w", err)
		}

		if r.PositionAmount.IsZero() {
			continue
		}

		lev := 20
		if !r.Leverage.IsZero() {
			lev = int(r.Leverage.Int64())
		}

		// Paper engine only supports one-way mode; coerce any legacy
		// ''/Long/Short values to BOTH so a dirty DB snapshot cannot
		// resurrect the bad state across restarts.
		side := types.PositionType(r.PositionSide)
		if side == "" || side == types.PositionLong || side == types.PositionShort {
			side = types.PositionType(PositionModeOneWay)
		}

		// Reconstruct LastFundingTime from the position row's updated_at:
		// the funding slot that contains updated_at is the most recent slot
		// the position was known to be active in. The applyFundingRate loop
		// then catches up chronologically (one slot per hourly tick) until
		// it reaches the current slot. Without this, container downtime
		// silently skips any 8h funding slots elapsed during the outage.
		lastFunding := lastFundingSlotUTC(time.Now())
		if !r.UpdatedAt.IsZero() {
			lastFunding = lastFundingSlotUTC(r.UpdatedAt)
		}

		state := &paperFuturesState{
			Leverage:           lev,
			PositionAmount:     r.PositionAmount,
			EntryPrice:         r.EntryPrice,
			PositionSide:       side,
			MarginAsset:        r.MarginAsset,
			StrategyInstanceID: r.StrategyInstanceID,
			LastFundingTime:    lastFunding,
		}
		e.futuresStates[r.Symbol] = state
		restored++
	}

	if restored > 0 {
		log.Infof("paper trade: restored %d futures positions from DB", restored)
	}
	return rows.Err()
}

// restoreMarginStates reconstructs the in-memory margin borrow state per asset
// by replaying paper_margin_loans minus paper_margin_repays. Without this,
// restart drops the Borrowed/Interest counters to zero, so the next borrow
// records only the new principal (allowing users to "forget" prior debt) and
// the interest clock restarts (undercharging for the elapsed gap).
func (e *PaperTradeExchange) restoreMarginStates(ctx context.Context) error {
	if e.db == nil {
		return nil
	}

	loansTable := e.tableName("margin_loans")
	repaysTable := e.tableName("margin_repays")

	type row struct {
		Asset     string           `db:"asset"`
		Principle fixedpoint.Value `db:"principle"`
	}
	var (
		borrowed = make(map[string]fixedpoint.Value)
		assets   []string
	)

	// Sum loans per asset.
	var loanQuery string
	var loanArgs []interface{}
	if e.db.DriverName() == "postgres" {
		loanQuery = `SELECT asset, SUM(principle::numeric) AS principle FROM "` + loansTable +
			`" WHERE user_id = $1 GROUP BY asset`
		loanArgs = append(loanArgs, e.userID)
	} else {
		loanQuery = `SELECT asset, SUM(principle) AS principle FROM ` + loansTable + ` GROUP BY asset`
	}
	loanRows, err := e.db.QueryxContext(ctx, loanQuery, loanArgs...)
	if err != nil {
		return fmt.Errorf("query margin loans: %w", err)
	}
	for loanRows.Next() {
		var r row
		if err := loanRows.StructScan(&r); err != nil {
			loanRows.Close()
			return fmt.Errorf("scan margin loan: %w", err)
		}
		borrowed[r.Asset] = borrowed[r.Asset].Add(r.Principle)
	}
	loanRows.Close()
	if err := loanRows.Err(); err != nil {
		return fmt.Errorf("iterate margin loans: %w", err)
	}

	// Subtract repays per asset.
	var repayQuery string
	var repayArgs []interface{}
	if e.db.DriverName() == "postgres" {
		repayQuery = `SELECT asset, SUM(principle::numeric) AS principle FROM "` + repaysTable +
			`" WHERE user_id = $1 GROUP BY asset`
		repayArgs = append(repayArgs, e.userID)
	} else {
		repayQuery = `SELECT asset, SUM(principle) AS principle FROM ` + repaysTable + ` GROUP BY asset`
	}
	repayRows, err := e.db.QueryxContext(ctx, repayQuery, repayArgs...)
	if err != nil {
		return fmt.Errorf("query margin repays: %w", err)
	}
	for repayRows.Next() {
		var r row
		if err := repayRows.StructScan(&r); err != nil {
			repayRows.Close()
			return fmt.Errorf("scan margin repay: %w", err)
		}
		borrowed[r.Asset] = borrowed[r.Asset].Sub(r.Principle)
	}
	repayRows.Close()
	if err := repayRows.Err(); err != nil {
		return fmt.Errorf("iterate margin repays: %w", err)
	}

	// Resolve the last interest accrual time per asset so the clock resumes
	// correctly after restart. Without this, LastAccrual=time.Now() gifts the
	// borrower the entire restart gap as interest-free.
	interestTable := e.tableName("margin_interests")
	lastAccrual := make(map[string]time.Time)
	var interestQuery string
	var interestArgs []interface{}
	if e.db.DriverName() == "postgres" {
		interestQuery = `SELECT asset, MAX(time) AS last_time FROM "` + interestTable +
			`" WHERE user_id = $1 GROUP BY asset`
		interestArgs = append(interestArgs, e.userID)
	} else {
		interestQuery = `SELECT asset, MAX(time) AS last_time FROM ` + interestTable + ` GROUP BY asset`
	}
	interestRows, err := e.db.QueryxContext(ctx, interestQuery, interestArgs...)
	if err != nil {
		return fmt.Errorf("query margin interest last time: %w", err)
	}
	for interestRows.Next() {
		var r struct {
			Asset    string    `db:"asset"`
			LastTime time.Time `db:"last_time"`
		}
		if err := interestRows.StructScan(&r); err != nil {
			interestRows.Close()
			return fmt.Errorf("scan margin interest last time: %w", err)
		}
		lastAccrual[r.Asset] = r.LastTime
	}
	interestRows.Close()
	if err := interestRows.Err(); err != nil {
		return fmt.Errorf("iterate margin interest last time: %w", err)
	}

	// For assets with no prior interest rows, fall back to the earliest loan
	// time so the clock starts from the original borrow.
	earliestLoan := make(map[string]time.Time)
	var loanTimeQuery string
	var loanTimeArgs []interface{}
	if e.db.DriverName() == "postgres" {
		loanTimeQuery = `SELECT asset, MIN(time) AS first_time FROM "` + loansTable +
			`" WHERE user_id = $1 GROUP BY asset`
		loanTimeArgs = append(loanTimeArgs, e.userID)
	} else {
		loanTimeQuery = `SELECT asset, MIN(time) AS first_time FROM ` + loansTable + ` GROUP BY asset`
	}
	loanTimeRows, err := e.db.QueryxContext(ctx, loanTimeQuery, loanTimeArgs...)
	if err != nil {
		return fmt.Errorf("query margin loan first time: %w", err)
	}
	for loanTimeRows.Next() {
		var r struct {
			Asset     string    `db:"asset"`
			FirstTime time.Time `db:"first_time"`
		}
		if err := loanTimeRows.StructScan(&r); err != nil {
			loanTimeRows.Close()
			return fmt.Errorf("scan margin loan first time: %w", err)
		}
		earliestLoan[r.Asset] = r.FirstTime
	}
	loanTimeRows.Close()
	if err := loanTimeRows.Err(); err != nil {
		return fmt.Errorf("iterate margin loan first time: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	var restored int
	for asset, amt := range borrowed {
		if amt.Sign() <= 0 {
			continue
		}
		state := e.getOrCreateMarginState(asset)
		state.Borrowed = amt
		// Resume the interest clock from the last accrual (or earliest loan if
		// interest never fired). Historical interest rows already debited the
		// wallet, so we must NOT use time.Now() — that would gift the borrower
		// the restart gap as interest-free.
		switch {
		case !lastAccrual[asset].IsZero():
			state.LastAccrual = lastAccrual[asset]
		case !earliestLoan[asset].IsZero():
			state.LastAccrual = earliestLoan[asset]
		default:
			state.LastAccrual = time.Now()
		}
		state.InterestRate = fixedpoint.MustNewFromString(defaultHourlyMarginRate)
		restored++
		assets = append(assets, asset)
	}
	if restored > 0 {
		log.Infof("paper trade: restored %d margin states from DB (assets: %v)", restored, assets)
	}
	return nil
}

// syncBalances writes current balances to the database.
func (e *PaperTradeExchange) syncBalances() {
	if e.db == nil {
		return
	}
	if err := e.upsertBalances(); err != nil {
		log.WithError(err).Warn("paper trade: failed to sync balances to DB")
	}
}

func (e *PaperTradeExchange) upsertBalances() error {
	tableName := e.tableName("balances")
	balances := e.account.Balances()
	for currency, b := range balances {
		var sql string
		switch e.db.DriverName() {
		case "postgres":
			sql = `INSERT INTO "` + tableName + `" (user_id, strategy_instance_id, currency, total, available, locked) VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (user_id, strategy_instance_id, currency) DO UPDATE SET total = $4, available = $5, locked = $6, updated_at = NOW()`
			_, err := e.db.Exec(sql, e.userID, e.strategyInstance, currency, b.Total().String(), b.Available.String(), b.Locked.String())
			if err != nil {
				return err
			}
		default:
			sql = `INSERT OR REPLACE INTO ` + tableName + ` (currency, total, available, locked) VALUES (?, ?, ?, ?)`
			_, err := e.db.Exec(sql, currency, b.Total(), b.Available, b.Locked)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *PaperTradeExchange) matchingBook(symbol string) (*paperMatchingBook, bool) {
	e.mu.Lock()
	m, ok := e.matchingBooks[symbol]
	if !ok {
		if market, hasMarket := e.markets[symbol]; hasMarket {
			m = &paperMatchingBook{
				Symbol:  symbol,
				Market:  market,
				Account: e.account,
				Parent:  e,
			}
			if e.userDataEmitter != nil {
				m.OnTradeUpdate(e.userDataEmitter.EmitTradeUpdate)
				m.OnOrderUpdate(e.userDataEmitter.EmitOrderUpdate)
				m.OnBalanceUpdate(e.userDataEmitter.EmitBalanceUpdate)
			}
			if e.db != nil {
				m.OnBalanceUpdate(func(types.BalanceMap) { e.syncBalances() })
			}
			e.matchingBooks[symbol] = m
			ok = true
		}
	}
	e.mu.Unlock()
	return m, ok
}

// --- ExchangeMinimal ---

func (e *PaperTradeExchange) Name() types.ExchangeName    { return e.inner.Name() }
func (e *PaperTradeExchange) PlatformFeeCurrency() string { return e.inner.PlatformFeeCurrency() }
func (e *PaperTradeExchange) String() string              { return fmt.Sprintf("PaperTrade<%s>", e.inner.Name()) }

// --- ExchangeMarketDataService (delegate to real exchange) ---

func (e *PaperTradeExchange) NewStream() types.Stream { return e.inner.NewStream() }

func (e *PaperTradeExchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	return e.markets, nil
}

func (e *PaperTradeExchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	return e.inner.QueryTicker(ctx, symbol)
}

func (e *PaperTradeExchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	return e.inner.QueryTickers(ctx, symbol...)
}

func (e *PaperTradeExchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	return e.inner.QueryKLines(ctx, symbol, interval, options)
}

// --- ExchangeAccountService (virtual) ---

func (e *PaperTradeExchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	return e.account, nil
}

func (e *PaperTradeExchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	return e.account.Balances(), nil
}

// --- ExchangeTradeService (virtual) ---

func (e *PaperTradeExchange) SubmitOrder(ctx context.Context, submit types.SubmitOrder) (*types.Order, error) {
	matching, ok := e.matchingBook(submit.Symbol)
	if !ok {
		return nil, fmt.Errorf("paper trade: matching engine not initialized for symbol %s", submit.Symbol)
	}

	market, ok := e.markets[submit.Symbol]
	if !ok {
		return nil, fmt.Errorf("paper trade: market not found for symbol %s", submit.Symbol)
	}

	if submit.Quantity.Sign() <= 0 {
		return nil, fmt.Errorf("paper trade: order quantity must be positive")
	}

	if submit.ReduceOnly && (e.futuresSettings.IsFutures || e.marginSettings.IsMargin) {
		e.mu.Lock()
		state := e.getOrCreateFuturesState(submit.Symbol)
		posAmt := state.PositionAmount
		e.mu.Unlock()

		if posAmt.IsZero() {
			return nil, fmt.Errorf("paper trade: reduceOnly order rejected — no open position for %s", submit.Symbol)
		}

		canReduce := (posAmt.Sign() > 0 && submit.Side == types.SideTypeSell) ||
			(posAmt.Sign() < 0 && submit.Side == types.SideTypeBuy)

		if !canReduce {
			return nil, fmt.Errorf("paper trade: reduceOnly order rejected — %s %s would increase position (current: %s)",
				submit.Side, submit.Symbol, posAmt.String())
		}

		if submit.Quantity.Compare(posAmt.Abs()) > 0 {
			submit.Quantity = posAmt.Abs()
		}
	}

	orderID := nextPaperOrderID()
	now := types.Time(time.Now())

	price := submit.Price
	if submit.Type == types.OrderTypeMarket {
		price = market.TruncatePrice(matching.LastPrice())
		if price.IsZero() {
			return nil, fmt.Errorf("paper trade: cannot place market order before receiving market data for %s", submit.Symbol)
		}
	}
	// Stop orders (esp. stop-market) carry the trigger price in StopPrice,
	// not Price. Lock margin against StopPrice so cancel/trigger paths can
	// symmetrically unlock the same amount.
	if price.IsZero() && !submit.StopPrice.IsZero() {
		price = submit.StopPrice
	}

	order := types.Order{
		SubmitOrder:      submit,
		Exchange:         e.inner.Name(),
		OrderID:          orderID,
		Status:           types.OrderStatusNew,
		ExecutedQuantity: fixedpoint.Zero,
		IsWorking:        true,
		CreationTime:     now,
		UpdateTime:       now,
	}
	order.StrategyInstanceID = submit.StrategyInstanceID
	order.IsFutures = e.futuresSettings.IsFutures
	order.IsMargin = e.marginSettings.IsMargin
	order.IsIsolated = e.futuresSettings.IsIsolatedFutures || e.marginSettings.IsIsolatedMargin
	if submit.Type == types.OrderTypeMarket {
		order.Price = price
	}

	// Lock balance
	isFutures := e.futuresSettings.IsFutures
	isMargin := e.marginSettings.IsMargin
	leverage := e.effectiveLeverage(submit.Symbol)

	switch submit.Side {
	case types.SideTypeBuy:
		lockAmt := submit.Quantity.Mul(price)
		if isFutures || isMargin {
			lockAmt = lockAmt.Div(leverage)
		}
		if err := e.account.LockBalance(market.QuoteCurrency, lockAmt); err != nil {
			return nil, fmt.Errorf("paper trade: %w", err)
		}
	case types.SideTypeSell:
		if isFutures || isMargin {
			// Futures/margin: lock margin (notional / leverage), allow short
			lockAmt := submit.Quantity.Mul(price).Div(leverage)
			baseBal, _ := e.account.Balance(market.BaseCurrency)
			if baseBal.Available.Compare(submit.Quantity) >= 0 {
				if err := e.account.LockBalance(market.BaseCurrency, submit.Quantity); err != nil {
					return nil, fmt.Errorf("paper trade: %w", err)
				}
			} else if lockAmt.Sign() > 0 {
				if err := e.account.LockBalance(market.QuoteCurrency, lockAmt); err != nil {
					return nil, fmt.Errorf("paper trade: %w", err)
				}
			}
		} else {
			if err := e.account.LockBalance(market.BaseCurrency, submit.Quantity); err != nil {
				return nil, fmt.Errorf("paper trade: %w", err)
			}
		}
	}

	matching.EmitBalanceUpdate(e.account.Balances())
	matching.EmitOrderUpdate(order)

	// Market orders and taker limit orders fill immediately at market price
	isStop := submit.Type == types.OrderTypeStopLimit || submit.Type == types.OrderTypeStopMarket ||
		submit.Type == types.OrderTypeTakeProfit || submit.Type == types.OrderTypeTakeProfitMarket

	if isStop {
		matching.mu.Lock()
		matching.stopOrders = append(matching.stopOrders, order)
		matching.mu.Unlock()

		log.Infof("[papertrade] stop order placed: %s %s %s stop=%s price=%s qty=%s id=%d",
			order.Side, order.Type, order.Symbol, order.StopPrice.String(), order.Price.String(), order.Quantity.String(), order.OrderID)
		return &order, nil
	}

	isTaker := submit.Type == types.OrderTypeMarket || isPaperLimitTaker(submit, matching.LastPrice())
	if isTaker {
		fillPrice := price
		if submit.Type == types.OrderTypeLimit && !matching.LastPrice().IsZero() {
			fillPrice = market.TruncatePrice(matching.LastPrice())
		}

		matching.mu.Lock()
		fill := matching.buildFillLocked(order, fillPrice, true)
		matching.mu.Unlock()

		// For taker limit BUY orders, refund the difference between locked
		// and used (locked limitPrice*qty, used fillPrice*qty). SELL never
		// over-locks — the fill already credits fillPrice*qty to quote, so
		// any additional surplus refund would double-count.
		if submit.Type == types.OrderTypeLimit && submit.Side == types.SideTypeBuy {
			refunded := false
			// Futures/margin locks notional/leverage at submit, so the refund
			// must be scaled by 1/leverage to match. Without this, a 10x
			// futures taker limit buy would unlock 10× the excess margin.
			var leverage fixedpoint.Value
			if isFutures || isMargin {
				leverage = e.effectiveLeverage(submit.Symbol)
			}
			refund := price.Sub(fillPrice).Mul(submit.Quantity)
			if refund.Sign() > 0 {
				if leverage.Sign() > 0 {
					refund = refund.Div(leverage)
				}
				e.account.UnlockBalance(market.QuoteCurrency, refund)
				refunded = true
			}
			if refunded {
				e.EmitBalanceUpdateFromAccount()
			}
		}

		matching.emitFills([]paperFill{fill})
		return &order, nil
	}

	// Limit maker orders go into the book
	matching.mu.Lock()
	switch submit.Side {
	case types.SideTypeBuy:
		matching.bidOrders = append(matching.bidOrders, order)
	case types.SideTypeSell:
		matching.askOrders = append(matching.askOrders, order)
	}
	matching.mu.Unlock()

	log.Infof("[papertrade] order placed: %s %s %s @ %s qty=%s id=%d",
		order.Side, order.Type, order.Symbol, order.Price.String(), order.Quantity.String(), order.OrderID)

	return &order, nil
}

func (e *PaperTradeExchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	for _, order := range orders {
		matching, ok := e.matchingBook(order.Symbol)
		if !ok {
			continue
		}
		matching.CancelOrder(order)
		log.Infof("[papertrade] order canceled: %d %s %s", order.OrderID, order.Side, order.Symbol)
	}
	return nil
}

func (e *PaperTradeExchange) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
	matching, ok := e.matchingBook(symbol)
	if !ok {
		return nil, fmt.Errorf("paper trade: matching engine not initialized for symbol %s", symbol)
	}
	return matching.OpenOrders(), nil
}

// --- ExchangeOrderQueryService ---

func (e *PaperTradeExchange) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	oid, _ := strconv.ParseUint(q.OrderID, 10, 64)
	e.mu.Lock()
	for _, matching := range e.matchingBooks {
		matching.mu.Lock()
		for _, o := range matching.bidOrders {
			if o.OrderID == oid {
				matching.mu.Unlock()
				e.mu.Unlock()
				return &o, nil
			}
		}
		for _, o := range matching.askOrders {
			if o.OrderID == oid {
				matching.mu.Unlock()
				e.mu.Unlock()
				return &o, nil
			}
		}
		matching.mu.Unlock()
	}
	e.mu.Unlock()
	return nil, fmt.Errorf("paper trade: order not found: %s", q.OrderID)
}

func (e *PaperTradeExchange) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	if e.db == nil || len(q.OrderID) == 0 {
		return nil, nil
	}
	tableName := e.tableName("trades")
	var query string
	var args []interface{}
	if e.db.DriverName() == "postgres" {
		query = `SELECT CAST(id AS BIGINT) as id, CAST(order_id AS BIGINT) as order_id, exchange, symbol, price, quantity, quote_quantity, side, is_buyer, is_maker, fee, fee_currency, strategy_instance_id, traded_at FROM "` + tableName + `" WHERE user_id = $1`
		args = append(args, e.userID)
		if e.strategyInstance != "" {
			query += " AND strategy_instance_id = $2"
			args = append(args, e.strategyInstance)
		}
	} else {
		query = "SELECT * FROM " + tableName + " WHERE 1=1"
	}
	// Use IN (...) so multiple order IDs are OR-matched. The previous loop
	// chained `AND order_id = ?` per ID, which is unsatisfiable for >1 ID.
	placeholders := make([]string, len(q.OrderID))
	for i, oid := range q.OrderID {
		if e.db.DriverName() == "postgres" {
			placeholders[i] = "$" + strconv.Itoa(len(args)+1)
		} else {
			placeholders[i] = "?"
		}
		args = append(args, oid)
	}
	query += " AND order_id IN (" + strings.Join(placeholders, ", ") + ")"
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trades []types.Trade
	for rows.Next() {
		var t types.Trade
		if err := rows.StructScan(&t); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, nil
}

// --- ExchangeTradeHistoryService ---

func (e *PaperTradeExchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	if e.db == nil {
		return nil, nil
	}
	tableName := e.tableName("trades")
	var query string
	var args []interface{}
	if e.db.DriverName() == "postgres" {
		query = `SELECT CAST(id AS BIGINT) as id, CAST(order_id AS BIGINT) as order_id, exchange, symbol, price, quantity, quote_quantity, side, is_buyer, is_maker, fee, fee_currency, strategy_instance_id, traded_at FROM "` + tableName + `" WHERE user_id = $1 AND symbol = $2`
		args = append(args, e.userID, symbol)
		if e.strategyInstance != "" {
			query += " AND strategy_instance_id = $3"
			args = append(args, e.strategyInstance)
		}
	} else {
		query = "SELECT * FROM " + tableName + " WHERE symbol = ?"
		args = append(args, symbol)
	}
	if options != nil && options.StartTime != nil {
		if e.db.DriverName() == "postgres" {
			nextIdx := len(args) + 1
			query += " AND traded_at >= $" + strconv.Itoa(nextIdx)
		} else {
			query += " AND traded_at >= ?"
		}
		args = append(args, *options.StartTime)
	}
	query += " ORDER BY traded_at ASC"
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trades []types.Trade
	for rows.Next() {
		var t types.Trade
		if err := rows.StructScan(&t); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, nil
}

func (e *PaperTradeExchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error) {
	if e.db == nil {
		return nil, nil
	}
	tableName := e.tableName("orders")
	var query string
	var args []interface{}
	if e.db.DriverName() == "postgres" {
		query = `SELECT exchange, CAST(order_id AS BIGINT) as order_id, client_order_id, order_type, status, symbol, price, stop_price, quantity, executed_quantity, side, is_working, time_in_force, created_at, updated_at, is_margin, is_futures, is_isolated, order_uuid as uuid, actual_order_id, strategy_instance_id FROM "` + tableName + `" WHERE user_id = $1 AND status IN ('FILLED', 'CANCELED', 'EXPIRED', 'REJECTED')`
		args = append(args, e.userID)
		if e.strategyInstance != "" {
			query += " AND strategy_instance_id = $2"
			args = append(args, e.strategyInstance)
		}
	} else {
		query = "SELECT * FROM " + tableName + " WHERE status IN ('FILLED', 'CANCELED', 'EXPIRED', 'REJECTED')"
	}
	if symbol != "" {
		if e.db.DriverName() == "postgres" {
			nextIdx := len(args) + 1
			query += " AND symbol = $" + strconv.Itoa(nextIdx)
		} else {
			query += " AND symbol = ?"
		}
		args = append(args, symbol)
	}
	if lastOrderID > 0 {
		if e.db.DriverName() == "postgres" {
			nextIdx := len(args) + 1
			query += " AND order_id > $" + strconv.Itoa(nextIdx)
		} else {
			query += " AND order_id > ?"
		}
		args = append(args, lastOrderID)
	}
	query += " ORDER BY order_id ASC"
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []types.Order
	for rows.Next() {
		var o types.Order
		if err := rows.StructScan(&o); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// --- DefaultFeeRates ---

func (e *PaperTradeExchange) DefaultFeeRates() types.ExchangeFee {
	return types.ExchangeFee{
		MakerFeeRate: fixedpoint.MustNewFromString("0.001"),
		TakerFeeRate: fixedpoint.MustNewFromString("0.001"),
	}
}
