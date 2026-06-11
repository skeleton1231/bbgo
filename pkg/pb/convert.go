package pb

import (
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func safeFixedPoint(s string) fixedpoint.Value {
	v, err := fixedpoint.NewFromString(s)
	if err != nil {
		log.WithError(err).Warnf("invalid fixedpoint string %q, defaulting to 0", s)
		return fixedpoint.Zero
	}
	return v
}

func PbKLineToTypes(k *KLine) types.KLine {
	t := types.KLine{
		Exchange:    types.ExchangeName(k.Exchange),
		Symbol:      k.Symbol,
		Open:        safeFixedPoint(k.Open),
		High:        safeFixedPoint(k.High),
		Low:         safeFixedPoint(k.Low),
		Close:       safeFixedPoint(k.Close),
		Volume:      safeFixedPoint(k.Volume),
		QuoteVolume: safeFixedPoint(k.QuoteVolume),
		StartTime:   types.Time(time.UnixMilli(k.StartTime)),
		EndTime:     types.Time(time.UnixMilli(k.EndTime)),
		Closed:      k.Closed,
	}
	t.Interval = IntervalFromDurationMs(t.EndTime.Time().Sub(t.StartTime.Time()).Milliseconds())
	return t
}

// IntervalFromDurationMs maps a kline duration in milliseconds to the canonical
// types.Interval. The proto KLine message lacks an interval field, so the
// interval must be derived from the start/end-time delta. Tolerates ±1% skew
// to absorb sub-second alignment differences across exchanges.
func IntervalFromDurationMs(ms int64) types.Interval {
	switch {
	case ms >= 57000 && ms <= 63000:
		return types.Interval1m
	case ms >= 177000 && ms <= 183000:
		return types.Interval3m
	case ms >= 297000 && ms <= 303000:
		return types.Interval5m
	case ms >= 897000 && ms <= 903000:
		return types.Interval15m
	case ms >= 1797000 && ms <= 1803000:
		return types.Interval30m
	case ms >= 3570000 && ms <= 3630000:
		return types.Interval1h
	case ms >= 7170000 && ms <= 7230000:
		return types.Interval2h
	case ms >= 14370000 && ms <= 14430000:
		return types.Interval4h
	case ms >= 21570000 && ms <= 21630000:
		return types.Interval6h
	case ms >= 43170000 && ms <= 43230000:
		return types.Interval12h
	case ms >= 86340000 && ms <= 86460000:
		return types.Interval1d
	case ms >= 259170000 && ms <= 259230000:
		return types.Interval3d
	case ms >= 604740000 && ms <= 604860000:
		return types.Interval1w
	}
	return ""
}

func PbTradeToTypes(t *Trade) types.Trade {
	id, err := strconv.ParseUint(t.Id, 10, 64)
	if err != nil {
		log.WithError(err).Warnf("invalid trade id: %s", t.Id)
	}
	return types.Trade{
		Exchange:    types.ExchangeName(t.Exchange),
		Symbol:      t.Symbol,
		ID:          id,
		Price:       safeFixedPoint(t.Price),
		Quantity:    safeFixedPoint(t.Quantity),
		Time:        types.Time(time.UnixMilli(t.CreatedAt)),
		Side:        PbSideToTypes(t.Side),
		FeeCurrency: t.FeeCurrency,
		Fee:         safeFixedPoint(t.Fee),
		IsMaker:     t.Maker,
	}
}

func PbSideToTypes(s Side) types.SideType {
	if s == Side_BUY {
		return types.SideTypeBuy
	}
	return types.SideTypeSell
}

func PbDepthToBook(d *Depth) types.SliceOrderBook {
	book := types.SliceOrderBook{Symbol: d.Symbol}
	for _, pv := range d.Asks {
		book.Asks = append(book.Asks, types.PriceVolume{
			Price:  safeFixedPoint(pv.Price),
			Volume: safeFixedPoint(pv.Volume),
		})
	}
	for _, pv := range d.Bids {
		book.Bids = append(book.Bids, types.PriceVolume{
			Price:  safeFixedPoint(pv.Price),
			Volume: safeFixedPoint(pv.Volume),
		})
	}
	return book
}
