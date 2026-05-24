package bbgo

import (
	"context"

	"github.com/c9s/bbgo/pkg/cache"
	"github.com/c9s/bbgo/pkg/envvar"
	"github.com/c9s/bbgo/pkg/types"
)

// MarketDataSource abstracts where a session gets its market data.
// DirectExchangeSource preserves the original bbgo behaviour (zero change).
// SharedServiceSource proxies everything through a central gRPC data service.
type MarketDataSource interface {
	LoadMarkets(ctx context.Context, session *ExchangeSession) (types.MarketMap, error)
	NewMarketDataStream(session *ExchangeSession) types.Stream
}

// DirectExchangeSource wraps the original direct-to-exchange behaviour.
// When MARKET_DATA_SERVICE_URL is not set this is the active source,
// guaranteeing identical behaviour to pre-modification bbgo.
type DirectExchangeSource struct{}

func (s *DirectExchangeSource) LoadMarkets(ctx context.Context, session *ExchangeSession) (types.MarketMap, error) {
	var disableMarketsCache bool
	if envvar.SetBool("DISABLE_MARKETS_CACHE", &disableMarketsCache); disableMarketsCache {
		return session.Exchange.QueryMarkets(ctx)
	}
	return cache.LoadExchangeMarketsWithCache(ctx, session.Exchange)
}

func (s *DirectExchangeSource) NewMarketDataStream(session *ExchangeSession) types.Stream {
	stream := session.Exchange.NewStream()
	stream.SetPublicOnly()
	return stream
}
