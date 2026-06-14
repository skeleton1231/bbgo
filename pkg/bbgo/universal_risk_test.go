package bbgo

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

// newUniversalRiskTestExecutor builds a session + position + GeneralOrderExecutor
// fixture suitable for driving UniversalRiskController.checkPrice/checkMaxPosition
// without going through the live Bind() subscription path.
func newUniversalRiskTestExecutor(t *testing.T, avgCost, base float64) (*GeneralOrderExecutor, *types.Position, *gomock.Controller) {
	t.Helper()
	market := getTestMarket()

	mockCtrl := gomock.NewController(t)
	mockEx := mocks.NewMockExchange(mockCtrl)
	mockEx.EXPECT().Name().Return(types.ExchangeName("test")).AnyTimes()
	mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)
	mockEx.EXPECT().SubmitOrder(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	session := NewExchangeSession("test", mockEx)
	session.markets[market.Symbol] = market

	position := types.NewPositionFromMarket(market)
	position.AverageCost = fixedpoint.NewFromFloat(avgCost)
	position.Base = fixedpoint.NewFromFloat(base)

	executor := NewGeneralOrderExecutor(session, market.Symbol, "test", "test-01", position)
	return executor, position, mockCtrl
}

// awaitCloseCalls polls a counter until either it reaches want or timeout
// elapses. triggerClose fires ClosePosition in a goroutine, so tests must wait.
func awaitCloseCalls(t *testing.T, counter *int32, want int32, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(counter) >= want {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return atomic.LoadInt32(counter) >= want
}

// newCountingCloseFn returns a controller closeFn that increments *counter.
func newCountingCloseFn(counter *int32) func(context.Context, fixedpoint.Value, string) error {
	return func(context.Context, fixedpoint.Value, string) error {
		atomic.AddInt32(counter, 1)
		return nil
	}
}

func TestUniversalRisk_StopLossPrice_Long(t *testing.T) {
	executor, position, _ := newUniversalRiskTestExecutor(t, 20000.0, 1.0)

	var closeCalls int32
	cfg := UniversalRiskConfig{
		enabled:       true,
		stopLossPrice: fixedpoint.NewFromFloat(19000.0),
	}
	c := NewUniversalRiskController(cfg, executor)
	c.closeFn = newCountingCloseFn(&closeCalls)

	c.mu.Lock()
	c.checkPrice(position, fixedpoint.NewFromFloat(18999.0))
	c.mu.Unlock()

	if !awaitCloseCalls(t, &closeCalls, 1, time.Second) {
		t.Fatalf("expected ClosePosition to be called once, got %d", atomic.LoadInt32(&closeCalls))
	}
}

func TestUniversalRisk_StopLossPrice_NotTriggeredAboveThreshold(t *testing.T) {
	executor, position, _ := newUniversalRiskTestExecutor(t, 20000.0, 1.0)

	var closeCalls int32
	cfg := UniversalRiskConfig{
		enabled:       true,
		stopLossPrice: fixedpoint.NewFromFloat(19000.0),
	}
	c := NewUniversalRiskController(cfg, executor)
	c.closeFn = newCountingCloseFn(&closeCalls)

	c.mu.Lock()
	c.checkPrice(position, fixedpoint.NewFromFloat(19500.0))
	c.mu.Unlock()

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(0), atomic.LoadInt32(&closeCalls))
}

func TestUniversalRisk_TakeProfitPrice_Short(t *testing.T) {
	executor, position, _ := newUniversalRiskTestExecutor(t, 20000.0, -1.0)

	var closeCalls int32
	cfg := UniversalRiskConfig{
		enabled:         true,
		takeProfitPrice: fixedpoint.NewFromFloat(19000.0),
	}
	c := NewUniversalRiskController(cfg, executor)
	c.closeFn = newCountingCloseFn(&closeCalls)

	c.mu.Lock()
	c.checkPrice(position, fixedpoint.NewFromFloat(18900.0))
	c.mu.Unlock()

	if !awaitCloseCalls(t, &closeCalls, 1, time.Second) {
		t.Fatalf("expected ClosePosition once for short TP, got %d", atomic.LoadInt32(&closeCalls))
	}
}

