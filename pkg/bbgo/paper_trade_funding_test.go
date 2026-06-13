package bbgo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TestLastFundingSlotUTC verifies the helper snaps an arbitrary time to the
// most recent UTC 00/08/16 funding boundary.
func TestLastFundingSlotUTC(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"before first slot", "2026-06-13T05:59:59Z", "2026-06-13T00:00:00Z"},
		{"just after midnight", "2026-06-13T00:30:00Z", "2026-06-13T00:00:00Z"},
		{"mid morning", "2026-06-13T09:15:00Z", "2026-06-13T08:00:00Z"},
		{"just before 16:00", "2026-06-13T15:59:59Z", "2026-06-13T08:00:00Z"},
		{"evening", "2026-06-13T20:45:00Z", "2026-06-13T16:00:00Z"},
		{"exactly 16:00", "2026-06-13T16:00:00Z", "2026-06-13T16:00:00Z"},
		{"non-utc input stays correct", "2026-06-13T18:00:00+08:00", "2026-06-13T08:00:00Z"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, err := time.Parse(time.RFC3339, c.in)
			assert.NoError(t, err)
			want, err := time.Parse(time.RFC3339, c.want)
			assert.NoError(t, err)
			got := lastFundingSlotUTC(in)
			assert.True(t, got.Equal(want), "got %s want %s", got.UTC(), want.UTC())
		})
	}
}

// TestApplyFundingRate_AlignsToUTCSlot verifies that funding is applied and
// LastFundingTime snaps to the UTC slot, not the wall-clock run time.
func TestApplyFundingRate_AlignsToUTCSlot(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)
	state.LastFundingTime = time.Time{} // zero → always fires

	var fired bool
	e.OnFundingPayment = func(_ types.FundingPayment) { fired = true }

	balBefore, _ := e.account.Balance("USDT")
	e.applyFundingRate()

	assert.True(t, fired, "funding should fire when LastFundingTime is zero")
	balAfter, _ := e.account.Balance("USDT")
	assert.True(t, balAfter.Available.Compare(balBefore.Available) < 0, "long balance should decrease after paying funding")
	slot := lastFundingSlotUTC(time.Now())
	assert.True(t, state.LastFundingTime.Equal(slot),
		"LastFundingTime should be UTC slot %s, got %s", slot, state.LastFundingTime)
}

// TestApplyFundingRate_SkipsAlreadySettledSlot verifies that once funding has
// been applied for the current UTC slot, a second call in the same slot is a no-op.
func TestApplyFundingRate_SkipsAlreadySettledSlot(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(0.001)
	state.LastFundingTime = lastFundingSlotUTC(time.Now()) // already funded this slot

	var fired bool
	e.OnFundingPayment = func(_ types.FundingPayment) { fired = true }

	balBefore, _ := e.account.Balance("USDT")
	e.applyFundingRate()

	assert.False(t, fired, "funding must not fire twice in the same UTC slot")
	balAfter, _ := e.account.Balance("USDT")
	assert.True(t, balAfter.Available.Compare(balBefore.Available) == 0, "balance must be unchanged when slot already settled")
}

// TestApplyFundingRate_ShortReceivesFunding verifies that a short position
// receives funding when the rate is positive (longs pay shorts).
func TestApplyFundingRate_ShortReceivesFunding(t *testing.T) {
	e := newTestPaperTradeExchange()
	e.UseFutures()

	state := e.getOrCreateFuturesState("BTCUSDT")
	state.EntryPrice = fixedpoint.NewFromFloat(60000.0)
	state.PositionAmount = fixedpoint.NewFromFloat(-0.001) // short

	balBefore, _ := e.account.Balance("USDT")
	e.applyFundingRate()

	balAfter, _ := e.account.Balance("USDT")
	assert.True(t, balAfter.Available.Compare(balBefore.Available) > 0, "short balance should increase when receiving funding")
}
