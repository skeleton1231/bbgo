package bbgo

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestPbKLineToTypes(t *testing.T) {
	pbK := &pb.KLine{
		Exchange:    "binance",
		Symbol:      "BTCUSDT",
		Open:        "100.5",
		High:        "110.0",
		Low:         "99.0",
		Close:       "105.0",
		Volume:      "500.25",
		QuoteVolume: "52526.125",
		StartTime:   1700000000000,
		EndTime:     1700000060000,
		Closed:      true,
	}

	k := pbKLineToTypes(pbK)

	assert.Equal(t, types.ExchangeName("binance"), k.Exchange)
	assert.Equal(t, "BTCUSDT", k.Symbol)
	assert.Equal(t, fixedpoint.MustNewFromString("100.5"), k.Open)
	assert.Equal(t, fixedpoint.MustNewFromString("110.0"), k.High)
	assert.Equal(t, fixedpoint.MustNewFromString("99.0"), k.Low)
	assert.Equal(t, fixedpoint.MustNewFromString("105.0"), k.Close)
	assert.Equal(t, fixedpoint.MustNewFromString("500.25"), k.Volume)
	assert.Equal(t, fixedpoint.MustNewFromString("52526.125"), k.QuoteVolume)
	assert.True(t, k.Closed)
}

func TestPbTradeToTypes(t *testing.T) {
	pbT := &pb.Trade{
		Exchange:    "binance",
		Symbol:      "ETHUSDT",
		Id:          "12345",
		Price:       "2000.50",
		Quantity:    "0.5",
		CreatedAt:   1700000000000,
		Side:        pb.Side_BUY,
		FeeCurrency: "USDT",
		Fee:         "0.1",
		Maker:       false,
	}

	tr := pbTradeToTypes(pbT)

	assert.Equal(t, types.ExchangeName("binance"), tr.Exchange)
	assert.Equal(t, "ETHUSDT", tr.Symbol)
	assert.Equal(t, uint64(12345), tr.ID)
	assert.Equal(t, fixedpoint.MustNewFromString("2000.50"), tr.Price)
	assert.Equal(t, fixedpoint.MustNewFromString("0.5"), tr.Quantity)
	assert.Equal(t, types.SideTypeBuy, tr.Side)
	assert.Equal(t, "USDT", tr.FeeCurrency)
	assert.False(t, tr.IsMaker)
}

func TestPbTradeToTypesInvalidID(t *testing.T) {
	pbT := &pb.Trade{
		Id:       "not-a-number",
		Price:    "100",
		Quantity: "1",
		Side:     pb.Side_SELL,
		Fee:      "0",
	}

	tr := pbTradeToTypes(pbT)
	assert.Equal(t, uint64(0), tr.ID)
	assert.Equal(t, types.SideTypeSell, tr.Side)
}

func TestPbSideToTypes(t *testing.T) {
	assert.Equal(t, types.SideTypeBuy, pbSideToTypes(pb.Side_BUY))
	assert.Equal(t, types.SideTypeSell, pbSideToTypes(pb.Side_SELL))
	assert.Equal(t, types.SideTypeSell, pbSideToTypes(pb.Side(99)))
}

func TestPbDepthToBook(t *testing.T) {
	pbD := &pb.Depth{
		Symbol: "BTCUSDT",
		Asks: []*pb.PriceVolume{
			{Price: "101.0", Volume: "10.5"},
			{Price: "102.0", Volume: "5.0"},
		},
		Bids: []*pb.PriceVolume{
			{Price: "100.0", Volume: "20.0"},
		},
	}

	book := pbDepthToBook(pbD)

	assert.Equal(t, "BTCUSDT", book.Symbol)
	assert.Len(t, book.Asks, 2)
	assert.Len(t, book.Bids, 1)
	assert.Equal(t, fixedpoint.MustNewFromString("101.0"), book.Asks[0].Price)
	assert.Equal(t, fixedpoint.MustNewFromString("10.5"), book.Asks[0].Volume)
	assert.Equal(t, fixedpoint.MustNewFromString("100.0"), book.Bids[0].Price)
}

