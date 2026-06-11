package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TestBOLL_MirrorsIndicatorSetProductionFlow reproduces the EXACT ordering used by
// bbgo.IndicatorSet.KLines / CLOSE / BOLL:
//
//	1. kLines := KLines(stream, symbol, interval)   // creates stream
//	2. kLines.BackFill(window)                       // BackFill BEFORE subscribers attach
//	3. closePrices := ClosePrices(kLines)            // subscribes via AddSubscriber -> replay
//	4. boll := BOLL(closePrices, window, k)          // subscribes via AddSubscriber -> replay
//
// If AddSubscriber replay works at every layer, BOLL.StdDev must be non-zero.
func TestBOLL_MirrorsIndicatorSetProductionFlow(t *testing.T) {
	interval := types.Interval1h
	symbol := "BTCUSDT"

	// step 1: create a KLineStream (no real upstream needed for this test)
	kLineStream := &KLineStream{}

	// step 2: BackFill BEFORE any downstream subscriber attaches
	historicalCloses := []float64{62000, 62100, 62200, 62150, 62250, 62300, 62280, 62310, 62340, 62320}
	history := make([]types.KLine, 0, len(historicalCloses))
	for _, c := range historicalCloses {
		history = append(history, types.KLine{
			Symbol:   symbol,
			Interval: interval,
			Open:     fixedpoint.NewFromFloat(c),
			High:     fixedpoint.NewFromFloat(c),
			Low:      fixedpoint.NewFromFloat(c),
			Close:    fixedpoint.NewFromFloat(c),
			Closed:   true,
		})
	}
	kLineStream.BackFill(history)

	// sanity: kLineStream buffered all entries even though no subscriber listened
	if got := kLineStream.Length(); got != len(historicalCloses) {
		t.Fatalf("kLineStream.Length() = %d, want %d (BackFill must populate buffer regardless of subscribers)", got, len(historicalCloses))
	}

	// step 3: ClosePrices subscribes NOW — AddSubscriber must replay buffered klines
	closePrices := ClosePrices(kLineStream)
	if got := closePrices.Length(); got != len(historicalCloses) {
		t.Fatalf("closePrices.Length() = %d, want %d (AddSubscriber replay from KLineStream failed)", got, len(historicalCloses))
	}

	// step 4: BOLL subscribes NOW — AddSubscriber must replay buffered closes
	boll := BOLL(closePrices, 20, 3.0)

	sma := boll.SMA.Last(0)
	std := boll.StdDev.Last(0)
	up := boll.UpBand.Last(0)
	down := boll.DownBand.Last(0)

	t.Logf("after production-order replay: sma=%v std=%v up=%v down=%v", sma, std, up, down)
	t.Logf("SMA rawValues length = %d", boll.SMA.Length())
	t.Logf("StdDev rawValues length = %d", boll.StdDev.Length())

	if std == 0 {
		t.Fatalf("StdDev is 0 — replay from PriceStream → StdDev failed in production-order flow")
	}
	if up == sma || down == sma {
		t.Fatalf("bands collapsed: up=%v sma=%v down=%v", up, sma, down)
	}
}