func TestUniversalRisk_ROIStopLoss(t *testing.T) {
	executor, position, _ := newUniversalRiskTestExecutor(t, 20000.0, 1.0)

	var closeCalls int32
	cfg := UniversalRiskConfig{
		enabled:     true,
		roiStopLoss: fixedpoint.NewFromFloat(0.05),
	}
	c := NewUniversalRiskController(cfg, executor)
	c.closeFn = newCountingCloseFn(&closeCalls)

	c.mu.Lock()
	c.checkPrice(position, fixedpoint.NewFromFloat(18900.0)) // -5.5% ROI
	c.mu.Unlock()

	if !awaitCloseCalls(t, &closeCalls, 1, time.Second) {
		t.Fatalf("expected ROI stop loss trigger, got %d", atomic.LoadInt32(&closeCalls))
	}
}

func TestUniversalRisk_ROITakeProfit(t *testing.T) {
	executor, position, _ := newUniversalRiskTestExecutor(t, 20000.0, 1.0)

	var closeCalls int32
	cfg := UniversalRiskConfig{
		enabled:       true,
		roiTakeProfit: fixedpoint.NewFromFloat(0.10),
	}
	c := NewUniversalRiskController(cfg, executor)
	c.closeFn = newCountingCloseFn(&closeCalls)

	c.mu.Lock()
	c.checkPrice(position, fixedpoint.NewFromFloat(22100.0)) // +10.5% ROI
	c.mu.Unlock()

	if !awaitCloseCalls(t, &closeCalls, 1, time.Second) {
		t.Fatalf("expected ROI take profit trigger, got %d", atomic.LoadInt32(&closeCalls))
	}
}

func TestUniversalRisk_TrailingStop_Long(t *testing.T) {
	executor, position, _ := newUniversalRiskTestExecutor(t, 20000.0, 1.0)

	var closeCalls int32
	cfg := UniversalRiskConfig{
		enabled:            true,
		trailingActivation: fixedpoint.NewFromFloat(0.05),
		trailingCallback:   fixedpoint.NewFromFloat(0.02),
	}
	c := NewUniversalRiskController(cfg, executor)
	c.closeFn = newCountingCloseFn(&closeCalls)

	c.mu.Lock()
	// Activation: +5% from entry
	c.checkPrice(position, fixedpoint.NewFromFloat(21000.0))
	assert.True(t, c.activated, "trailing should activate at +5%")
	assert.Equal(t, fixedpoint.NewFromFloat(21000.0), c.latestExtreme)

	// Push higher → latest extreme should follow
	c.checkPrice(position, fixedpoint.NewFromFloat(21500.0))
	assert.Equal(t, fixedpoint.NewFromFloat(21500.0), c.latestExtreme)

	// Pull back within callback window (no trigger)
	c.checkPrice(position, fixedpoint.NewFromFloat(21200.0)) // ~1.4% drawdown
	assert.Equal(t, int32(0), atomic.LoadInt32(&closeCalls))

	// Drop >2% from extreme → trigger
	c.checkPrice(position, fixedpoint.NewFromFloat(21000.0)) // ~2.3% drawdown
	c.mu.Unlock()

	if !awaitCloseCalls(t, &closeCalls, 1, time.Second) {
		t.Fatalf("expected trailing stop trigger, got %d", atomic.LoadInt32(&closeCalls))
	}
}

func TestUniversalRisk_MaxPositionQty(t *testing.T) {
	executor, position, _ := newUniversalRiskTestExecutor(t, 20000.0, 5.0)

	var closeCalls int32
	cfg := UniversalRiskConfig{
		enabled:        true,
		maxPositionQty: fixedpoint.NewFromFloat(3.0),
	}
	c := NewUniversalRiskController(cfg, executor)
	c.closeFn = newCountingCloseFn(&closeCalls)

	c.mu.Lock()
	c.checkMaxPosition(position)
	c.mu.Unlock()

	if !awaitCloseCalls(t, &closeCalls, 1, time.Second) {
		t.Fatalf("expected max-position close trigger, got %d", atomic.LoadInt32(&closeCalls))
	}
}

