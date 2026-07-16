package bbgo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/types"
)

// TestGRPCExchangeProxy_DelegatesTradeHistory is the regression test for grid2
// (and any strategy using ExchangeTradeHistoryService for recovery) in SaaS
// paper mode. grpcExchangeProxy embeds the types.Exchange *interface*, and Go
// only promotes that interface's methods — so even though the underlying
// PaperTradeExchange implements QueryTrades/QueryClosedOrders, the proxy did
// not, and grid2's recover() logged "ExchangeTradeHistoryService is not
// implemented, can not recover grid" every 30 minutes.
//
// The fix delegates ExchangeTradeHistoryService to the real exchange, mirroring
// the existing ExchangeRiskService delegation. Trade history is not market
// data, so it is served directly (klines/tickers still go through gRPC).
func TestGRPCExchangeProxy_DelegatesTradeHistory(t *testing.T) {
	real := newTestPaperTradeExchange() // implements ExchangeTradeHistoryService
	proxy := newGRPCExchangeProxy(real, nil, "binance")

	th, ok := interface{}(proxy).(types.ExchangeTradeHistoryService)
	require.True(t, ok, "grpc proxy must expose ExchangeTradeHistoryService so grid2 recovery works in paper mode")

	// Delegation reaches the real exchange; PaperTradeExchange's nil-db guard
	// returns (nil, nil), proving the call was forwarded (not blocked at the proxy).
	trades, err := th.QueryTrades(context.Background(), "BTCUSDT", nil)
	assert.NoError(t, err)
	assert.Nil(t, trades)

	orders, err := th.QueryClosedOrders(context.Background(), "BTCUSDT", time.Unix(0, 0), time.Now(), 0)
	assert.NoError(t, err)
	assert.Nil(t, orders)
}

// TestGRPCExchangeProxy_TradeHistoryNilWhenRealLacksIt ensures the proxy does
// not falsely claim ExchangeTradeHistoryService when the wrapped exchange does
// not implement it — the delegation must be conditional, matching the
// ExchangeRiskService pattern, so live exchanges without trade history still
// report "not implemented" honestly to strategies.
func TestGRPCExchangeProxy_TradeHistoryNilWhenRealLacksIt(t *testing.T) {
	// newTestPaperTradeExchange always implements ExchangeTradeHistoryService,
	// so build a proxy that wraps a non-history exchange by zeroing the field.
	real := newTestPaperTradeExchange()
	proxy := newGRPCExchangeProxy(real, nil, "binance")
	require.NotNil(t, proxy.tradeHistory, "sanity: paper exchange provides trade history")

	// Force the "real does not implement it" branch and confirm the method
	// returns a clear error instead of panicking.
	proxy.tradeHistory = nil
	_, err := proxy.QueryTrades(context.Background(), "BTCUSDT", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ExchangeTradeHistoryService")
}
