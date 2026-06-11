package indicatorv2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

func TestBOLL_StdDevNotZeroWithDistinctValues(t *testing.T) {
	src := types.NewFloat64Series()
	boll := BOLL(src, 20, 3.0)

	closes := []float64{62200, 62250, 62300, 62280, 62320, 62290, 62310}

	for _, c := range closes {
		src.PushAndEmit(c)
	}

	sma := boll.SMA.Last(0)
	std := boll.StdDev.Last(0)
	up := boll.UpBand.Last(0)
	down := boll.DownBand.Last(0)

	t.Logf("sma=%v std=%v up=%v down=%v", sma, std, up, down)

	if std == 0 {
		t.Fatalf("stddev is 0 despite %d distinct closes — rawValues length should be %d",
			len(closes), len(closes))
	}

	if up == sma || down == sma {
		t.Fatalf("up==sma or down==sma — bands collapsed. up=%v sma=%v down=%v", up, sma, down)
	}
}

func TestBOLL_StdDevBackfillOrder(t *testing.T) {
	src := types.NewFloat64Series()
	for _, c := range []float64{100, 101, 102, 103, 104} {
		src.PushAndEmit(c)
	}

	boll := BOLL(src, 20, 3.0)

	sma := boll.SMA.Last(0)
	std := boll.StdDev.Last(0)

	t.Logf("after backfill: sma=%v std=%v", sma, std)

	if std == 0 {
		t.Fatalf("stddev is 0 after backfill of 5 distinct values")
	}
}
