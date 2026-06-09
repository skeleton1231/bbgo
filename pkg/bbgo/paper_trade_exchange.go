package bbgo

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

var paperOrderID uint64
var paperTradeID uint64

func init() {
	// Use time-based offset to avoid UNIQUE constraint collisions across restarts
	paperOrderID = uint64(time.Now().UnixNano()) / 1e6
	paperTradeID = paperOrderID
}

func nextPaperOrderID() uint64 {
	return atomic.AddUint64(&paperOrderID, 1)
}

func nextPaperTradeID() uint64 {
	return atomic.AddUint64(&paperTradeID, 1)
}

const paperTradeFeeRate = 0.001

// isPaperLimitTaker checks if a limit order would immediately match on a real exchange.
// Buy limit at or above market price, or sell limit at or below market price → taker.
func isPaperLimitTaker(o types.SubmitOrder, currentPrice fixedpoint.Value) bool {
	if currentPrice.IsZero() || o.Type != types.OrderTypeLimit {
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

	mu          sync.Mutex
	bidOrders   []types.Order
	askOrders   []types.Order
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
	orders := make([]types.Order, 0, len(m.bidOrders)+len(m.askOrders))
	orders = append(orders, m.bidOrders...)
	orders = append(orders, m.askOrders...)
	return orders
}

// ProcessKLine processes a closed kline and fills matching orders.
func (m *paperMatchingBook) ProcessKLine(kline types.KLine) {
	m.currentTime = kline.EndTime.Time()

	var fills []paperFill

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

	m.emitFills(fills)
}

// paperFill records a fill to be emitted after the lock is released.
// This prevents deadlock when callbacks trigger new order submissions.
type paperFill struct {
	Trade    types.Trade
	Order    types.Order
	Balances types.BalanceMap
}

// buyToPrice simulates price going up — limit sell orders get filled.
func (m *paperMatchingBook) buyToPrice(price fixedpoint.Value) []paperFill {
	m.mu.Lock()
	defer m.mu.Unlock()

	var fills []paperFill
	var remainingAsk []types.Order
	for _, o := range m.askOrders {
		if o.Type == types.OrderTypeLimit && price.Compare(o.Price) >= 0 {
			fillPrice := o.Price
			if o.Price.Compare(m.lastPrice) < 0 {
				fillPrice = m.lastPrice
			}
			fills = append(fills, m.buildFillLocked(o, fillPrice))
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
		if o.Type == types.OrderTypeLimit && price.Compare(o.Price) <= 0 {
			fillPrice := o.Price
			if o.Price.Compare(m.lastPrice) > 0 {
				fillPrice = m.lastPrice
			}
			fills = append(fills, m.buildFillLocked(o, fillPrice))
		} else {
			remainingBid = append(remainingBid, o)
		}
	}
	m.bidOrders = remainingBid
	m.lastPrice = price
	return fills
}

// emitFills emits all fill callbacks outside the lock.
func (m *paperMatchingBook) emitFills(fills []paperFill) {
	for _, f := range fills {
		m.EmitTradeUpdate(f.Trade)
		m.EmitOrderUpdate(f.Order)
		m.EmitBalanceUpdate(f.Balances)
		log.Infof("[papertrade] FILLED: order=%d %s %s @ %s qty=%s",
			f.Order.OrderID, f.Order.Side, f.Order.Symbol, f.Order.Price.String(), f.Order.Quantity.String())
	}
}

func (m *paperMatchingBook) buildFillLocked(order types.Order, fillPrice fixedpoint.Value) paperFill {
	now := types.Time(m.currentTime)
	if time.Time(now).IsZero() {
		now = types.Time(time.Now())
	}

	feeRate := fixedpoint.NewFromFloat(paperTradeFeeRate)
	quoteQty := order.Quantity.Mul(fillPrice)
	fee := quoteQty.Mul(feeRate)

	trade := types.Trade{
		ID:            nextPaperTradeID(),
		OrderID:       order.OrderID,
		Exchange:      order.Exchange,
		Symbol:        order.Symbol,
		Side:          order.Side,
		Price:         fillPrice,
		Quantity:      order.Quantity,
		QuoteQuantity: quoteQty,
		IsBuyer:       order.Side == types.SideTypeBuy,
		IsMaker:       true,
		Fee:           fee,
		FeeCurrency:   m.Market.QuoteCurrency,
		Time:          now,
	}

	switch order.Side {
	case types.SideTypeBuy:
		m.Account.UseLockedBalance(m.Market.QuoteCurrency, quoteQty)
		m.Account.AddBalance(m.Market.BaseCurrency, order.Quantity)
		m.Account.AddBalance(m.Market.QuoteCurrency, fee.Neg())

	case types.SideTypeSell:
		m.Account.UseLockedBalance(m.Market.BaseCurrency, order.Quantity)
		m.Account.AddBalance(m.Market.QuoteCurrency, quoteQty.Sub(fee))
	}

	filled := order
	filled.Status = types.OrderStatusFilled
	filled.ExecutedQuantity = order.Quantity
	filled.AveragePrice = fillPrice
	filled.IsWorking = false
	filled.UpdateTime = now

	return paperFill{
		Trade:    trade,
		Order:    filled,
		Balances: m.Account.Balances(),
	}
}

// CancelOrder removes an order from the book and unlocks balance.
func (m *paperMatchingBook) CancelOrder(order types.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch order.Side {
	case types.SideTypeBuy:
		var remaining []types.Order
		for _, o := range m.bidOrders {
			if o.OrderID == order.OrderID {
				m.Account.UnlockBalance(m.Market.QuoteCurrency, o.Price.Mul(o.Quantity))
				continue
			}
			remaining = append(remaining, o)
		}
		m.bidOrders = remaining

	case types.SideTypeSell:
		var remaining []types.Order
		for _, o := range m.askOrders {
			if o.OrderID == order.OrderID {
				m.Account.UnlockBalance(m.Market.BaseCurrency, o.Quantity)
				continue
			}
			remaining = append(remaining, o)
		}
		m.askOrders = remaining
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

	matchingBooks    map[string]*paperMatchingBook
	mu               sync.Mutex
	userDataEmitter  types.StandardStreamEmitter
	supabaseService  *service.SupabaseService // nil when not in Supabase mode
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
		inner:         inner,
		markets:       markets,
		account:       account,
		matchingBooks: make(map[string]*paperMatchingBook),
	}

	for symbol, market := range markets {
		e.matchingBooks[symbol] = &paperMatchingBook{
			Symbol:  symbol,
			Market:  market,
			Account: account,
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
// When Supabase is configured, adds a balance sync callback to persist every change.
func (e *PaperTradeExchange) BindUserData(userDataStream types.StandardStreamEmitter) {
	e.mu.Lock()
	e.userDataEmitter = userDataStream
	for _, matching := range e.matchingBooks {
		matching.OnTradeUpdate(userDataStream.EmitTradeUpdate)
		matching.OnOrderUpdate(userDataStream.EmitOrderUpdate)
		matching.OnBalanceUpdate(userDataStream.EmitBalanceUpdate)
		// Persist balance changes to Supabase on every update
		if e.supabaseService != nil {
			matching.OnBalanceUpdate(func(types.BalanceMap) { e.syncBalancesToSupabase() })
		}
	}
	e.mu.Unlock()
}

// SetSupabaseService enables Supabase persistence for balance sync.
// Must be called before BindUserData if Supabase is available.
func (e *PaperTradeExchange) SetSupabaseService(svc *service.SupabaseService) {
	e.supabaseService = svc
}

// RestoreFromSupabase loads open orders and balances from Supabase into
// the in-memory paper trade engine. Must be called before strategies run
// so that QueryOpenOrders() returns restored orders and strategies skip
// re-placing existing grids.
func (e *PaperTradeExchange) RestoreFromSupabase(ctx context.Context, svc *service.SupabaseService) error {
	if svc == nil {
		return nil
	}
	e.supabaseService = svc

	// 1. Restore balances
	balances, err := svc.QueryBalances()
	if err != nil {
		log.WithError(err).Warn("paper trade: failed to restore balances from Supabase, using config defaults")
	} else if len(balances) > 0 {
		e.account.UpdateBalances(balances)
		log.Infof("paper trade: restored %d balances from Supabase", len(balances))
	}

	// 2. Restore open orders.
	// Orders are added directly to matchingBook bid/ask slices. The book's
	// OnTradeUpdate/OnOrderUpdate/OnBalanceUpdate callbacks are already wired
	// (via matchingBook() lazy creation or BindUserData), so when a restored
	// order is filled by an incoming kline, the book's match() emits correctly.
	orders, err := svc.QueryOpenOrders("")
	if err != nil {
		log.WithError(err).Warn("paper trade: failed to restore open orders from Supabase")
		return nil // non-fatal: strategy will re-place orders
	}
	if len(orders) == 0 {
		log.Info("paper trade: no open orders to restore from Supabase")
		return nil
	}

	var maxOrderID uint64
	for _, order := range orders {
		matching, ok := e.matchingBook(order.Symbol)
		if !ok {
			continue
		}
		matching.mu.Lock()
		switch order.Side {
		case types.SideTypeBuy:
			matching.bidOrders = append(matching.bidOrders, order)
		case types.SideTypeSell:
			matching.askOrders = append(matching.askOrders, order)
		}
		matching.mu.Unlock()

		if order.OrderID > maxOrderID {
			maxOrderID = order.OrderID
		}
	}

	// Ensure new order IDs won't collide with restored ones
	for {
		current := atomic.LoadUint64(&paperOrderID)
		if maxOrderID <= current || atomic.CompareAndSwapUint64(&paperOrderID, current, maxOrderID) {
			break
		}
	}

	log.Infof("paper trade: restored %d open orders from Supabase (max order ID: %d)", len(orders), maxOrderID)
	return nil
}

// syncBalancesToSupabase writes current balances to Supabase.
// Called after every balance-changing operation (order placement, fill, cancel).
func (e *PaperTradeExchange) syncBalancesToSupabase() {
	if e.supabaseService == nil {
		return
	}
	if err := e.supabaseService.UpsertBalances(e.account.Balances()); err != nil {
		log.WithError(err).Warn("paper trade: failed to sync balances to Supabase")
	}
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
			}
			if e.userDataEmitter != nil {
				m.OnTradeUpdate(e.userDataEmitter.EmitTradeUpdate)
				m.OnOrderUpdate(e.userDataEmitter.EmitOrderUpdate)
				m.OnBalanceUpdate(e.userDataEmitter.EmitBalanceUpdate)
				if e.supabaseService != nil {
					m.OnBalanceUpdate(func(types.BalanceMap) { e.syncBalancesToSupabase() })
				}
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

	orderID := nextPaperOrderID()
	now := types.Time(time.Now())

	price := submit.Price
	if submit.Type == types.OrderTypeMarket {
		price = market.TruncatePrice(matching.LastPrice())
		if price.IsZero() {
			return nil, fmt.Errorf("paper trade: cannot place market order before receiving market data for %s", submit.Symbol)
		}
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
	if submit.Type == types.OrderTypeMarket {
		order.Price = price
	}

	// Lock balance
	switch submit.Side {
	case types.SideTypeBuy:
		if err := e.account.LockBalance(market.QuoteCurrency, submit.Quantity.Mul(price)); err != nil {
			return nil, fmt.Errorf("paper trade: %w", err)
		}
	case types.SideTypeSell:
		if err := e.account.LockBalance(market.BaseCurrency, submit.Quantity); err != nil {
			return nil, fmt.Errorf("paper trade: %w", err)
		}
	}

	matching.EmitBalanceUpdate(e.account.Balances())
	matching.EmitOrderUpdate(order)

	// Market orders and taker limit orders fill immediately at market price
	isTaker := submit.Type == types.OrderTypeMarket || isPaperLimitTaker(submit, matching.LastPrice())
	if isTaker {
		fillPrice := price
		if submit.Type == types.OrderTypeLimit && !matching.LastPrice().IsZero() {
			fillPrice = market.TruncatePrice(matching.LastPrice())
		}

		matching.mu.Lock()
		fill := matching.buildFillLocked(order, fillPrice)
		matching.mu.Unlock()

		// For taker limit orders, refund the difference between locked and used
		if submit.Type == types.OrderTypeLimit {
			switch submit.Side {
			case types.SideTypeBuy:
				refund := price.Sub(fillPrice).Mul(submit.Quantity)
				if refund.Sign() > 0 {
					e.account.UnlockBalance(market.QuoteCurrency, refund)
				}
			case types.SideTypeSell:
				refund := fillPrice.Sub(price).Mul(submit.Quantity)
				if refund.Sign() > 0 {
					e.account.AddBalance(market.QuoteCurrency, refund)
				}
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
	return nil, nil
}

// --- ExchangeTradeHistoryService ---

func (e *PaperTradeExchange) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	return nil, nil
}

func (e *PaperTradeExchange) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error) {
	return nil, nil
}

// --- DefaultFeeRates ---

func (e *PaperTradeExchange) DefaultFeeRates() types.ExchangeFee {
	return types.ExchangeFee{
		MakerFeeRate: fixedpoint.MustNewFromString("0.001"),
		TakerFeeRate: fixedpoint.MustNewFromString("0.001"),
	}
}
