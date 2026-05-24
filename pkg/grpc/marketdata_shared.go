package grpc

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
)

type subKey struct {
	channel string
	symbol  string
}

// SharedBroadcaster maintains a single shared exchange WebSocket stream
// per session and fans out received market data to all connected gRPC
// clients.  This avoids creating N WebSocket connections for N clients.
type SharedBroadcaster struct {
	mu      sync.RWMutex
	session *bbgo.ExchangeSession
	stream  types.Stream
	clients map[uint64]chan *pb.MarketData
	nextID  uint64
	subs    map[subKey]bool
	started bool
}

func NewSharedBroadcaster(session *bbgo.ExchangeSession) *SharedBroadcaster {
	return &SharedBroadcaster{
		session: session,
		clients: make(map[uint64]chan *pb.MarketData),
		subs:    make(map[subKey]bool),
	}
}

func (b *SharedBroadcaster) RegisterClient() (uint64, <-chan *pb.MarketData) {
	ch := make(chan *pb.MarketData, 256)
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	b.clients[id] = ch
	b.mu.Unlock()
	return id, ch
}

func (b *SharedBroadcaster) UnregisterClient(id uint64) {
	b.mu.Lock()
	ch, ok := b.clients[id]
	if ok {
		delete(b.clients, id)
	}
	b.mu.Unlock()
	if ok {
		close(ch)
	}
}

func (b *SharedBroadcaster) AddSubscriptions(subs []types.Subscription) {
	b.mu.Lock()
	changed := false
	for _, sub := range subs {
		key := subKey{channel: string(sub.Channel), symbol: sub.Symbol}
		if !b.subs[key] {
			b.subs[key] = true
			changed = true
		}
	}
	b.mu.Unlock()
	if changed {
		b.ensureStream()
	}
}

func (b *SharedBroadcaster) ensureStream() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stream == nil {
		stream := b.session.Exchange.NewStream()
		stream.SetPublicOnly()
		for key := range b.subs {
			stream.Subscribe(types.Channel(key.channel), key.symbol, types.SubscribeOptions{})
		}
		b.bindCallbacks(stream)
		b.stream = stream
		go func() {
			ctx := context.Background()
			if err := stream.Connect(ctx); err != nil {
				log.WithError(err).Error("shared stream connect failed")
			}
		}()
		b.started = true
		return
	}

	if b.started {
		_ = b.stream.Resubscribe(func(oldSubs []types.Subscription) ([]types.Subscription, error) {
			all := make([]types.Subscription, len(oldSubs))
			copy(all, oldSubs)
			for key := range b.subs {
				found := false
				for _, existing := range all {
					if string(existing.Channel) == key.channel && existing.Symbol == key.symbol {
						found = true
						break
					}
				}
				if !found {
					all = append(all, types.Subscription{
						Channel: types.Channel(key.channel),
						Symbol:  key.symbol,
					})
				}
			}
			return all, nil
		})
	}
}

func (b *SharedBroadcaster) bindCallbacks(stream types.Stream) {
	session := b.session
	stream.OnKLineClosed(func(kline types.KLine) {
		b.broadcast(transKLineResponse(session, kline))
	})
	stream.OnKLine(func(kline types.KLine) {
		b.broadcast(transKLineResponse(session, kline))
	})
	stream.OnMarketTrade(func(trade types.Trade) {
		b.broadcast(transMarketTrade(session, trade))
	})
	stream.OnBookSnapshot(func(book types.SliceOrderBook) {
		b.broadcast(transBook(session, book, pb.Event_SNAPSHOT))
	})
	stream.OnBookUpdate(func(book types.SliceOrderBook) {
		b.broadcast(transBook(session, book, pb.Event_UPDATE))
	})
}

func (b *SharedBroadcaster) broadcast(msg *pb.MarketData) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for id, ch := range b.clients {
		select {
		case ch <- msg:
		default:
			log.Debugf("broadcast: client %d channel full, dropping %v", id, msg.Channel)
		}
	}
}
