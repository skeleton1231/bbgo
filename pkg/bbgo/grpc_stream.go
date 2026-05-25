package bbgo

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
)

// GRPCStream implements types.Stream by consuming market data from a
// centralised gRPC MarketDataService instead of connecting directly to
// an exchange WebSocket.  Strategies bind their callbacks exactly as
// they would with StandardStream — they never know the difference.
type GRPCStream struct {
	exchangeName string
	conn         *grpc.ClientConn

	mu            sync.Mutex
	subscriptions []types.Subscription

	ctx    context.Context
	cancel context.CancelFunc

	// callbacks — mirrors StandardStream callback slices
	startCallbacks           []func()
	connectCallbacks         []func()
	disconnectCallbacks      []func()
	authCallbacks            []func()
	kLineCallbacks           []func(kline types.KLine)
	kLineClosedCallbacks     []func(kline types.KLine)
	marketTradeCallbacks     []func(trade types.Trade)
	bookUpdateCallbacks      []func(book types.SliceOrderBook)
	bookSnapshotCallbacks    []func(book types.SliceOrderBook)
	bookTickerCallbacks      []func(ticker types.BookTicker)
	orderUpdateCallbacks     []func(order types.Order)
	tradeUpdateCallbacks     []func(trade types.Trade)
	balanceSnapshotCallbacks []func(balances types.BalanceMap)
	balanceUpdateCallbacks   []func(balances types.BalanceMap)
}

func NewGRPCStream(conn *grpc.ClientConn, exchangeName string) *GRPCStream {
	return &GRPCStream{
		conn:         conn,
		exchangeName: exchangeName,
	}
}

// --- types.Stream interface ---

func (s *GRPCStream) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions = append(s.subscriptions, types.Subscription{
		Channel: channel,
		Symbol:  symbol,
		Options: options,
	})
}

func (s *GRPCStream) GetSubscriptions() []types.Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]types.Subscription, len(s.subscriptions))
	copy(out, s.subscriptions)
	return out
}

func (s *GRPCStream) Resubscribe(fn func(oldSubs []types.Subscription) (newSubs []types.Subscription, err error)) error {
	newSubs, err := fn(s.GetSubscriptions())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.subscriptions = newSubs
	cancelFn := s.cancel
	s.mu.Unlock()
	if cancelFn != nil {
		cancelFn()
	}
	return nil
}

func (s *GRPCStream) SetPublicOnly()      {}
func (s *GRPCStream) GetPublicOnly() bool { return true }

func (s *GRPCStream) Connect(ctx context.Context) error {
	s.mu.Lock()
	subs := make([]types.Subscription, len(s.subscriptions))
	copy(subs, s.subscriptions)
	s.mu.Unlock()

	pbSubs := make([]*pb.Subscription, 0, len(subs))
	for _, sub := range subs {
		pbSubs = append(pbSubs, typesSubToPB(sub, s.exchangeName))
	}

	streamCtx, cancel := context.WithCancel(ctx)
	s.ctx = streamCtx
	s.cancel = cancel

	client := pb.NewMarketDataServiceClient(s.conn)
	req := &pb.SubscribeRequest{Subscriptions: pbSubs}
	stream, err := client.Subscribe(streamCtx, req)
	if err != nil {
		cancel()
		return errors.Wrap(err, "gRPC subscribe")
	}

	s.EmitConnect()
	go s.receiveLoop(stream)
	s.EmitStart()
	return nil
}

func (s *GRPCStream) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *GRPCStream) Reconnect() {
	if s.cancel != nil {
		s.cancel()
	}
}

// --- receive loop ---

func (s *GRPCStream) receiveLoop(stream pb.MarketDataService_SubscribeClient) {
	defer s.EmitDisconnect()

	for {
		data, err := stream.Recv()
		if err != nil {
			log.WithError(err).Warn("gRPC market data stream closed")
			go s.reconnectLoop()
			return
		}
		s.dispatch(data)
	}
}

func (s *GRPCStream) reconnectLoop() {
	for {
		s.mu.Lock()
		parentCtx := s.ctx
		s.mu.Unlock()
		if parentCtx.Err() != nil {
			return
		}
		time.Sleep(3 * time.Second)
		log.Info("attempting gRPC market data reconnect")

		subs := s.GetSubscriptions()
		pbSubs := make([]*pb.Subscription, 0, len(subs))
		for _, sub := range subs {
			pbSubs = append(pbSubs, typesSubToPB(sub, s.exchangeName))
		}

		streamCtx, cancel := context.WithCancel(parentCtx)
		client := pb.NewMarketDataServiceClient(s.conn)
		req := &pb.SubscribeRequest{Subscriptions: pbSubs}
		stream, err := client.Subscribe(streamCtx, req)
		if err != nil {
			cancel()
			log.WithError(err).Warn("gRPC reconnect failed, retrying")
			continue
		}

		s.mu.Lock()
		if s.cancel != nil {
			s.cancel()
		}
		s.ctx = streamCtx
		s.cancel = cancel
		s.mu.Unlock()

		s.EmitConnect()
		go s.receiveLoop(stream)
		return
	}
}

