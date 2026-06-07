package instanceid

import (
	"encoding/json"
	"testing"
)

func TestCompute_Simple(t *testing.T) {
	tests := []struct {
		strategy string
		symbol   string
		config   string
		want     string
	}{
		{"supertrend", "BTCUSDT", `{}`, "supertrend:BTCUSDT"},
		{"bollmaker", "ETHUSDT", `{}`, "bollmaker:ETHUSDT"},
		{"dca", "BTCUSDT", `{}`, "dca:BTCUSDT"},
		{"autobuy", "BTCUSDT", `{}`, "autobuy:BTCUSDT"},
		{"fmaker", "BTCUSDT", `{}`, "fmaker:BTCUSDT"},
		{"xmaker", "BTCUSDT", `{}`, "xmaker:BTCUSDT"},
	}
	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			got := Compute(tt.strategy, tt.symbol, json.RawMessage(tt.config))
			if got != tt.want {
				t.Errorf("Compute(%q, %q, ...) = %q, want %q", tt.strategy, tt.symbol, got, tt.want)
			}
		})
	}
}

func TestCompute_Grid2(t *testing.T) {
	config := `{"gridNumber":10,"upperPrice":"70000","lowerPrice":"50000"}`
	got := Compute("grid2", "BTCUSDT", json.RawMessage(config))
	want := "grid2-BTCUSDT-size-10-70000-50000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_Grid(t *testing.T) {
	config := `{"gridNumber":10,"upperPrice":"70000","lowerPrice":"50000"}`
	got := Compute("grid", "BTCUSDT", json.RawMessage(config))
	want := "grid-BTCUSDT-10-70000-50000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_Emacross(t *testing.T) {
	config := `{"interval":"1h","fastWindow":7,"slowWindow":25}`
	got := Compute("emacross", "BTCUSDT", json.RawMessage(config))
	want := "emacross:BTCUSDT:1h:7-25"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_Atrpin(t *testing.T) {
	config := `{"interval":"1h","window":14}`
	got := Compute("atrpin", "BTCUSDT", json.RawMessage(config))
	want := "atrpin:BTCUSDT:1h:14"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_WithInterval(t *testing.T) {
	tests := []struct {
		strategy string
		config   string
		want     string
	}{
		{"bollgrid", `{"interval":"1h"}`, "bollgrid:BTCUSDT:1h"},
		{"swing", `{"interval":"15m"}`, "swing:BTCUSDT:15m"},
		{"flashcrash", `{"interval":"5m"}`, "flashcrash:BTCUSDT:5m"},
	}
	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			got := Compute(tt.strategy, "BTCUSDT", json.RawMessage(tt.config))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompute_DashSymbol(t *testing.T) {
	tests := []struct {
		strategy string
		want     string
	}{
		{"dca2", "dca2-BTCUSDT"},
		{"dca3", "dca3-BTCUSDT"},
		{"xfunding", "xfunding-BTCUSDT"},
	}
	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			got := Compute(tt.strategy, "BTCUSDT", json.RawMessage(`{}`))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompute_IDOnly(t *testing.T) {
	tests := []struct {
		strategy string
		want     string
	}{
		{"rebalance", "rebalance"},
		{"tradingdesk", "tradingdesk"},
	}
	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			got := Compute(tt.strategy, "", json.RawMessage(`{}`))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompute_XHedgeGrid(t *testing.T) {
	config := `{"gridNumber":10,"upperPrice":"70000","lowerPrice":"50000"}`
	got := Compute("xhedgegrid", "BTCUSDT", json.RawMessage(config))
	want := "xhedgegrid-BTCUSDT-size-10-70000-50000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_XFundingV2(t *testing.T) {
	config := `{"candidateSymbols":["BTCUSDT","ETHUSDT"],"futuresDirection":"short"}`
	got := Compute("xfundingv2", "", json.RawMessage(config))
	want := "xfundingv2-BTCUSDT_ETHUSDT-short-futures"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_XDepthMaker(t *testing.T) {
	config := `{"makerExchange":"binance","hedgeExchange":"bybit","hedgeSymbol":"BTCUSDT"}`
	got := Compute("xdepthmaker", "BTCUSDT", json.RawMessage(config))
	want := "xdepthmaker-binance-BTCUSDT-bybit-BTCUSDT"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_XPremium(t *testing.T) {
	config := `{"baseSession":"binance","premiumSession":"bybit"}`
	got := Compute("xpremium", "BTCUSDT", json.RawMessage(config))
	want := "xpremium:binance:bybit:BTCUSDT"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_XGap(t *testing.T) {
	config := `{"tradingExchange":"binance"}`
	got := Compute("xgap", "BTCUSDT", json.RawMessage(config))
	want := "xgap:binance:BTCUSDT"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_Default(t *testing.T) {
	got := Compute("newstrategy", "BTCUSDT", json.RawMessage(`{}`))
	want := "newstrategy:BTCUSDT"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompute_NilConfig(t *testing.T) {
	got := Compute("supertrend", "BTCUSDT", nil)
	want := "supertrend:BTCUSDT"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
