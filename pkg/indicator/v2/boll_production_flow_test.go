package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func makeKline(symbol string, interval types.Interval, close float64) types.KLine {
	return types.KLine{
		Symbol:   symbol,
		Interval: interval,
		Open:     fixedpoint.NewFromFloat(close),
		High:     fixedpoint.NewFromFloat(close),
		Low:      fixedpoint.NewFromFloat(close),
		Close:    fixedpoint.NewFromFloat(close),
		Closed:   true,
	}
}

func TestBOLL_ProductionFlow(t *testing.T) {
	interval := types.Interval1h
	symbol := "BTCUSDT"

	kLineStream := &KLineStream{}
	closePrices := ClosePrices(kLineStream)
	boll := BOLL(closePrices, 20, 3.0)

	historicalCloses := []float64{62000, 62100, 62200, 62150, 62250, 62300, 62280, 62310, 62340, 62320}
	history := make([]types.KLine, 0, len(historicalCloses))
	for _, c := range historicalCloses {
		history = append(history, makeKline(symbol, interval, c))
	}
	kLineStream.BackFill(history)

	sma := boll.SMA.Last(0)
	std := boll.StdDev.Last(0)
	t.Logf("after backfill: sma=%v std=%v sma-slice-len=%d", sma, std, boll.SMA.Length())

	if std == 0 {
		t.Fatalf("stddev is 0 after backfill of %d historical closes", len(historicalCloses))
	}
	if up := boll.UpBand.Last(0); up == sma {
		t.Fatalf("bands collapsed after backfill: up=%v sma=%v", up, sma)
	}
}

func TestBOLL_TwoInstancesOnSameSource(t *testing.T) {
	interval := types.Interval1h
	symbol := "BTCUSDT"

	kLineStream := &KLineStream{}
	closePrices := ClosePrices(kLineStream)

	neutralBoll := BOLL(closePrices, 20, 2.0)
	defaultBoll := BOLL(closePrices, 20, 3.0)

	closes := []float64{62000, 62100, 62200, 62150, 62250, 62300, 62280, 62310, 62340, 62320}
	history := make([]types.KLine, 0, len(closes))
	for _, c := range closes {
		history = append(history, makeKline(symbol, interval, c))
	}
	kLineStream.BackFill(history)

	nSma := neutralBoll.SMA.Last(0)
	nStd := neutralBoll.StdDev.Last(0)
	dSma := defaultBoll.SMA.Last(0)
	dStd := defaultBoll.StdDev.Last(0)

	t.Logf("neutral: sma=%v std=%v", nSma, nStd)
	t.Logf("default: sma=%v std=%v", dSma, dStd)

	if nStd == 0 {
		t.Fatalf("neutralBoll stddev is 0 — first instance bug")
	}
	if dStd == 0 {
		t.Fatalf("defaultBoll stddev is 0 — second instance bug")
	}
}