func TestPbTickerToTypes(t *testing.T) {
	pbT := &pb.Ticker{
		Exchange: "binance",
		Symbol:   "BTCUSDT",
		Open:     100.0,
		High:     110.0,
		Low:      99.0,
		Close:    105.0,
		Volume:   500.0,
	}

	ticker := pbTickerToTypes(pbT)

	assert.NotNil(t, ticker)
	assert.Equal(t, fixedpoint.NewFromFloat(100.0), ticker.Open)
	assert.Equal(t, fixedpoint.NewFromFloat(110.0), ticker.High)
	assert.Equal(t, fixedpoint.NewFromFloat(99.0), ticker.Low)
	assert.Equal(t, fixedpoint.NewFromFloat(105.0), ticker.Last)
	assert.Equal(t, fixedpoint.NewFromFloat(500.0), ticker.Volume)
}

func TestPbTickerToTypesNil(t *testing.T) {
	assert.Nil(t, pbTickerToTypes(nil))
}

func TestTypesSubToPB_KLine(t *testing.T) {
	sub := types.Subscription{
		Channel: types.KLineChannel,
		Symbol:  "BTCUSDT",
		Options: types.SubscribeOptions{Interval: types.Interval1m},
	}

	pbSub := typesSubToPB(sub, "binance")

	assert.Equal(t, "binance", pbSub.Exchange)
	assert.Equal(t, "BTCUSDT", pbSub.Symbol)
	assert.Equal(t, pb.Channel_KLINE, pbSub.Channel)
	assert.Equal(t, "1m", pbSub.Interval)
}

func TestTypesSubToPB_Book(t *testing.T) {
	sub := types.Subscription{
		Channel: types.BookChannel,
		Symbol:  "ETHUSDT",
		Options: types.SubscribeOptions{Depth: "20"},
	}

	pbSub := typesSubToPB(sub, "max")

	assert.Equal(t, pb.Channel_BOOK, pbSub.Channel)
	assert.Equal(t, "20", pbSub.Depth)
}

func TestTypesSubToPB_Trade(t *testing.T) {
	sub := types.Subscription{
		Channel: types.MarketTradeChannel,
		Symbol:  "BTCUSDT",
	}

	pbSub := typesSubToPB(sub, "kucoin")

	assert.Equal(t, pb.Channel_TRADE, pbSub.Channel)
}

func TestTypesSubToPB_BookTicker(t *testing.T) {
	sub := types.Subscription{
		Channel: types.BookTickerChannel,
		Symbol:  "BTCUSDT",
	}

	pbSub := typesSubToPB(sub, "bybit")

	assert.Equal(t, pb.Channel_TICKER, pbSub.Channel)
}

func TestGRPCStreamSubscribeAndCollect(t *testing.T) {
	s := NewGRPCStream(nil, "binance")

	s.Subscribe(types.KLineChannel, "BTCUSDT", types.SubscribeOptions{Interval: types.Interval1m})
	s.Subscribe(types.BookChannel, "ETHUSDT", types.SubscribeOptions{Depth: "20"})

	subs := s.GetSubscriptions()
	assert.Len(t, subs, 2)
	assert.Equal(t, types.KLineChannel, subs[0].Channel)
	assert.Equal(t, "BTCUSDT", subs[0].Symbol)
	assert.Equal(t, types.BookChannel, subs[1].Channel)
	assert.Equal(t, "ETHUSDT", subs[1].Symbol)
}

