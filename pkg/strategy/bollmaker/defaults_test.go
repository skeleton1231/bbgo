package bollmaker

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
)

// TestStrategy_Defaults_FillsZeroBandWidthOnPartialConfig is a regression test for
// the collapsed-bollinger-bands bug where Defaults() only set BandWidth when the
// entire *BollingerSetting was nil. If the user config supplied interval+window
// but omitted bandWidth, BandWidth stayed 0 → UpBand == DownBand == SMA.
func TestStrategy_Defaults_FillsZeroBandWidthOnPartialConfig(t *testing.T) {
	s := &Strategy{
		Symbol: "BTCUSDT",
		IntervalWindow: types.IntervalWindow{Interval: types.Interval1h, Window: 15},
		NeutralBollinger: &BollingerSetting{
			IntervalWindow: types.IntervalWindow{Interval: types.Interval1h, Window: 20},
		},
		DefaultBollinger: &BollingerSetting{
			IntervalWindow: types.IntervalWindow{Interval: types.Interval1h, Window: 20},
		},
	}

	if err := s.Defaults(); err != nil {
		t.Fatalf("Defaults() returned error: %v", err)
	}

	if s.NeutralBollinger.BandWidth != 2.0 {
		t.Errorf("NeutralBollinger.BandWidth = %v, want 2.0 (default must be applied when zero)", s.NeutralBollinger.BandWidth)
	}
	if s.DefaultBollinger.BandWidth != 3.0 {
		t.Errorf("DefaultBollinger.BandWidth = %v, want 3.0 (default must be applied when zero)", s.DefaultBollinger.BandWidth)
	}
}

func TestStrategy_Defaults_PreservesExplicitBandWidth(t *testing.T) {
	s := &Strategy{
		Symbol: "BTCUSDT",
		IntervalWindow: types.IntervalWindow{Interval: types.Interval15m, Window: 15},
		NeutralBollinger: &BollingerSetting{
			IntervalWindow: types.IntervalWindow{Interval: types.Interval1h, Window: 30},
			BandWidth:      1.5,
		},
		DefaultBollinger: &BollingerSetting{
			IntervalWindow: types.IntervalWindow{Interval: types.Interval1h, Window: 30},
			BandWidth:      2.5,
		},
	}

	if err := s.Defaults(); err != nil {
		t.Fatalf("Defaults() returned error: %v", err)
	}

	if s.NeutralBollinger.BandWidth != 1.5 {
		t.Errorf("NeutralBollinger.BandWidth = %v, want 1.5 (explicit value must be preserved)", s.NeutralBollinger.BandWidth)
	}
	if s.DefaultBollinger.BandWidth != 2.5 {
		t.Errorf("DefaultBollinger.BandWidth = %v, want 2.5 (explicit value must be preserved)", s.DefaultBollinger.BandWidth)
	}
	if s.NeutralBollinger.Window != 30 {
		t.Errorf("NeutralBollinger.Window = %v, want 30 (explicit value must be preserved)", s.NeutralBollinger.Window)
	}
}

func TestStrategy_Defaults_FillsMissingIntervalAndWindow(t *testing.T) {
	s := &Strategy{
		Symbol: "BTCUSDT",
		IntervalWindow: types.IntervalWindow{Interval: types.Interval15m, Window: 15},
		NeutralBollinger: &BollingerSetting{
			BandWidth: 2.0,
		},
		DefaultBollinger: &BollingerSetting{
			BandWidth: 3.0,
		},
	}

	if err := s.Defaults(); err != nil {
		t.Fatalf("Defaults() returned error: %v", err)
	}

	if s.NeutralBollinger.Interval != types.Interval15m {
		t.Errorf("NeutralBollinger.Interval = %v, want %v (should fall back to strategy Interval)", s.NeutralBollinger.Interval, types.Interval15m)
	}
	if s.NeutralBollinger.Window != 20 {
		t.Errorf("NeutralBollinger.Window = %v, want 20", s.NeutralBollinger.Window)
	}
}
