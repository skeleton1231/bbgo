package grpc

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestSubKeyIncludesIntervalAndDepth(t *testing.T) {
	subs := []types.Subscription{
		{Symbol: "BTCUSDT", Channel: types.KLineChannel, Options: types.SubscribeOptions{Interval: types.Interval("1h")}},
		{Symbol: "ETHUSDT", Channel: types.BookChannel, Options: types.SubscribeOptions{Depth: types.Depth("20")}},
	}

	keys := map[subKey]bool{}
	for _, sub := range subs {
		key := subKey{
			channel:  string(sub.Channel),
			symbol:   sub.Symbol,
			interval: string(sub.Options.Interval),
			depth:    string(sub.Options.Depth),
		}
		keys[key] = true
	}

	if _, ok := keys[subKey{channel: "kline", symbol: "BTCUSDT", interval: "1h"}]; !ok {
		t.Error("expected kline+interval subKey")
	}
	if _, ok := keys[subKey{channel: "book", symbol: "ETHUSDT", depth: "20"}]; !ok {
		t.Error("expected book+depth subKey")
	}
}

func TestSubKeyDifferentiatesInterval(t *testing.T) {
	k1 := subKey{channel: "kline", symbol: "BTCUSDT", interval: "1m"}
	k2 := subKey{channel: "kline", symbol: "BTCUSDT", interval: "1h"}
	if k1 == k2 {
		t.Error("different intervals should produce different keys")
	}
}

func TestSubKeyDifferentiatesDepth(t *testing.T) {
	k1 := subKey{channel: "book", symbol: "BTCUSDT", depth: "10"}
	k2 := subKey{channel: "book", symbol: "BTCUSDT", depth: "20"}
	if k1 == k2 {
		t.Error("different depths should produce different keys")
	}
}
