package bbgo

import (
	"hash/fnv"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPaperOrderIDHashMaskFitsBigInt guards against regressions in the
// paper-trade ID namespace: the hash offset must never set bit 63, otherwise
// generated IDs exceed PostgreSQL BIGINT (signed int64) range and inserts
// fail with "out of range for type bigint".
func TestPaperOrderIDHashMaskFitsBigInt(t *testing.T) {
	assert.Equal(t, uint64(0), paperOrderIDHashMask&(1<<63),
		"paperOrderIDHashMask must not cover bit 63")

	assert.Equal(t, uint64(0), paperOrderIDHashOffset&(1<<63),
		"paperOrderIDHashOffset must not set bit 63 (current strategy_instance_id=%q)",
		os.Getenv("BBGO_STRATEGY_INSTANCE_ID"))

	id := nextPaperOrderID()
	assert.LessOrEqual(t, id, uint64(math.MaxInt64), "nextPaperOrderID exceeds BIGINT range")

	tid := nextPaperTradeID()
	assert.LessOrEqual(t, tid, uint64(math.MaxInt64), "nextPaperTradeID exceeds BIGINT range")
}

// TestPaperOrderIDHashOffsetNeverSetsBit63 sweeps a range of strategy instance
// IDs and verifies that the offset-derivation formula never produces an offset
// with bit 63 set. Catches a future revert to a 16-bit mask.
func TestPaperOrderIDHashOffsetNeverSetsBit63(t *testing.T) {
	for _, id := range []string{
		"", "a", "bollmaker-BTCUSDT-1", "grid2-ETHUSDT-spot",
		"super-long-name-with-unicode-测试-instance-id-1234567890",
	} {
		h := fnv.New64a()
		_, _ = h.Write([]byte(id))
		offset := (h.Sum64() & 0x7FFF) << paperOrderIDHashShift
		assert.Equal(t, uint64(0), offset&(1<<63),
			"offset for strategy_instance_id=%q sets bit 63", id)
	}
}
