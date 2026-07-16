package bbgo

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
	"google.golang.org/grpc"
)

type grpcExchangeProxy struct {
	types.Exchange
	exchangeName string
	queryClient  *pb.MarketDataQueryClient
	riskService  types.ExchangeRiskService
	// tradeHistory holds ExchangeTradeHistoryService from the wrapped exchange
	// when present. grpcExchangeProxy embeds the types.Exchange *interface*, so
	// Go only promotes that interface's methods — QueryTrades/QueryClosedOrders
	// (from the separate ExchangeTradeHistoryService) would otherwise be hidden,
	// breaking strategies like grid2 that recover state from trade history.
	tradeHistory types.ExchangeTradeHistoryService
}

func newGRPCExchangeProxy(real types.Exchange, conn *grpc.ClientConn, exchangeName string) *grpcExchangeProxy {
	p := &grpcExchangeProxy{
		Exchange:     real,
		exchangeName: exchangeName,
		queryClient:  pb.NewMarketDataQueryClient(conn),
	}
	if rs, ok := real.(types.ExchangeRiskService); ok {
		p.riskService = rs
	}
	if th, ok := real.(types.ExchangeTradeHistoryService); ok {
		p.tradeHistory = th
	}
	return p
}

// QueryTrades delegates ExchangeTradeHistoryService to the real exchange.
// Trade history is account data, not market data, so it is served directly
// (klines/tickers still route through gRPC).
func (p *grpcExchangeProxy) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	if p.tradeHistory == nil {
		return nil, fmt.Errorf("exchange %s does not implement ExchangeTradeHistoryService", p.exchangeName)
	}
	return p.tradeHistory.QueryTrades(ctx, symbol, options)
}

// QueryClosedOrders delegates ExchangeTradeHistoryService to the real exchange.
func (p *grpcExchangeProxy) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error) {
	if p.tradeHistory == nil {
		return nil, fmt.Errorf("exchange %s does not implement ExchangeTradeHistoryService", p.exchangeName)
	}
	return p.tradeHistory.QueryClosedOrders(ctx, symbol, since, until, lastOrderID)
}

func (p *grpcExchangeProxy) QueryPositionRisk(ctx context.Context, symbol ...string) ([]types.PositionRisk, error) {
	if p.riskService == nil {
		return nil, fmt.Errorf("exchange %s does not implement ExchangeRiskService", p.exchangeName)
	}
	return p.riskService.QueryPositionRisk(ctx, symbol...)
}

func (p *grpcExchangeProxy) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if p.riskService == nil {
		return fmt.Errorf("exchange %s does not implement ExchangeRiskService", p.exchangeName)
	}
	return p.riskService.SetLeverage(ctx, symbol, leverage)
}

func (p *grpcExchangeProxy) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	req := &pb.QueryKLinesRequest{
		Exchange: p.exchangeName,
		Symbol:   symbol,
		Interval: string(interval),
	}
	if options.Limit > 0 {
		req.Limit = int64(options.Limit)
	}
	if options.StartTime != nil {
		req.StartTime = options.StartTime.Unix()
	}
	if options.EndTime != nil {
		req.EndTime = options.EndTime.Unix()
	}

	resp, err := p.queryClient.QueryKLines(ctx, req)
	if err != nil {
		log.WithError(err).Warn("grpc proxy QueryKLines failed, falling back to direct exchange")
		return p.Exchange.QueryKLines(ctx, symbol, interval, options)
	}
	if resp.Error != nil {
		log.Warnf("grpc proxy QueryKLines returned error: %s, falling back to direct exchange", resp.Error.ErrorMessage)
		return p.Exchange.QueryKLines(ctx, symbol, interval, options)
	}

	klines := make([]types.KLine, 0, len(resp.Klines))
	for _, k := range resp.Klines {
		klines = append(klines, pb.PbKLineToTypes(k))
	}
	return klines, nil
}

func (p *grpcExchangeProxy) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	resp, err := p.queryClient.QueryTicker(ctx, &pb.QueryTickerRequest{
		Exchange: p.exchangeName,
		Symbol:   symbol,
	})
	if err != nil {
		log.WithError(err).Warn("grpc proxy QueryTicker failed, falling back to direct exchange")
		return p.Exchange.QueryTicker(ctx, symbol)
	}
	if resp.Error != nil {
		log.Warnf("grpc proxy QueryTicker returned error: %s, falling back to direct exchange", resp.Error.ErrorMessage)
		return p.Exchange.QueryTicker(ctx, symbol)
	}
	return pbTickerToTypes(resp.Ticker), nil
}

func (p *grpcExchangeProxy) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	resp, err := p.queryClient.QueryTickers(ctx, &pb.QueryTickersRequest{
		Exchange: p.exchangeName,
		Symbols:  symbol,
	})
	if err != nil {
		log.WithError(err).Warn("grpc proxy QueryTickers failed, falling back to direct exchange")
		return p.Exchange.QueryTickers(ctx, symbol...)
	}
	if resp.Error != nil {
		log.Warnf("grpc proxy QueryTickers returned error: %s, falling back to direct exchange", resp.Error.ErrorMessage)
		return p.Exchange.QueryTickers(ctx, symbol...)
	}

	tickers := make(map[string]types.Ticker, len(resp.Tickers))
	for _, t := range resp.Tickers {
		tickers[t.Symbol] = *pbTickerToTypes(t)
	}
	return tickers, nil
}

func pbTickerToTypes(t *pb.Ticker) *types.Ticker {
	if t == nil {
		return nil
	}
	last := fixedpoint.NewFromFloat(t.Close)
	return &types.Ticker{
		Open:   fixedpoint.NewFromFloat(t.Open),
		High:   fixedpoint.NewFromFloat(t.High),
		Low:    fixedpoint.NewFromFloat(t.Low),
		Last:   last,
		Buy:    last,
		Sell:   last,
		Volume: fixedpoint.NewFromFloat(t.Volume),
	}
}

// compile-time interface checks
var _ types.Exchange = (*grpcExchangeProxy)(nil)
var _ types.ExchangeRiskService = (*grpcExchangeProxy)(nil)
var _ types.ExchangeTradeHistoryService = (*grpcExchangeProxy)(nil)
