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
	return types.KLine{
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
