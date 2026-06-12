package emacross

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/instanceid"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "emacross"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol     string           `json:"symbol"`
	Interval   types.Interval   `json:"interval"`
	SlowWindow int              `json:"slowWindow"`
	FastWindow int              `json:"fastWindow"`
	OpenBelow  fixedpoint.Value `json:"openBelow"`
	CloseAbove fixedpoint.Value `json:"closeAbove"`

	lastKLine types.KLine

	bbgo.OpenPositionOptions
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	return nil
}

func (s *Strategy) InstanceID() string {
	return instanceid.Emacross(s.Symbol, string(s.Interval), s.FastWindow, s.SlowWindow)
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())

	fastEMA := session.Indicators(s.Symbol).EWMA(types.IntervalWindow{Interval: s.Interval, Window: s.FastWindow})
	slowEMA := session.Indicators(s.Symbol).EWMA(types.IntervalWindow{Interval: s.Interval, Window: s.SlowWindow})

	log.WithFields(logrus.Fields{
		"symbol":     s.Symbol,
		"interval":   s.Interval.String(),
		"fastWindow": s.FastWindow,
		"slowWindow": s.SlowWindow,
		"leverage":   s.Leverage.String(),
		"quantity":   s.Quantity.String(),
	}).Infof("strategy started")

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(k types.KLine) {
		s.lastKLine = k

		fastLen := fastEMA.Length()
		slowLen := slowEMA.Length()
		warmedUp := fastLen >= s.FastWindow && slowLen >= s.SlowWindow

		log.WithFields(logrus.Fields{
			"symbol":       k.Symbol,
			"interval":     s.Interval.String(),
			"open":         k.Open.String(),
			"high":         k.High.String(),
			"low":          k.Low.String(),
			"close":        k.Close.String(),
			"volume":       k.Volume.String(),
			"emaFast":      fastEMA.Last(0),
			"emaSlow":      slowEMA.Last(0),
			"emaFastLen":   fastLen,
			"emaSlowLen":   slowLen,
			"warmedUp":     warmedUp,
			"positionBase": s.Position.Base.String(),
			"positionCost": s.Position.AverageCost.String(),
			"startTime":    k.StartTime.Time().UTC().Format(time.RFC3339),
		}).Infof("kline closed")
	}))

	cross := indicatorv2.Cross(fastEMA, slowEMA)
	cross.OnUpdate(func(v float64) {
		crossType := indicatorv2.CrossType(v)
		crossName := "unknown"
		switch crossType {
		case indicatorv2.CrossOver:
			crossName = "crossOver"
		case indicatorv2.CrossUnder:
			crossName = "crossUnder"
		}

		logger := log.WithFields(logrus.Fields{
			"symbol":   s.Symbol,
			"interval": s.Interval.String(),
			"cross":    crossName,
			"emaFast":  fastEMA.Last(0),
			"emaSlow":  slowEMA.Last(0),
		})

		switch crossType {

		case indicatorv2.CrossOver:
			logger.Infof("EMA crossover detected, opening long position")

			if err := s.Strategy.OrderExecutor.GracefulCancel(ctx); err != nil {
				log.WithError(err).Errorf("unable to cancel order")
			}

			opts := s.OpenPositionOptions
			opts.Long = true
			if price, ok := session.LastPrice(s.Symbol); ok {
				opts.Price = price
			}

			opts.Tags = []string{"emaCrossOver"}

			orders, err := s.Strategy.OrderExecutor.OpenPosition(ctx, opts)
			if err != nil {
				logger.WithError(err).Errorf("unable to open position")
				return
			}
			logger.WithField("orders", len(orders)).Infof("opened long position")
		case indicatorv2.CrossUnder:
			logger.Infof("EMA crossunder detected, closing position")

			err := s.Strategy.OrderExecutor.ClosePosition(ctx, fixedpoint.One)
			if err != nil {
				logger.WithError(err).Errorf("unable to submit close position order")
				return
			}
			logger.Infof("submitted close position order")
		}
	})

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		bbgo.Sync(ctx, s)
	})

	return nil
}