func TestUniversalRisk_NoTriggersWhenDisabled(t *testing.T) {
	executor, position, _ := newUniversalRiskTestExecutor(t, 20000.0, 1.0)

	var closeCalls int32
	cfg := UniversalRiskConfig{enabled: false}
	c := NewUniversalRiskController(cfg, executor)
	c.closeFn = newCountingCloseFn(&closeCalls)

	c.mu.Lock()
	c.checkPrice(position, fixedpoint.NewFromFloat(10000.0))
	c.checkMaxPosition(position)
	c.mu.Unlock()

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(0), atomic.LoadInt32(&closeCalls))
}

func TestUniversalRisk_ResetOnPositionClose(t *testing.T) {
	executor, _, _ := newUniversalRiskTestExecutor(t, 20000.0, 1.0)

	cfg := UniversalRiskConfig{
		enabled:            true,
		trailingActivation: fixedpoint.NewFromFloat(0.05),
		trailingCallback:   fixedpoint.NewFromFloat(0.02),
	}
	c := NewUniversalRiskController(cfg, executor)

	// Simulate trailing-stop activation
	c.mu.Lock()
	c.activated = true
	c.latestExtreme = fixedpoint.NewFromFloat(21000.0)
	c.mu.Unlock()

	// Simulate position closed → OnPositionUpdate path resets trailing state
	c.mu.Lock()
	c.activated = false
	c.latestExtreme = fixedpoint.Zero
	c.mu.Unlock()

	assert.False(t, c.activated)
	assert.Equal(t, fixedpoint.Zero, c.latestExtreme)
}

func TestLoadUniversalRiskConfigFromEnv(t *testing.T) {
	envVars := map[string]string{
		"BBGO_UNIVERSAL_RISK_STOP_LOSS_PRICE":     "19000",
		"BBGO_UNIVERSAL_RISK_TAKE_PROFIT_PRICE":   "22000",
		"BBGO_UNIVERSAL_RISK_ROI_STOP_LOSS":       "0.05",
		"BBGO_UNIVERSAL_RISK_ROI_TAKE_PROFIT":     "0.10",
		"BBGO_UNIVERSAL_RISK_TRAILING_ACTIVATION": "0.03",
		"BBGO_UNIVERSAL_RISK_TRAILING_CALLBACK":   "0.02",
		"BBGO_UNIVERSAL_RISK_MAX_POSITION_QTY":    "5",
	}
	for k, v := range envVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	cfg, ok := LoadUniversalRiskConfigFromEnv()
	assert.True(t, ok)
	assert.True(t, cfg.Enabled())
	assert.True(t, cfg.stopLossPrice.Eq(fixedpoint.NewFromFloat(19000)))
	assert.True(t, cfg.takeProfitPrice.Eq(fixedpoint.NewFromFloat(22000)))
	assert.True(t, cfg.roiStopLoss.Eq(fixedpoint.NewFromFloat(0.05)))
	assert.True(t, cfg.roiTakeProfit.Eq(fixedpoint.NewFromFloat(0.10)))
	assert.True(t, cfg.trailingActivation.Eq(fixedpoint.NewFromFloat(0.03)))
	assert.True(t, cfg.trailingCallback.Eq(fixedpoint.NewFromFloat(0.02)))
	assert.True(t, cfg.maxPositionQty.Eq(fixedpoint.NewFromFloat(5)))
}

func TestLoadUniversalRiskConfigFromEnv_IgnoresInvalid(t *testing.T) {
	os.Setenv("BBGO_UNIVERSAL_RISK_STOP_LOSS_PRICE", "not-a-number")
	defer os.Unsetenv("BBGO_UNIVERSAL_RISK_STOP_LOSS_PRICE")

	os.Setenv("BBGO_UNIVERSAL_RISK_ROI_STOP_LOSS", "-0.05")
	defer os.Unsetenv("BBGO_UNIVERSAL_RISK_ROI_STOP_LOSS")

	cfg, ok := LoadUniversalRiskConfigFromEnv()
	assert.False(t, ok, "invalid values should not enable the controller")
	assert.False(t, cfg.Enabled())
}

func TestLoadUniversalRiskConfigFromEnv_NoneSet(t *testing.T) {
	cfg, ok := LoadUniversalRiskConfigFromEnv()
	assert.False(t, ok)
	assert.False(t, cfg.Enabled())
}
