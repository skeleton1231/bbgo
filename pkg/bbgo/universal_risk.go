package bbgo

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/envvar"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// UniversalRiskConfig defines risk parameters that apply to ANY strategy
// using GeneralOrderExecutor. The SaaS manager sets these via BBGO_UNIVERSAL_RISK_*
// env vars on the container; the executor's Bind() auto-wires them.
//
// Design rationale: only ~12 of the ~50 SaaS-exposed strategies declare
// ExitMethods bbgo.ExitMethodSet. Rather than modify each strategy struct,
// we attach the risk controller at the executor layer so every strategy
// using GeneralOrderExecutor inherits it transparently.
type UniversalRiskConfig struct {
	enabled bool

	stopLossPrice   fixedpoint.Value
	takeProfitPrice fixedpoint.Value

	roiStopLoss   fixedpoint.Value
	roiTakeProfit fixedpoint.Value

	trailingActivation fixedpoint.Value
	trailingCallback   fixedpoint.Value

	maxPositionQty fixedpoint.Value
}

// Enabled reports whether any universal risk parameter is set.
func (c UniversalRiskConfig) Enabled() bool { return c.enabled }

// LoadUniversalRiskConfigFromEnv reads BBGO_UNIVERSAL_RISK_* env vars and
// returns the assembled config plus a flag indicating whether any were set.
// All values are parsed as fixedpoint; invalid or zero values are ignored.
func LoadUniversalRiskConfigFromEnv() (UniversalRiskConfig, bool) {
	cfg := UniversalRiskConfig{}

	set := func(name string, target *fixedpoint.Value) {
		v, ok := envvar.String(name)
		if !ok || v == "" {
			return
		}
		p, err := fixedpoint.NewFromString(v)
		if err != nil {
			log.WithError(err).Errorf("[UniversalRisk] cannot parse %s=%q as fixedpoint", name, v)
			return
		}
		if p.Sign() <= 0 {
			return
		}
		*target = p
		cfg.enabled = true
	}

	set("BBGO_UNIVERSAL_RISK_STOP_LOSS_PRICE", &cfg.stopLossPrice)
	set("BBGO_UNIVERSAL_RISK_TAKE_PROFIT_PRICE", &cfg.takeProfitPrice)
	set("BBGO_UNIVERSAL_RISK_ROI_STOP_LOSS", &cfg.roiStopLoss)
	set("BBGO_UNIVERSAL_RISK_ROI_TAKE_PROFIT", &cfg.roiTakeProfit)
	set("BBGO_UNIVERSAL_RISK_TRAILING_ACTIVATION", &cfg.trailingActivation)
	set("BBGO_UNIVERSAL_RISK_TRAILING_CALLBACK", &cfg.trailingCallback)
	set("BBGO_UNIVERSAL_RISK_MAX_POSITION_QTY", &cfg.maxPositionQty)

	return cfg, cfg.enabled
}

// UniversalRiskController watches a position through GeneralOrderExecutor's
// OnPositionUpdate callback and triggers a close when risk thresholds are
// crossed. State for trailing stop is reset when the position closes.
type UniversalRiskController struct {
	cfg UniversalRiskConfig

	session       *ExchangeSession
	orderExecutor *GeneralOrderExecutor

	// trailing stop state — protected by mu because OnPositionUpdate
	// and OnKLineClosed fire from different goroutines
	mu            sync.Mutex
	activated     bool
	latestExtreme fixedpoint.Value

	// closeFn overrides the default close path (which calls
	// orderExecutor.ClosePosition). Used by tests; nil in production.
	closeFn func(ctx context.Context, percentage fixedpoint.Value, tag string) error
}

// NewUniversalRiskController binds a controller to the given executor.
// The caller must invoke Bind() after GeneralOrderExecutor.Bind().
func NewUniversalRiskController(cfg UniversalRiskConfig, executor *GeneralOrderExecutor) *UniversalRiskController {
	return &UniversalRiskController{
		cfg:           cfg,
		orderExecutor: executor,
		session:       executor.Session(),
	}
}

