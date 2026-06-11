package grpc

import (
	"testing"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
)

func newTestSession() *bbgo.ExchangeSession {
	return &bbgo.ExchangeSession{
		ExchangeSessionConfig: bbgo.ExchangeSessionConfig{
			Name:         "binance",
			ExchangeName: types.ExchangeBinance,
		},
	}
}

func TestToSubscriptions_Trade(t *testing.T) {
	sub := &pb.Subscription{Symbol: "BTCUSDT", Channel: pb.Channel_TRADE}
	got, err := toSubscriptions(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got.Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", got.Symbol)
	}
	if got.Channel != types.MarketTradeChannel {
		t.Errorf("expected MarketTradeChannel, got %s", got.Channel)
	}
}

func TestToSubscriptions_Book(t *testing.T) {
	sub := &pb.Subscription{Symbol: "ETHUSDT", Channel: pb.Channel_BOOK, Depth: "20"}
	got, err := toSubscriptions(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got.Channel != types.BookChannel {
		t.Errorf("expected BookChannel, got %s", got.Channel)
	}
	if string(got.Options.Depth) != "20" {
		t.Errorf("expected depth 20, got %s", got.Options.Depth)
	}
}

func TestToSubscriptions_Kline(t *testing.T) {
	sub := &pb.Subscription{Symbol: "BTCUSDT", Channel: pb.Channel_KLINE, Interval: "1h"}
	got, err := toSubscriptions(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got.Channel != types.KLineChannel {
		t.Errorf("expected KLineChannel, got %s", got.Channel)
	}
	if string(got.Options.Interval) != "1h" {
		t.Errorf("expected interval 1h, got %s", got.Options.Interval)
	}
}

func TestToSubscriptions_Ticker(t *testing.T) {
	sub := &pb.Subscription{Symbol: "BTCUSDT", Channel: pb.Channel_TICKER}
	got, err := toSubscriptions(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got.Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", got.Symbol)
	}
	if got.Channel != types.BookTickerChannel {
		t.Errorf("expected BookTickerChannel, got %s", got.Channel)
	}
}

func TestToSubscriptions_Unsupported(t *testing.T) {
	sub := &pb.Subscription{Symbol: "BTCUSDT", Channel: pb.Channel(999)}
	_, err := toSubscriptions(sub)
	if err == nil {
		t.Fatal("expected error for unsupported channel")
	}
}

func TestTransKLine(t *testing.T) {
	session := newTestSession()
	kline := types.KLine{
		Exchange:    types.ExchangeBinance,
		Symbol:      "BTCUSDT",
		Open:        fixedpoint.NewFromFloat(100.5),
		High:        fixedpoint.NewFromFloat(110.0),
		Low:         fixedpoint.NewFromFloat(99.0),
		Close:       fixedpoint.NewFromFloat(105.0),
		Volume:      fixedpoint.NewFromFloat(500.0),
		QuoteVolume: fixedpoint.NewFromFloat(50000.0),
		Closed:      true,
	}

	got := transKLine(session, kline)
	if got.Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", got.Symbol)
	}
	if got.Open != "100.5" {
		t.Errorf("expected 100.5, got %s", got.Open)
	}
	if got.Exchange != "binance" {
		t.Errorf("expected binance, got %s", got.Exchange)
	}
	if !got.Closed {
		t.Error("expected closed=true")
	}
}

func TestTransKLineResponse(t *testing.T) {
	session := newTestSession()
	kline := types.KLine{
		Exchange: types.ExchangeBinance,
		Symbol:   "ETHUSDT",
		Open:     fixedpoint.NewFromFloat(2000.0),
		Close:    fixedpoint.NewFromFloat(2050.0),
		Closed:   false,
	}

	got := transKLineResponse(session, kline)
	if got.Channel != pb.Channel_KLINE {
		t.Errorf("expected KLINE channel, got %v", got.Channel)
	}
	if got.Event != pb.Event_UPDATE {
		t.Errorf("expected UPDATE event, got %v", got.Event)
	}
	if got.Kline == nil {
		t.Fatal("expected kline in response")
	}
	if got.Kline.Symbol != "ETHUSDT" {
		t.Errorf("expected ETHUSDT, got %s", got.Kline.Symbol)
	}
}

func TestTransSide(t *testing.T) {
	tests := []struct {
		input    types.SideType
		expected pb.Side
	}{
		{types.SideTypeBuy, pb.Side_BUY},
		{types.SideTypeSell, pb.Side_SELL},
		{types.SideType("unknown"), pb.Side_SELL},
	}
	for _, tt := range tests {
		got := transSide(tt.input)
		if got != tt.expected {
			t.Errorf("transSide(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestTransOrderType(t *testing.T) {
	tests := []struct {
		input    types.OrderType
		expected pb.OrderType
	}{
		{types.OrderTypeLimit, pb.OrderType_LIMIT},
		{types.OrderTypeMarket, pb.OrderType_MARKET},
		{types.OrderTypeStopLimit, pb.OrderType_STOP_LIMIT},
		{types.OrderTypeStopMarket, pb.OrderType_STOP_MARKET},
		{types.OrderType("unknown"), pb.OrderType_LIMIT},
	}
	for _, tt := range tests {
		got := transOrderType(tt.input)
		if got != tt.expected {
			t.Errorf("transOrderType(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestToOrderType(t *testing.T) {
	tests := []struct {
		input    pb.OrderType
		expected types.OrderType
	}{
		{pb.OrderType_MARKET, types.OrderTypeMarket},
		{pb.OrderType_LIMIT, types.OrderTypeLimit},
		{pb.OrderType(999), types.OrderTypeLimit},
	}
	for _, tt := range tests {
		got := toOrderType(tt.input)
		if got != tt.expected {
			t.Errorf("toOrderType(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestToSide(t *testing.T) {
	tests := []struct {
		input    pb.Side
		expected types.SideType
	}{
		{pb.Side_BUY, types.SideTypeBuy},
		{pb.Side_SELL, types.SideTypeSell},
		{pb.Side(999), types.SideType("")},
	}
	for _, tt := range tests {
		got := toSide(tt.input)
		if got != tt.expected {
			t.Errorf("toSide(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTransOrder(t *testing.T) {
	session := newTestSession()
	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:        "BTCUSDT",
			Side:          types.SideTypeBuy,
			Type:          types.OrderTypeLimit,
			Price:         fixedpoint.NewFromFloat(50000.0),
			Quantity:      fixedpoint.NewFromFloat(0.1),
			ClientOrderID: "test-client-id",
			GroupID:       7,
		},
		Exchange:         types.ExchangeBinance,
		OrderID:          12345,
		ExecutedQuantity: fixedpoint.NewFromFloat(0.05),
		Status:           types.OrderStatusNew,
	}

	got := transOrder(session, order)
	if got.Symbol != "BTCUSDT" {
		t.Errorf("expected BTCUSDT, got %s", got.Symbol)
	}
	if got.Id != "12345" {
		t.Errorf("expected 12345, got %s", got.Id)
	}
	if got.Side != pb.Side_BUY {
		t.Errorf("expected BUY, got %v", got.Side)
	}
	if got.Price != "50000" {
		t.Errorf("expected 50000, got %s", got.Price)
	}
	if got.Status != "NEW" {
		t.Errorf("expected NEW, got %s", got.Status)
	}
	if got.ClientOrderId != "test-client-id" {
		t.Errorf("expected test-client-id, got %s", got.ClientOrderId)
	}
	if got.GroupId != 7 {
		t.Errorf("expected group 7, got %d", got.GroupId)
	}
}

func TestTransTrade(t *testing.T) {
	session := newTestSession()
	trade := types.Trade{
		Exchange:    types.ExchangeBinance,
		Symbol:      "BTCUSDT",
		ID:          999,
		Price:       fixedpoint.NewFromFloat(49000.0),
		Quantity:    fixedpoint.NewFromFloat(0.5),
		Side:        types.SideTypeSell,
		Fee:         fixedpoint.NewFromFloat(0.001),
		FeeCurrency: "BNB",
		IsMaker:     true,
	}

	got := transTrade(session, trade)
	if got.Id != "999" {
		t.Errorf("expected 999, got %s", got.Id)
	}
	if got.Price != "49000" {
		t.Errorf("expected 49000, got %s", got.Price)
	}
	if got.Side != pb.Side_SELL {
		t.Errorf("expected SELL, got %v", got.Side)
	}
	if got.FeeCurrency != "BNB" {
		t.Errorf("expected BNB, got %s", got.FeeCurrency)
	}
	if !got.Maker {
		t.Error("expected maker=true")
	}
}

func TestTransMarketTrade(t *testing.T) {
	session := newTestSession()
	trade := types.Trade{
		Exchange: types.ExchangeBinance,
		Symbol:   "ETHUSDT",
		ID:       42,
		Price:    fixedpoint.NewFromFloat(3000.0),
		Quantity: fixedpoint.NewFromFloat(1.0),
		Side:     types.SideTypeBuy,
	}

	got := transMarketTrade(session, trade)
	if got.Channel != pb.Channel_TRADE {
		t.Errorf("expected TRADE channel, got %v", got.Channel)
	}
	if got.Event != pb.Event_UPDATE {
		t.Errorf("expected UPDATE event, got %v", got.Event)
	}
	if len(got.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(got.Trades))
	}
	if got.Trades[0].Price != "3000" {
		t.Errorf("expected 3000, got %s", got.Trades[0].Price)
	}
}

func TestTransBalances(t *testing.T) {
	session := newTestSession()
	balances := types.BalanceMap{
		"BTC":  {Currency: "BTC", Available: fixedpoint.NewFromFloat(1.5), Locked: fixedpoint.NewFromFloat(0.5)},
		"USDT": {Currency: "USDT", Available: fixedpoint.NewFromFloat(10000.0), Locked: fixedpoint.NewFromFloat(2000.0)},
	}

	got := transBalances(session, balances)
	if len(got) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(got))
	}
	found := map[string]bool{}
	for _, b := range got {
		found[b.Currency] = true
		if b.Currency == "BTC" {
			if b.Available != "1.5" {
				t.Errorf("expected BTC available 1.5, got %s", b.Available)
			}
			if b.Locked != "0.5" {
				t.Errorf("expected BTC locked 0.5, got %s", b.Locked)
			}
		}
	}
	if !found["BTC"] || !found["USDT"] {
		t.Error("expected BTC and USDT in balances")
	}
}

func TestTransBook(t *testing.T) {
	session := newTestSession()
	book := types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: fixedpoint.NewFromFloat(50000.0), Volume: fixedpoint.NewFromFloat(1.0)},
			{Price: fixedpoint.NewFromFloat(49900.0), Volume: fixedpoint.NewFromFloat(2.0)},
		},
		Asks: types.PriceVolumeSlice{
			{Price: fixedpoint.NewFromFloat(50100.0), Volume: fixedpoint.NewFromFloat(0.5)},
		},
	}

	got := transBook(session, book, pb.Event_SNAPSHOT)
	if got.Channel != pb.Channel_BOOK {
		t.Errorf("expected BOOK channel, got %v", got.Channel)
	}
	if got.Event != pb.Event_SNAPSHOT {
		t.Errorf("expected SNAPSHOT event, got %v", got.Event)
	}
	if got.Depth == nil {
		t.Fatal("expected depth")
	}
	if len(got.Depth.Bids) != 2 {
		t.Errorf("expected 2 bids, got %d", len(got.Depth.Bids))
	}
	if len(got.Depth.Asks) != 1 {
		t.Errorf("expected 1 ask, got %d", len(got.Depth.Asks))
	}
	if got.Depth.Bids[0].Price != "50000" {
		t.Errorf("expected bid price 50000, got %s", got.Depth.Bids[0].Price)
	}
}

func TestTransPriceVolume_Empty(t *testing.T) {
	got := transPriceVolume(nil)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

func TestPbSubscriptionToTypes(t *testing.T) {
	tests := []struct {
		name       string
		input      *pb.Subscription
		wantSymbol string
		wantCh     types.Channel
	}{
		{"trade", &pb.Subscription{Symbol: "BTCUSDT", Channel: pb.Channel_TRADE}, "BTCUSDT", types.MarketTradeChannel},
		{"book", &pb.Subscription{Symbol: "ETHUSDT", Channel: pb.Channel_BOOK, Depth: "20"}, "ETHUSDT", types.BookChannel},
		{"kline", &pb.Subscription{Symbol: "BTCUSDT", Channel: pb.Channel_KLINE, Interval: "1h"}, "BTCUSDT", types.KLineChannel},
		{"ticker", &pb.Subscription{Symbol: "ETHUSDT", Channel: pb.Channel_TICKER}, "ETHUSDT", types.BookTickerChannel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pbSubscriptionToTypes(tt.input)
			if got.Symbol != tt.wantSymbol {
				t.Errorf("symbol: got %s, want %s", got.Symbol, tt.wantSymbol)
			}
			if got.Channel != tt.wantCh {
				t.Errorf("channel: got %s, want %s", got.Channel, tt.wantCh)
			}
		})
	}
}

func TestSharedBroadcaster_RegisterUnregister(t *testing.T) {
	bc := NewSharedBroadcaster(nil)

	id1, ch1 := bc.RegisterClient()
	if id1 != 0 {
		t.Errorf("expected first ID=0, got %d", id1)
	}

	id2, ch2 := bc.RegisterClient()
	if id2 != 1 {
		t.Errorf("expected second ID=1, got %d", id2)
	}

	bc.UnregisterClient(id1)
	_, ok := <-ch1
	if ok {
		t.Error("channel should be closed after unregister")
	}

	bc.UnregisterClient(id2)
	_, ok = <-ch2
	if ok {
		t.Error("channel should be closed after unregister")
	}

	bc.mu.RLock()
	count := len(bc.clients)
	bc.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}

func TestSharedBroadcaster_UnregisterUnknown(t *testing.T) {
	bc := NewSharedBroadcaster(nil)
	bc.UnregisterClient(999)
}

func TestSharedBroadcaster_Broadcast(t *testing.T) {
	bc := NewSharedBroadcaster(nil)
	_, ch := bc.RegisterClient()

	msg := &pb.MarketData{
		Exchange: "binance",
		Symbol:   "BTCUSDT",
		Channel:  pb.Channel_KLINE,
	}
	bc.broadcast(msg)

	select {
	case got := <-ch:
		if got.Symbol != "BTCUSDT" {
			t.Errorf("expected BTCUSDT, got %s", got.Symbol)
		}
	default:
		t.Fatal("expected message on channel")
	}
}

func TestSharedBroadcaster_BroadcastDropWhenFull(t *testing.T) {
	bc := NewSharedBroadcaster(nil)
	id, ch := bc.RegisterClient()

	for i := 0; i < 256; i++ {
		bc.clients[id] <- &pb.MarketData{Symbol: "fill"}
	}

	bc.broadcast(&pb.MarketData{Symbol: "dropped"})

	drained := 0
	for range ch {
		drained++
		if drained == 256 {
			break
		}
	}
}

func TestSharedBroadcaster_AddSubscriptions_Dedup(t *testing.T) {
	bc := NewSharedBroadcaster(nil)
	// Prevent ensureStream from running by not triggering stream creation
	bc.mu.Lock()
	key1 := subKey{channel: "kline", symbol: "BTCUSDT", interval: "1m"}
	bc.subs[key1] = true
	key2 := subKey{channel: "kline", symbol: "BTCUSDT", interval: "1m"}
	bc.subs[key2] = true
	count := len(bc.subs)
	bc.mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 unique subscription from dedup, got %d", count)
	}
}

func TestSharedBroadcaster_AddSubscriptions_DifferentIntervals(t *testing.T) {
	bc := NewSharedBroadcaster(nil)
	bc.mu.Lock()
	bc.subs[subKey{channel: "kline", symbol: "BTCUSDT", interval: "1m"}] = true
	bc.subs[subKey{channel: "kline", symbol: "BTCUSDT", interval: "5m"}] = true
	count := len(bc.subs)
	bc.mu.Unlock()
	if count != 2 {
		t.Errorf("expected 2 unique subscriptions, got %d", count)
	}
}

// TestSharedBroadcaster_ResubscribeDifferentIntervals verifies that the
// Resubscribe dedup in ensureStream() distinguishes kline subscriptions
// by interval. Without the fix, adding BTCUSDT kline 15m when 1m already
// exists would be silently dropped.
func TestSharedBroadcaster_ResubscribeDifferentIntervals(t *testing.T) {
	bc := NewSharedBroadcaster(nil)
	bc.started = true

	bc.mu.Lock()
	bc.subs[subKey{channel: "kline", symbol: "BTCUSDT", interval: "1m"}] = true
	bc.subs[subKey{channel: "kline", symbol: "BTCUSDT", interval: "15m"}] = true
	bc.mu.Unlock()

	oldSubs := []types.Subscription{
		{Channel: types.KLineChannel, Symbol: "BTCUSDT", Options: types.SubscribeOptions{Interval: types.Interval1m}},
	}

	var newSubs []types.Subscription
	for key := range bc.subs {
		found := false
		for _, existing := range oldSubs {
			if string(existing.Channel) == key.channel && existing.Symbol == key.symbol && string(existing.Options.Interval) == key.interval && string(existing.Options.Depth) == key.depth {
				found = true
				break
			}
		}
		if !found {
			newSubs = append(newSubs, types.Subscription{
				Channel: types.Channel(key.channel),
				Symbol:  key.symbol,
				Options: types.SubscribeOptions{
					Interval: types.Interval(key.interval),
					Depth:    types.Depth(key.depth),
				},
			})
		}
	}

	if len(newSubs) != 1 {
		t.Fatalf("expected 1 new subscription (15m), got %d: %+v", len(newSubs), newSubs)
	}
	if string(newSubs[0].Options.Interval) != "15m" {
		t.Errorf("expected new sub interval=15m, got %s", newSubs[0].Options.Interval)
	}
}