func (s *GRPCStream) dispatch(data *pb.MarketData) {
	switch data.Channel {
	case pb.Channel_KLINE:
		if data.Kline == nil {
			return
		}
		k := pb.PbKLineToTypes(data.Kline)
		s.EmitKLine(k)
		if k.Closed {
			s.EmitKLineClosed(k)
		}

	case pb.Channel_TRADE:
		if len(data.Trades) == 0 {
			return
		}
		t := pb.PbTradeToTypes(data.Trades[0])
		s.EmitMarketTrade(t)

	case pb.Channel_BOOK:
		if data.Depth == nil {
			return
		}
		book := pb.PbDepthToBook(data.Depth)
		if data.Event == pb.Event_SNAPSHOT {
			s.EmitBookSnapshot(book)
		} else {
			s.EmitBookUpdate(book)
		}
	}
}

// --- callback registration (mirrors StandardStream) ---

func (s *GRPCStream) OnStart(cb func())      { s.startCallbacks = append(s.startCallbacks, cb) }
func (s *GRPCStream) EmitStart()             { for _, cb := range s.startCallbacks { cb() } }

func (s *GRPCStream) OnConnect(cb func())    { s.connectCallbacks = append(s.connectCallbacks, cb) }
func (s *GRPCStream) EmitConnect()           { for _, cb := range s.connectCallbacks { cb() } }

func (s *GRPCStream) OnDisconnect(cb func()) { s.disconnectCallbacks = append(s.disconnectCallbacks, cb) }
func (s *GRPCStream) EmitDisconnect()        { for _, cb := range s.disconnectCallbacks { cb() } }

func (s *GRPCStream) OnAuth(cb func()) { s.authCallbacks = append(s.authCallbacks, cb) }

func (s *GRPCStream) OnKLine(cb func(types.KLine))       { s.kLineCallbacks = append(s.kLineCallbacks, cb) }
func (s *GRPCStream) EmitKLine(k types.KLine)            { for _, cb := range s.kLineCallbacks { cb(k) } }

func (s *GRPCStream) OnKLineClosed(cb func(types.KLine)) { s.kLineClosedCallbacks = append(s.kLineClosedCallbacks, cb) }
func (s *GRPCStream) EmitKLineClosed(k types.KLine)      { for _, cb := range s.kLineClosedCallbacks { cb(k) } }

func (s *GRPCStream) OnMarketTrade(cb func(types.Trade))  { s.marketTradeCallbacks = append(s.marketTradeCallbacks, cb) }
func (s *GRPCStream) EmitMarketTrade(t types.Trade)       { for _, cb := range s.marketTradeCallbacks { cb(t) } }

func (s *GRPCStream) OnBookUpdate(cb func(types.SliceOrderBook))   { s.bookUpdateCallbacks = append(s.bookUpdateCallbacks, cb) }
func (s *GRPCStream) EmitBookUpdate(b types.SliceOrderBook)        { for _, cb := range s.bookUpdateCallbacks { cb(b) } }

func (s *GRPCStream) OnBookSnapshot(cb func(types.SliceOrderBook)) { s.bookSnapshotCallbacks = append(s.bookSnapshotCallbacks, cb) }
func (s *GRPCStream) EmitBookSnapshot(b types.SliceOrderBook)      { for _, cb := range s.bookSnapshotCallbacks { cb(b) } }

func (s *GRPCStream) OnBookTickerUpdate(cb func(types.BookTicker)) { s.bookTickerCallbacks = append(s.bookTickerCallbacks, cb) }

func (s *GRPCStream) OnRawMessage(cb func(raw []byte)) {}

func (s *GRPCStream) OnTradeUpdate(cb func(types.Trade))       { s.tradeUpdateCallbacks = append(s.tradeUpdateCallbacks, cb) }
func (s *GRPCStream) OnOrderUpdate(cb func(types.Order))       { s.orderUpdateCallbacks = append(s.orderUpdateCallbacks, cb) }
func (s *GRPCStream) OnBalanceSnapshot(cb func(types.BalanceMap)) { s.balanceSnapshotCallbacks = append(s.balanceSnapshotCallbacks, cb) }
func (s *GRPCStream) OnBalanceUpdate(cb func(types.BalanceMap))   { s.balanceUpdateCallbacks = append(s.balanceUpdateCallbacks, cb) }

func (s *GRPCStream) OnAggTrade(cb func(types.Trade))               {}
func (s *GRPCStream) OnForceOrder(cb func(types.LiquidationInfo))    {}
func (s *GRPCStream) OnFuturesPositionUpdate(cb func(types.FuturesPositionMap))   {}
func (s *GRPCStream) OnFuturesPositionSnapshot(cb func(types.FuturesPositionMap)) {}

// --- helper: types.Subscription → pb.Subscription ---

func typesSubToPB(sub types.Subscription, exchangeName string) *pb.Subscription {
	pbSub := &pb.Subscription{
		Exchange: exchangeName,
		Symbol:   sub.Symbol,
	}

	switch sub.Channel {
	case types.MarketTradeChannel:
		pbSub.Channel = pb.Channel_TRADE
	case types.BookChannel:
		pbSub.Channel = pb.Channel_BOOK
		pbSub.Depth = string(sub.Options.Depth)
	case types.KLineChannel:
		pbSub.Channel = pb.Channel_KLINE
		pbSub.Interval = string(sub.Options.Interval)
	case types.BookTickerChannel:
		pbSub.Channel = pb.Channel_TICKER
	}

	return pbSub
}
