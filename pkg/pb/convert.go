package pb

import (
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func PbKLineToTypes(k *KLine) types.KLine {
	return types.KLine{
		Exchange:    types.ExchangeName(k.Exchange),
		Symbol:      k.Symbol,
		Open:        fixedpoint.MustNewFromString(k.Open),
		High:        fixedpoint.MustNewFromString(k.High),
		Low:         fixedpoint.MustNewFromString(k.Low),
		Close:       fixedpoint.MustNewFromString(k.Close),
		Volume:      fixedpoint.MustNewFromString(k.Volume),
		QuoteVolume: fixedpoint.MustNewFromString(k.QuoteVolume),
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
		Price:       fixedpoint.MustNewFromString(t.Price),
		Quantity:    fixedpoint.MustNewFromString(t.Quantity),
		Time:        types.Time(time.UnixMilli(t.CreatedAt)),
		Side:        PbSideToTypes(t.Side),
		FeeCurrency: t.FeeCurrency,
		Fee:         fixedpoint.MustNewFromString(t.Fee),
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
			Price:  fixedpoint.MustNewFromString(pv.Price),
			Volume: fixedpoint.MustNewFromString(pv.Volume),
		})
	}
	for _, pv := range d.Bids {
		book.Bids = append(book.Bids, types.PriceVolume{
			Price:  fixedpoint.MustNewFromString(pv.Price),
			Volume: fixedpoint.MustNewFromString(pv.Volume),
		})
	}
	return book
}