func TestGRPCStreamCallbacks(t *testing.T) {
	s := NewGRPCStream(nil, "binance")

	var klineReceived types.KLine
	var tradeReceived types.Trade
	var bookUpdateReceived types.SliceOrderBook
	var bookSnapshotReceived types.SliceOrderBook
	var connectCalled, startCalled, disconnectCalled bool

	s.OnKLine(func(k types.KLine) { klineReceived = k })
	s.OnMarketTrade(func(tr types.Trade) { tradeReceived = tr })
	s.OnBookUpdate(func(b types.SliceOrderBook) { bookUpdateReceived = b })
	s.OnBookSnapshot(func(b types.SliceOrderBook) { bookSnapshotReceived = b })
	s.OnConnect(func() { connectCalled = true })
	s.OnStart(func() { startCalled = true })
	s.OnDisconnect(func() { disconnectCalled = true })

	s.EmitConnect()
	assert.True(t, connectCalled)

	s.EmitStart()
	assert.True(t, startCalled)

	s.EmitDisconnect()
	assert.True(t, disconnectCalled)

	k := types.KLine{Symbol: "BTCUSDT", Close: fixedpoint.MustNewFromString("105.0")}
	s.EmitKLine(k)
	assert.Equal(t, "BTCUSDT", klineReceived.Symbol)

	tr := types.Trade{Symbol: "ETHUSDT", Price: fixedpoint.MustNewFromString("2000")}
	s.EmitMarketTrade(tr)
	assert.Equal(t, "ETHUSDT", tradeReceived.Symbol)

	book := types.SliceOrderBook{Symbol: "BTCUSDT"}
	s.EmitBookUpdate(book)
	assert.Equal(t, "BTCUSDT", bookUpdateReceived.Symbol)

	s.EmitBookSnapshot(book)
	assert.Equal(t, "BTCUSDT", bookSnapshotReceived.Symbol)
}

func TestGRPCStreamKLineClosed(t *testing.T) {
	s := NewGRPCStream(nil, "binance")

	var closedKline types.KLine
	s.OnKLineClosed(func(k types.KLine) { closedKline = k })

	s.dispatch(&pb.MarketData{
		Channel: pb.Channel_KLINE,
		Kline: &pb.KLine{
			Symbol:   "BTCUSDT",
			Open:     "100",
			High:     "110",
			Low:      "99",
			Close:    "105",
			Volume:   "500",
			Closed:   true,
			EndTime:  1700000060000,
		},
	})

	assert.Equal(t, "BTCUSDT", closedKline.Symbol)
	assert.True(t, closedKline.Closed)
}

func TestGRPCStreamDispatchBookSnapshot(t *testing.T) {
	s := NewGRPCStream(nil, "binance")

	var snap types.SliceOrderBook
	s.OnBookSnapshot(func(b types.SliceOrderBook) { snap = b })

	s.dispatch(&pb.MarketData{
		Channel: pb.Channel_BOOK,
		Event:   pb.Event_SNAPSHOT,
		Depth: &pb.Depth{
			Symbol: "BTCUSDT",
			Bids:   []*pb.PriceVolume{{Price: "100", Volume: "10"}},
		},
	})

	assert.Equal(t, "BTCUSDT", snap.Symbol)
	assert.Len(t, snap.Bids, 1)
}

func TestGRPCStreamClose(t *testing.T) {
	s := NewGRPCStream(nil, "binance")
	err := s.Close()
	assert.NoError(t, err)
}

func TestGRPCStreamGetPublicOnly(t *testing.T) {
	s := NewGRPCStream(nil, "binance")
	assert.True(t, s.GetPublicOnly())
}

func TestDirectExchangeSourceImplementsInterface(t *testing.T) {
	var _ MarketDataSource = (*DirectExchangeSource)(nil)
}

func TestSharedServiceSourceImplementsInterface(t *testing.T) {
	var _ MarketDataSource = (*SharedServiceSource)(nil)
}

func TestGRPCStreamResubscribeUpdatesSubs(t *testing.T) {
	s := NewGRPCStream(nil, "binance")
	s.Subscribe(types.KLineChannel, "BTCUSDT", types.SubscribeOptions{Interval: types.Interval1m})

	err := s.Resubscribe(func(oldSubs []types.Subscription) ([]types.Subscription, error) {
		return []types.Subscription{
			{Channel: types.KLineChannel, Symbol: "ETHUSDT", Options: types.SubscribeOptions{Interval: types.Interval5m}},
		}, nil
	})
	assert.NoError(t, err)

	subs := s.GetSubscriptions()
	assert.Len(t, subs, 1)
	assert.Equal(t, "ETHUSDT", subs[0].Symbol)
	assert.Equal(t, types.Interval5m, subs[0].Options.Interval)
}
