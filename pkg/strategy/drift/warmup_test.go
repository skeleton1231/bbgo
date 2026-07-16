package drift

import (
	"testing"

	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

// newWarmedUpStrategy builds the drift indicator pipeline the same way
// Strategy.initIndicators does, so the warmup gate can be exercised
// without a live exchange session.
func newWarmedUpStrategy(t *testing.T, predictOffset int) *Strategy {
	t.Helper()
	s := &Strategy{
		IntervalWindow:        types.IntervalWindow{Interval: types.Interval15m, Window: 5},
		SmootherWindow:        3,
		FisherTransformWindow: 3,
		PredictOffset:         predictOffset,
	}
	s.drift = &DriftMA{
		drift: &indicator.WeightedDrift{
			MA:             &indicator.SMA{IntervalWindow: s.IntervalWindow},
			IntervalWindow: s.IntervalWindow,
		},
		ma1: &indicator.EWMA{
			IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.SmootherWindow},
		},
		ma2: &indicator.FisherTransform{
			IntervalWindow: types.IntervalWindow{Interval: s.Interval, Window: s.FisherTransformWindow},
		},
	}
	s.drift.SeriesBase.Series = s.drift
	return s
}

// Test_DriftWarmedUpUsesLengthNotArraySlice is the regression test for the bug
// where the drift strategy permanently logged "indicator not warmed up" and
// never traded: the gate compared len(Array(2)) — which is structurally capped
// at 2 — against PredictOffset (>2 in normal configs), so it could never pass.
// The gate must compare the full series Length() against PredictOffset.
func Test_DriftWarmedUpUsesLengthNotArraySlice(t *testing.T) {
	const predictOffset = 10
	s := newWarmedUpStrategy(t, predictOffset)

	// Feed enough samples for both series to warm well past PredictOffset.
	price := 100.0
	for i := 0; i < 300; i++ {
		price *= 1.0005
		s.drift.Update(price, 10.0)
	}

	t.Logf("drift.Length()=%d  drift.drift.Length()=%d  len(Array(2))=%d",
		s.drift.Length(), s.drift.drift.Length(), len(s.drift.Array(2)))

	// Array(2) is structurally capped at 2 — the root cause of the old bug.
	assert.LessOrEqual(t, len(s.drift.Array(2)), 2,
		"Array(2) can never reach PredictOffset=%d, so it must not gate warmup", predictOffset)

	// After warmup the full series length exceeds PredictOffset, so the
	// corrected gate reports ready. With the old len(Array(2)) formulation
	// this was permanently false for any PredictOffset > 2.
	assert.True(t, s.driftWarmedUp(), "drift should be warmed up after sufficient samples")
	assert.True(t, s.driftDerivativeWarmedUp(), "drift-derivative should be warmed up after sufficient samples")
}

// Test_DriftNotWarmedUpBeforePredictOffset confirms the gate still blocks
// during the genuine warmup window (PredictOffset samples not yet reached),
// so the fix does not over-eagerly skip the warmup protection.
func Test_DriftNotWarmedUpBeforePredictOffset(t *testing.T) {
	const predictOffset = 10
	s := newWarmedUpStrategy(t, predictOffset)

	// Only a few samples — not enough to reach PredictOffset.
	price := 100.0
	for i := 0; i < 3; i++ {
		price *= 1.0005
		s.drift.Update(price, 10.0)
	}

	assert.False(t, s.driftWarmedUp(), "drift should not be warmed up before PredictOffset samples")
}

// Test_ValidateRejectsNegativePredictOffset locks in the semantic Validate()
// guard: a negative predictOffset would otherwise make driftWarmedUp()
// trivially true (Length() >= negative) and silently trade after 2 samples.
func Test_ValidateRejectsNegativePredictOffset(t *testing.T) {
	s := &Strategy{
		Symbol:         "BTCUSDT",
		MinInterval:    types.Interval5m,
		IntervalWindow: types.IntervalWindow{Interval: types.Interval15m, Window: 100},
		PredictOffset:  -1,
	}
	err := s.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "predictOffset")
}
