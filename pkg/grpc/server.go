package grpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
)

type TradingService struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedTradingServiceServer
}

func (s *TradingService) SubmitOrder(ctx context.Context, request *pb.SubmitOrderRequest) (*pb.SubmitOrderResponse, error) {
	sessionName := request.Session

	if len(sessionName) == 0 {
		return nil, fmt.Errorf("session name can not be empty")
	}

	session, ok := s.Environ.Session(sessionName)
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionName)
	}

	submitOrders := toSubmitOrders(request.SubmitOrders)
	for i := range submitOrders {
		if market, ok := session.Market(submitOrders[i].Symbol); ok {
			submitOrders[i].Market = market
		} else {
			log.Warnf("session %s market %s not found", sessionName, submitOrders[i].Symbol)
		}
	}

	// we will return this error later because some orders could be succeeded
	createdOrders, _, err := bbgo.BatchRetryPlaceOrder(ctx, session.Exchange, nil, nil, log.StandardLogger(), submitOrders...)

	// convert response
	resp := &pb.SubmitOrderResponse{
		Session: sessionName,
		Orders:  nil,
	}

	for _, createdOrder := range createdOrders {
		resp.Orders = append(resp.Orders, transOrder(session, createdOrder))
	}

	return resp, err
}

func (s *TradingService) CancelOrder(ctx context.Context, request *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	sessionName := request.Session

	if len(sessionName) == 0 {
		return nil, fmt.Errorf("session name can not be empty")
	}

	session, ok := s.Environ.Session(sessionName)
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionName)
	}

	uuidOrderID := ""
	orderID, err := strconv.ParseUint(request.OrderId, 10, 64)
	if err != nil {
		// TODO: validate uuid
		uuidOrderID = request.OrderId
	}

	err = session.Exchange.CancelOrders(ctx, types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: request.ClientOrderId,
		},
		OrderID: orderID,
		UUID:    uuidOrderID,
	})
	if err != nil {
		return nil, fmt.Errorf("cancel order %s: %w", request.OrderId, err)
	}

	resp := &pb.CancelOrderResponse{}
	return resp, nil
}

func (s *TradingService) QueryOrder(ctx context.Context, request *pb.QueryOrderRequest) (*pb.QueryOrderResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TradingService) QueryOrders(ctx context.Context, request *pb.QueryOrdersRequest) (*pb.QueryOrdersResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TradingService) QueryTrades(ctx context.Context, request *pb.QueryTradesRequest) (*pb.QueryTradesResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

type UserDataService struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	pb.UnimplementedUserDataServiceServer
}

func (s *UserDataService) Subscribe(request *pb.UserDataRequest, server pb.UserDataService_SubscribeServer) error {
	sessionName := request.Session

	if len(sessionName) == 0 {
		return fmt.Errorf("session name can not be empty")
	}

	session, ok := s.Environ.Session(sessionName)
	if !ok {
		return fmt.Errorf("session %s not found", sessionName)
	}

	userDataStream := session.Exchange.NewStream()
	userDataStream.OnOrderUpdate(func(order types.Order) {
		err := server.Send(&pb.UserData{
			Channel: pb.Channel_ORDER,
			Event:   pb.Event_UPDATE,
			Orders:  []*pb.Order{transOrder(session, order)},
		})
		if err != nil {
			log.WithError(err).Errorf("grpc: can not send user data")
		}
	})
	userDataStream.OnTradeUpdate(func(trade types.Trade) {
		err := server.Send(&pb.UserData{
			Channel: pb.Channel_TRADE,
			Event:   pb.Event_UPDATE,
			Trades:  []*pb.Trade{transTrade(session, trade)},
		})
		if err != nil {
			log.WithError(err).Errorf("grpc: can not send user data")
		}
	})

	balanceHandler := func(balances types.BalanceMap) {
		err := server.Send(&pb.UserData{
			Channel:  pb.Channel_BALANCE,
			Event:    pb.Event_UPDATE,
			Balances: transBalances(session, balances),
		})
		if err != nil {
			log.WithError(err).Errorf("grpc: can not send user data")
		}
	}
	userDataStream.OnBalanceUpdate(balanceHandler)
	userDataStream.OnBalanceSnapshot(balanceHandler)

	ctx := server.Context()

	balances, err := session.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	err = server.Send(&pb.UserData{
		Channel:  pb.Channel_BALANCE,
		Event:    pb.Event_SNAPSHOT,
		Balances: transBalances(session, balances),
	})
	if err != nil {
		log.WithError(err).Errorf("grpc: can not send user data")
	}

	go userDataStream.Connect(ctx)

	defer func() {
		if err := userDataStream.Close(); err != nil {
			log.WithError(err).Errorf("user data stream close error")
		}
	}()

	<-ctx.Done()
	return nil
}

