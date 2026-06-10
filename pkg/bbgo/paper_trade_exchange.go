package bbgo

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
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
		fills = append(fills, m.buildFillLocked(order, fillPrice))
		log.Infof("[papertrade] STOP TRIGGERED: order=%d %s %s stop=%s fill=%s qty=%s",
			order.OrderID, order.Side, order.Symbol, stopPrice.String(), fillPrice.String(), order.Quantity.String())
	}

	m.stopOrders = remaining
	return fills
}

func (m *paperMatchingBook) buildFillLocked(order types.Order, fillPrice fixedpoint.Value) paperFill {
	now := types.Time(m.currentTime)
	if time.Time(now).IsZero() {
		now = types.Time(time.Now())
	}

	feeRate := fixedpoint.NewFromFloat(paperTradeFeeRate)
	quoteQty := order.Quantity.Mul(fillPrice)
	fee := quoteQty.Mul(feeRate)

	isFutures := false
	isMargin := false
	if m.Parent != nil {
		isFutures = m.Parent.futuresSettings.IsFutures
		isMargin = m.Parent.marginSettings.IsMargin
	}

	trade := types.Trade{
		ID:                nextPaperTradeID(),
		OrderID:           order.OrderID,
		Exchange:          order.Exchange,
		Symbol:            order.Symbol,
		Side:              order.Side,
		Price:             fillPrice,
		Quantity:          order.Quantity,
		QuoteQuantity:     quoteQty,
		IsBuyer:           order.Side == types.SideTypeBuy,
		IsMaker:           true,
		Fee:               fee,
		FeeCurrency:       m.Market.QuoteCurrency,
		StrategyInstanceID: order.StrategyInstanceID,
		Time:              now,
		IsFutures:         isFutures,
		IsMargin:          isMargin,
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
		m.Parent.updateFuturesPositionLocked(order.Symbol, order.Side, fillPrice, order.Quantity, order.StrategyInstanceID)
		m.Parent.mu.Unlock()
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

	isStop := order.Type == types.OrderTypeStopLimit || order.Type == types.OrderTypeStopMarket ||
		order.Type == types.OrderTypeTakeProfit || order.Type == types.OrderTypeTakeProfitMarket

	if isStop {
		var remaining []types.Order
		for _, o := range m.stopOrders {
			if o.OrderID == order.OrderID {
				// Unlock the margin that was locked when the stop was placed
				if m.Parent != nil && (m.Parent.futuresSettings.IsFutures || m.Parent.marginSettings.IsMargin) {
					leverage := m.Parent.effectiveLeverage(order.Symbol)
					lockAmt := order.Quantity.Mul(order.StopPrice).Div(leverage)
					m.Account.UnlockBalance(m.Market.QuoteCurrency, lockAmt)
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
	db               *sqlx.DB // nil when not in DB mode
	tablePrefix      string
	userID           string

	// Futures state per symbol
	futuresSettings types.FuturesSettings
	futuresStates   map[string]*paperFuturesState

	// Margin state per asset
	marginSettings types.MarginSettings
	marginStates   map[string]*paperMarginState
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
		inner:          inner,
		markets:        markets,
		account:        account,
		matchingBooks:  make(map[string]*paperMatchingBook),
		futuresStates:  make(map[string]*paperFuturesState),
		marginStates:   make(map[string]*paperMarginState),
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
}

// EmitBalanceUpdateFromAccount emits a balance update using the current account state.
func (e *PaperTradeExchange) EmitBalanceUpdateFromAccount() {
	if e.userDataEmitter != nil {
		e.userDataEmitter.EmitBalanceUpdate(e.account.Balances())
	}
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

	for {
		current := atomic.LoadUint64(&paperOrderID)
		if maxOrderID <= current || atomic.CompareAndSwapUint64(&paperOrderID, current, maxOrderID) {
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
	} else {
		query = "SELECT * FROM " + tableName + " WHERE status IN ('NEW', 'PARTIALLY_FILLED')"
	}
	if symbol != "" {
		if e.db.DriverName() == "postgres" {
			query += " AND symbol = $2"
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
		query = "SELECT currency, available, locked FROM \"" + tableName + "\" WHERE user_id = $1"
		args = append(args, e.userID)
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
		// Get the latest row per symbol where position_amount != '0'
		query = `SELECT DISTINCT ON (symbol) symbol, position_side, leverage, entry_price, position_amount, margin_asset, strategy_instance_id
			FROM "` + tableName + `"
			WHERE user_id = $1 AND position_amount != '0'
			ORDER BY symbol, updated_at DESC`
		args = append(args, e.userID)
	} else {
		query = `SELECT symbol, position_side, leverage, entry_price, position_amount, margin_asset, strategy_instance_id
			FROM ` + tableName + `
			WHERE position_amount != 0
			GROUP BY symbol
			HAVING gid = MAX(gid)`
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
			Symbol             string          `db:"symbol"`
			PositionSide       string          `db:"position_side"`
			Leverage           fixedpoint.Value `db:"leverage"`
			EntryPrice         fixedpoint.Value `db:"entry_price"`
			PositionAmount     fixedpoint.Value `db:"position_amount"`
			MarginAsset        string          `db:"margin_asset"`
			StrategyInstanceID string          `db:"strategy_instance_id"`
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

		state := &paperFuturesState{
			Leverage:           lev,
			PositionAmount:     r.PositionAmount,
			EntryPrice:         r.EntryPrice,
			PositionSide:       types.PositionType(r.PositionSide),
			MarginAsset:        r.MarginAsset,
			StrategyInstanceID: r.StrategyInstanceID,
		}
		e.futuresStates[r.Symbol] = state
		restored++
	}

	if restored > 0 {
		log.Infof("paper trade: restored %d futures positions from DB", restored)
	}
	return rows.Err()
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
		if b.Total().IsZero() && b.Available.IsZero() && b.Locked.IsZero() {
			continue
		}
		var sql string
		switch e.db.DriverName() {
		case "postgres":
			sql = `INSERT INTO "` + tableName + `" (user_id, currency, total, available, locked) VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (user_id, currency) DO UPDATE SET total = $3, available = $4, locked = $5`
			_, err := e.db.Exec(sql, e.userID, currency, b.Total().String(), b.Available.String(), b.Locked.String())
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
				if e.db != nil {
					m.OnBalanceUpdate(func(types.BalanceMap) { e.syncBalances() })
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
	order.StrategyInstanceID = submit.StrategyInstanceID
	order.IsFutures = e.futuresSettings.IsFutures
	order.IsMargin = e.marginSettings.IsMargin
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
				_ = e.account.LockBalance(market.BaseCurrency, submit.Quantity)
			} else if lockAmt.Sign() > 0 {
				_ = e.account.LockBalance(market.QuoteCurrency, lockAmt)
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
