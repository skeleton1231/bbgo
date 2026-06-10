package bbgo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/c9s/bbgo/pkg/cache"
	"github.com/c9s/bbgo/pkg/types"
)

// SharedServiceSource loads market data from a centralised gRPC service.
// Falls back to direct exchange access when the shared service is unavailable.
type SharedServiceSource struct {
	grpcAddr string
	conn     *grpc.ClientConn
}

func NewSharedServiceSource(addr string) (*SharedServiceSource, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.Wrapf(err, "dial shared data service %s", addr)
	}
	log.Infof("shared data service connected: %s", addr)
	return &SharedServiceSource{grpcAddr: addr, conn: conn}, nil
}

func (s *SharedServiceSource) Conn() *grpc.ClientConn {
	return s.conn
}

func (s *SharedServiceSource) LoadMarkets(ctx context.Context, session *ExchangeSession) (types.MarketMap, error) {
	if markets, err := s.loadMarketsFromSharedFile(session.ExchangeName); err == nil && len(markets) > 0 {
		log.Infof("loaded %d markets from shared cache file for %s", len(markets), session.ExchangeName)
		return markets, nil
	}

	log.Warnf("shared cache file unavailable for %s, falling back to direct query", session.ExchangeName)
	return cache.LoadExchangeMarketsWithCache(ctx, session.Exchange)
}

func (s *SharedServiceSource) NewMarketDataStream(session *ExchangeSession) types.Stream {
	return NewGRPCStream(s.conn, string(session.ExchangeName), s.grpcAddr)
}

func (s *SharedServiceSource) loadMarketsFromSharedFile(exchangeName types.ExchangeName) (types.MarketMap, error) {
	cacheDir := cache.CacheDir()
	cacheFile := filepath.Join(cacheDir, string(exchangeName)+"-markets.json")

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}

	var markets types.MarketMap
	if err := json.Unmarshal(data, &markets); err != nil {
		return nil, errors.Wrap(err, "unmarshal shared markets cache")
	}

	return markets, nil
}