type MarketDataService struct {
	Config  *bbgo.Config
	Environ *bbgo.Environment
	Trader  *bbgo.Trader

	broadcasters map[string]*SharedBroadcaster
	bcMu         sync.Mutex
	cache        *KLineCache

	pb.UnimplementedMarketDataServiceServer
}

func (s *MarketDataService) getOrCreateBroadcaster(session *bbgo.ExchangeSession) *SharedBroadcaster {
	s.bcMu.Lock()
	defer s.bcMu.Unlock()
	if s.broadcasters == nil {
		s.broadcasters = make(map[string]*SharedBroadcaster)
	}
	if bc, ok := s.broadcasters[session.Name]; ok {
		return bc
	}
	bc := NewSharedBroadcaster(session)
	s.broadcasters[session.Name] = bc
	return bc
}

func (s *MarketDataService) Subscribe(request *pb.SubscribeRequest, server pb.MarketDataService_SubscribeServer) error {
	exchangeSubscriptions := map[string][]types.Subscription{}
	for _, sub := range request.Subscriptions {
		session, ok := s.Environ.Session(sub.Exchange)
		if !ok {
			return fmt.Errorf("exchange %s not found", sub.Exchange)
		}
		ss, err := toSubscriptions(sub)
		if err != nil {
			return err
		}
		exchangeSubscriptions[session.Name] = append(exchangeSubscriptions[session.Name], ss)
	}

	type clientReg struct {
		bc *SharedBroadcaster
		id uint64
	}
	var regs []clientReg

	for sessionName, subs := range exchangeSubscriptions {
		session, ok := s.Environ.Session(sessionName)
		if !ok {
			log.Errorf("session %s not found", sessionName)
			continue
		}
		bc := s.getOrCreateBroadcaster(session)
		id, ch := bc.RegisterClient()
		regs = append(regs, clientReg{bc: bc, id: id})

		bc.AddSubscriptions(subs)
		log.Infof("shared broadcaster: client %d subscribed to %s (%d subs)", id, sessionName, len(subs))

		go func() {
			for msg := range ch {
				if err := server.Send(msg); err != nil {
					log.WithError(err).Error("grpc stream send error")
					return
				}
			}
		}()
	}

	defer func() {
		for _, r := range regs {
			r.bc.UnregisterClient(r.id)
		}
	}()

	ctx := server.Context()
	<-ctx.Done()
	return ctx.Err()
}

func (s *MarketDataService) QueryKLines(ctx context.Context, request *pb.QueryKLinesRequest) (*pb.QueryKLinesResponse, error) {
	exchangeName, err := types.ValidExchangeName(request.Exchange)
	if err != nil {
		return nil, err
	}

	cacheKey := klineCacheKey{
		exchange: request.Exchange,
		symbol:   request.Symbol,
		interval: request.Interval,
		startMs:  request.StartTime,
		endMs:    request.EndTime,
		limit:    int32(request.Limit),
	}

	// L1: memory cache
	if s.cache != nil {
		if cached, ok := s.cache.getMemory(cacheKey); ok {
			return &pb.QueryKLinesResponse{Klines: cached}, nil
		}
	}

	// L2: SQLite — only query when startTime is specified (needs a time range)
	if s.cache != nil && request.StartTime != 0 {
		startTime := time.Unix(request.StartTime, 0)
		endTime := time.Now()
		if request.EndTime != 0 {
			endTime = time.Unix(request.EndTime, 0)
		}

		sqliteKlines, err := s.cache.querySQLite(ctx, request.Exchange, request.Symbol, request.Interval, startTime, endTime, int(request.Limit))
		if err != nil {
			log.WithError(err).Debug("kline cache sqlite miss")
		} else if len(sqliteKlines) > 0 {
			var pbKlines []*pb.KLine
			for _, session := range s.Environ.Sessions() {
				if session.ExchangeName == exchangeName {
					for _, k := range sqliteKlines {
						pbKlines = append(pbKlines, transKLine(session, k))
					}
					break
				}
			}
			s.cache.setMemory(cacheKey, pbKlines, klineTTL(request.Interval))
			return &pb.QueryKLinesResponse{Klines: pbKlines}, nil
		}
	}

	// L3: exchange API
	for _, session := range s.Environ.Sessions() {
		if session.ExchangeName == exchangeName {
			response := &pb.QueryKLinesResponse{}

			endTime := time.Now()
			if request.EndTime != 0 {
				endTime = time.Unix(request.EndTime, 0)
			}

			options := types.KLineQueryOptions{
				Limit: int(request.Limit),
			}
			options.EndTime = &endTime

			if request.StartTime != 0 {
				st := time.Unix(request.StartTime, 0)
				options.StartTime = &st
			}

			klines, err := session.Exchange.QueryKLines(ctx, request.Symbol, types.Interval(request.Interval), options)
			if err != nil {
				return nil, err
			}

			for _, kline := range klines {
				response.Klines = append(response.Klines, transKLine(session, kline))
			}

			// Write back to L1 + L2
			if s.cache != nil {
				s.cache.setMemory(cacheKey, response.Klines, klineTTL(request.Interval))
				writeCtx, writeCancel := context.WithTimeout(context.Background(), 30*time.Second)
				go func() {
					defer writeCancel()
					defer func() {
						if r := recover(); r != nil {
							log.WithField("recover", r).Warn("kline cache: recovered from panic in sqlite write-back")
						}
					}()
					s.cache.writeSQLite(writeCtx, request.Exchange, klines)
				}()
			}

			return response, nil
		}
	}

	return nil, fmt.Errorf("exchange %s not found", request.Exchange)
}

