package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
	"google.golang.org/grpc"
)

type grpcExchangeProxy struct {
	types.Exchange
	exchangeName string
	queryClient  *pb.MarketDataQueryClient
}

func newGRPCExchangeProxy(real types.Exchange, conn *grpc.ClientConn, exchangeName string) *grpcExchangeProxy {
	return &grpcExchangeProxy{
		Exchange:     real,
		exchangeName: exchangeName,
		queryClient:  pb.NewMarketDataQueryClient(conn),
	}
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
		return p.Exchange.QueryKLines(ctx, symbol, interval, options)
	}
	if resp.Error != nil {
		return p.Exchange.QueryKLines(ctx, symbol, interval, options)
	}

	klines := make([]types.KLine, 0, len(resp.Klines))
	for _, k := range resp.Klines {
		klines = append(klines, pbKLineToTypes(k))
	}
	return klines, nil
}

func (p *grpcExchangeProxy) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	resp, err := p.queryClient.QueryTicker(ctx, &pb.QueryTickerRequest{
		Exchange: p.exchangeName,
		Symbol:   symbol,
	})
	if err != nil {
		return p.Exchange.QueryTicker(ctx, symbol)
	}
	if resp.Error != nil {
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
		return p.Exchange.QueryTickers(ctx, symbol...)
	}
	if resp.Error != nil {
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
	return &types.Ticker{
		Open:   fixedpoint.NewFromFloat(t.Open),
		High:   fixedpoint.NewFromFloat(t.High),
		Low:    fixedpoint.NewFromFloat(t.Low),
		Last:   fixedpoint.NewFromFloat(t.Close),
		Volume: fixedpoint.NewFromFloat(t.Volume),
	}
}
// compile-time interface check
var _ types.Exchange = (*grpcExchangeProxy)(nil)