// Bind wires OnPositionUpdate and a 1m KLine subscription.
// Safe to call once. Subscribes to kline via the session.
func (c *UniversalRiskController) Bind() {
	position := c.orderExecutor.Position()
	if position == nil {
		return
	}

	c.orderExecutor.TradeCollector().OnPositionUpdate(func(p *types.Position) {
		c.mu.Lock()
		defer c.mu.Unlock()

		if p.IsClosed() {
			c.activated = false
			c.latestExtreme = fixedpoint.Zero
			return
		}
		c.checkMaxPosition(p)
	})

	c.session.Subscribe(types.KLineChannel, position.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
	c.session.MarketDataStream.OnKLineClosed(types.KLineWith(position.Symbol, types.Interval1m, func(k types.KLine) {
		c.mu.Lock()
		defer c.mu.Unlock()

		if position.IsClosed() || position.IsDust(k.Close) || position.IsClosing() {
			return
		}
		c.checkPrice(position, k.Close)
	}))

	log.Infof("[UniversalRisk] %s bound: stopLossPrice=%s takeProfitPrice=%s roiStopLoss=%s roiTakeProfit=%s trailingAct=%s trailingCb=%s maxQty=%s",
		position.Symbol,
		c.cfg.stopLossPrice.String(),
		c.cfg.takeProfitPrice.String(),
		c.cfg.roiStopLoss.String(),
		c.cfg.roiTakeProfit.String(),
		c.cfg.trailingActivation.String(),
		c.cfg.trailingCallback.String(),
		c.cfg.maxPositionQty.String(),
	)
}

// checkMaxPosition fires immediately on position update if hard limit is exceeded.
// Caller must hold c.mu.
func (c *UniversalRiskController) checkMaxPosition(p *types.Position) {
	if c.cfg.maxPositionQty.IsZero() || p.IsClosing() {
		return
	}
	if p.GetBase().Abs().Compare(c.cfg.maxPositionQty) > 0 {
		log.Warnf("[UniversalRisk] %s max position qty %s exceeded (%s), closing",
			p.Symbol, c.cfg.maxPositionQty.String(), p.GetBase().String())
		c.triggerClose(p, "universalRisk:maxPositionQty")
	}
}

// checkPrice evaluates price-driven exits. Caller must hold c.mu.
func (c *UniversalRiskController) checkPrice(p *types.Position, price fixedpoint.Value) {
	if !c.cfg.stopLossPrice.IsZero() {
		long := p.IsLong()
		short := p.IsShort()
		if (long && price.Compare(c.cfg.stopLossPrice) <= 0) ||
			(short && price.Compare(c.cfg.stopLossPrice) >= 0) {
			log.Warnf("[UniversalRisk] %s stop loss hit at %s (SL=%s)", p.Symbol, price.String(), c.cfg.stopLossPrice.String())
			c.triggerClose(p, "universalRisk:stopLossPrice")
			return
		}
	}

	if !c.cfg.takeProfitPrice.IsZero() {
		long := p.IsLong()
		short := p.IsShort()
		if (long && price.Compare(c.cfg.takeProfitPrice) >= 0) ||
			(short && price.Compare(c.cfg.takeProfitPrice) <= 0) {
			log.Infof("[UniversalRisk] %s take profit hit at %s (TP=%s)", p.Symbol, price.String(), c.cfg.takeProfitPrice.String())
			c.triggerClose(p, "universalRisk:takeProfitPrice")
			return
		}
	}

	if !c.cfg.roiStopLoss.IsZero() {
		roi := p.ROI(price)
		threshold := c.cfg.roiStopLoss.Abs().Neg()
		if roi.Compare(threshold) < 0 {
			log.Warnf("[UniversalRisk] %s ROI stop loss triggered (roi=%s, threshold=%s)",
				p.Symbol, roi.Percentage(), threshold.Percentage())
			c.triggerClose(p, "universalRisk:roiStopLoss")
			return
		}
	}

	if !c.cfg.roiTakeProfit.IsZero() {
		roi := p.ROI(price)
		if roi.Compare(c.cfg.roiTakeProfit) >= 0 {
			log.Infof("[UniversalRisk] %s ROI take profit triggered (roi=%s, target=%s)",
				p.Symbol, roi.Percentage(), c.cfg.roiTakeProfit.Percentage())
			c.triggerClose(p, "universalRisk:roiTakeProfit")
			return
		}
	}

	if !c.cfg.trailingCallback.IsZero() {
		c.updateTrailing(p, price)
	}
}

// updateTrailing manages trailing-stop activation and drawdown check.
// Caller must hold c.mu.
func (c *UniversalRiskController) updateTrailing(p *types.Position, price fixedpoint.Value) {
	if !c.activated {
		if c.cfg.trailingActivation.IsZero() {
			c.activated = true
			c.latestExtreme = price
			return
		}
		var ratio fixedpoint.Value
		if p.IsLong() {
			ratio = price.Sub(p.AverageCost).Div(p.AverageCost)
		} else if p.IsShort() {
			ratio = p.AverageCost.Sub(price).Div(p.AverageCost)
		}
		if ratio.Compare(c.cfg.trailingActivation) >= 0 {
			c.activated = true
			c.latestExtreme = price
			log.Infof("[UniversalRisk] %s trailing stop activated at %s", p.Symbol, price.String())
		}
		return
	}

	if p.IsLong() && price.Compare(c.latestExtreme) > 0 {
		c.latestExtreme = price
	} else if p.IsShort() && (c.latestExtreme.IsZero() || price.Compare(c.latestExtreme) < 0) {
		c.latestExtreme = price
	}

	var drawdown fixedpoint.Value
	if p.IsLong() {
		drawdown = c.latestExtreme.Sub(price).Div(c.latestExtreme)
	} else if p.IsShort() {
		drawdown = price.Sub(c.latestExtreme).Div(c.latestExtreme)
	}

	if drawdown.Compare(c.cfg.trailingCallback) >= 0 {
		log.Infof("[UniversalRisk] %s trailing stop triggered at %s (drawdown=%s)",
			p.Symbol, price.String(), drawdown.Percentage())
		c.triggerClose(p, "universalRisk:trailingStop")
	}
}

// triggerClose fires ClosePosition and resets trailing state. Caller must hold c.mu.
// The mutex is released while ClosePosition runs because it submits orders
// through the session's stream; we reset state synchronously to prevent
// duplicate triggers before the close completes.
func (c *UniversalRiskController) triggerClose(p *types.Position, tag string) {
	c.activated = false
	c.latestExtreme = fixedpoint.Zero

	go func() {
		var err error
		if c.closeFn != nil {
			err = c.closeFn(context.Background(), fixedpoint.One, tag)
		} else {
			err = c.orderExecutor.ClosePosition(context.Background(), fixedpoint.One, tag)
		}
		if err != nil {
			log.WithError(err).Errorf("[UniversalRisk] failed to close position %s", p.Symbol)
		}
	}()
}