func (s *MarketDataService) QueryTicker(ctx context.Context, request *pb.QueryTickerRequest) (*pb.QueryTickerResponse, error) {
	exchangeName, err := types.ValidExchangeName(request.Exchange)
	if err != nil {
		return nil, err
	}

	for _, session := range s.Environ.Sessions() {
		if session.ExchangeName == exchangeName {
			ticker, err := session.Exchange.QueryTicker(ctx, request.Symbol)
			if err != nil {
				return nil, err
			}
			return &pb.QueryTickerResponse{
				Ticker: &pb.Ticker{
					Exchange: request.Exchange,
					Symbol:   request.Symbol,
					Open:     ticker.Open.Float64(),
					High:     ticker.High.Float64(),
					Low:      ticker.Low.Float64(),
					Close:    ticker.Last.Float64(),
					Volume:   ticker.Volume.Float64(),
				},
			}, nil
		}
	}

	return nil, fmt.Errorf("exchange %s not found", request.Exchange)
}

func (s *MarketDataService) QueryTickers(ctx context.Context, request *pb.QueryTickersRequest) (*pb.QueryTickersResponse, error) {
	exchangeName, err := types.ValidExchangeName(request.Exchange)
	if err != nil {
		return nil, err
	}

	for _, session := range s.Environ.Sessions() {
		if session.ExchangeName == exchangeName {
			tickers, err := session.Exchange.QueryTickers(ctx, request.Symbols...)
			if err != nil {
				return nil, err
			}
			resp := &pb.QueryTickersResponse{}
			for symbol, t := range tickers {
				resp.Tickers = append(resp.Tickers, &pb.Ticker{
					Exchange: request.Exchange,
					Symbol:   symbol,
					Open:     t.Open.Float64(),
					High:     t.High.Float64(),
					Low:      t.Low.Float64(),
					Close:    t.Last.Float64(),
					Volume:   t.Volume.Float64(),
				})
			}
			return resp, nil
		}
	}

	return nil, fmt.Errorf("exchange %s not found", request.Exchange)
}

type Server struct {
	Config      *bbgo.Config
	Environ     *bbgo.Environment
	Trader      *bbgo.Trader
	KLineDBPath string
}

func (s *Server) ListenAndServe(bind string) error {
	conn, err := net.Listen("tcp", bind)
	if err != nil {
		return errors.Wrapf(err, "failed to bind network at %s", bind)
	}

	var klineCache *KLineCache
	if s.KLineDBPath != "" {
		db, err := sqlx.Open("sqlite3", s.KLineDBPath)
		if err != nil {
			log.WithError(err).Warn("kline cache: failed to open sqlite, running without disk cache")
		} else {
			klineCache = NewKLineCache(db)
			log.Infof("kline cache: enabled with db=%s", s.KLineDBPath)
		}
	} else {
		klineCache = NewKLineCache(nil)
	}

	var grpcServer = grpc.NewServer()
	pb.RegisterMarketDataServiceServer(grpcServer, &MarketDataService{
		Config:  s.Config,
		Environ: s.Environ,
		Trader:  s.Trader,
		cache:   klineCache,
	})

	pb.RegisterTradingServiceServer(grpcServer, &TradingService{
		Config:  s.Config,
		Environ: s.Environ,
		Trader:  s.Trader,
	})

	pb.RegisterUserDataServiceServer(grpcServer, &UserDataService{
		Config:  s.Config,
		Environ: s.Environ,
		Trader:  s.Trader,
	})

	reflection.Register(grpcServer)

	if err := grpcServer.Serve(conn); err != nil {
		return errors.Wrap(err, "failed to serve grpc connections")
	}

	return nil
}
