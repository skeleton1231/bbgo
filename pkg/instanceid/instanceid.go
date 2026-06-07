// Package instanceid provides the single source of truth for strategy instance IDs.
// Both bbgo strategies and the SaaS manager use these functions to generate
// deterministic instance IDs from strategy config.
//
// When adding a new strategy:
//  1. Add a function here
//  2. Have the strategy's InstanceID() call it
//  3. The manager's computeInstanceID calls it too
//
// This guarantees one formula, one result, everywhere.
package instanceid

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Compute returns the bbgo-canonical instance ID for the given strategy + symbol + config.
// This is the main entry point for the manager, which may not know the strategy-specific fields.
func Compute(strategy, symbol string, config json.RawMessage) string {
	var params map[string]any
	if len(config) > 0 && string(config) != "null" {
		_ = json.Unmarshal(config, &params)
	}
	if params == nil {
		params = map[string]any{}
	}

	switch strategy {
	case "grid2", "xhedgegrid":
		return GridLike(strategy, symbol, paramInt(params, "gridNumber"), paramString(params, "upperPrice"), paramString(params, "lowerPrice"))
	case "grid":
		return Grid(symbol, paramInt(params, "gridNumber"), paramString(params, "upperPrice"), paramString(params, "lowerPrice"))
	case "bollgrid":
		return WithInterval(strategy, symbol, paramString(params, "interval"))
	case "emacross":
		return Emacross(symbol, paramString(params, "interval"), paramInt(params, "fastWindow"), paramInt(params, "slowWindow"))
	case "supertrend", "bollmaker":
		return Simple(strategy, symbol)
	case "dca":
		return Simple(strategy, symbol)
	case "dca2", "dca3":
		return DashSymbol(strategy, symbol)
	case "autobuy":
		return Simple(strategy, symbol)
	case "atrpin":
		return Atrpin(symbol, paramString(params, "interval"), paramInt(params, "window"))
	case "swing", "flashcrash":
		return WithInterval(strategy, symbol, paramString(params, "interval"))
	case "drift", "elliottwave":
		return WithBacktest(strategy, symbol, false)
	case "rebalance", "tradingdesk":
		return IDOnly(strategy)
	case "deposit2transfer":
		return Deposit2Transfer(paramString(params, "assets"))
	case "convert":
		return Convert(paramString(params, "from"), paramString(params, "to"))
	case "tri":
		return Tri(paramStringSlice(params, "symbols"))
	case "xfundingv2":
		return XFundingV2(paramStringSlice(params, "candidateSymbols"), paramString(params, "futuresDirection"))
	case "xdepthmaker":
		return XDepthMaker(paramString(params, "makerExchange"), symbol, paramString(params, "hedgeExchange"), paramString(params, "hedgeSymbol"))
	case "xpremium":
		return XPremium(paramString(params, "baseSession"), paramString(params, "premiumSession"), symbol)
	case "xgap":
		return XGap(paramString(params, "tradingExchange"), symbol)
	case "xalign":
		return XAlign(paramStringSlice(params, "preferredSessions"), paramStringSlice(params, "expectedBalances"))
	case "xmaker":
		return Simple(strategy, symbol)
	case "xfunding":
		return DashSymbol(strategy, symbol)
	default:
		return Simple(strategy, symbol)
	}
}

// Simple returns "{strategy}:{symbol}" — used by ~25 strategies.
func Simple(strategy, symbol string) string {
	return strategy + ":" + symbol
}

// DashSymbol returns "{strategy}-{symbol}".
func DashSymbol(strategy, symbol string) string {
	return strategy + "-" + symbol
}

// IDOnly returns just the strategy ID (for strategies without a symbol).
func IDOnly(strategy string) string {
	return strategy
}

// WithBacktest returns "{strategy}:{symbol}:{isBacktest}" — used by drift, elliottwave.
func WithBacktest(strategy, symbol string, isBacktest bool) string {
	return fmt.Sprintf("%s:%s:%v", strategy, symbol, isBacktest)
}

// WithInterval returns "{strategy}:{symbol}:{interval}".
func WithInterval(strategy, symbol, interval string) string {
	if interval != "" {
		return fmt.Sprintf("%s:%s:%s", strategy, symbol, interval)
	}
	return Simple(strategy, symbol)
}

// GridLike returns "{strategy}-{symbol}-size-{gridNum}-{upper}-{lower}".
// Used by grid2 and xhedgegrid.
func GridLike(strategy, symbol string, gridNum int, upper, lower string) string {
	return fmt.Sprintf("%s-%s-size-%d-%s-%s", strategy, symbol, gridNum, upper, lower)
}

// GridLikeAutoRange returns "{strategy}-{symbol}-size-{gridNum}-autoRange-{autoRange}".
func GridLikeAutoRange(strategy, symbol string, gridNum int, autoRange string) string {
	return fmt.Sprintf("%s-%s-size-%d-autoRange-%s", strategy, symbol, gridNum, autoRange)
}

// Grid returns "grid-{symbol}-{gridNum}-{upper}-{lower}".
func Grid(symbol string, gridNum int, upper, lower string) string {
	return fmt.Sprintf("grid-%s-%d-%s-%s", symbol, gridNum, upper, lower)
}

// Emacross returns "emacross:{symbol}:{interval}:{fast}-{slow}".
func Emacross(symbol, interval string, fast, slow int) string {
	return fmt.Sprintf("emacross:%s:%s:%d-%d", symbol, interval, fast, slow)
}

// Atrpin returns "atrpin:{symbol}:{interval}:{window}".
func Atrpin(symbol, interval string, window int) string {
	return fmt.Sprintf("atrpin:%s:%s:%d", symbol, interval, window)
}

// Deposit2Transfer returns "deposit2transfer-{assets}".
func Deposit2Transfer(assets string) string {
	return "deposit2transfer-" + assets
}

// Convert returns "convert:{from}-{to}".
func Convert(from, to string) string {
	return fmt.Sprintf("convert:%s-%s", from, to)
}

// Tri returns "tri" + joined symbols.
func Tri(symbols []string) string {
	return "tri" + strings.Join(symbols, "-")
}

// XFundingV2 returns "xfundingv2-{symbols}-{direction}-futures".
func XFundingV2(symbols []string, direction string) string {
	return fmt.Sprintf("xfundingv2-%s-%s-futures", strings.Join(symbols, "_"), direction)
}

// XDepthMaker returns "xdepthmaker-{makerExchange}-{symbol}-{hedgeExchange}-{hedgeSymbol}".
func XDepthMaker(makerExchange, symbol, hedgeExchange, hedgeSymbol string) string {
	return strings.Join([]string{"xdepthmaker", makerExchange, symbol, hedgeExchange, hedgeSymbol}, "-")
}

// XPremium returns "xpremium:{baseSession}:{premiumSession}:{symbol}".
func XPremium(baseSession, premiumSession, symbol string) string {
	return fmt.Sprintf("xpremium:%s:%s:%s", baseSession, premiumSession, symbol)
}

// XGap returns "xgap:{tradingExchange}:{symbol}".
func XGap(tradingExchange, symbol string) string {
	return fmt.Sprintf("xgap:%s:%s", tradingExchange, symbol)
}

// XAlign returns "xalign" + joined sessions + joined currencies.
func XAlign(sessions, currencies []string) string {
	return "xalign" + strings.Join(sessions, "-") + strings.Join(currencies, "-")
}

// --- JSON param helpers ---

func paramString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func paramInt(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0
		}
		return int(i)
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

func paramStringSlice(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch arr := v.(type) {
	case []any:
		var result []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return arr
	default:
		return nil
	}
}
