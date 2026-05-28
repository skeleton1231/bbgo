package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// QueryTickerRequest is a request for querying the current ticker of a symbol.
type QueryTickerRequest struct {
	Exchange string `protobuf:"bytes,1,opt,name=exchange,proto3" json:"exchange,omitempty"`
	Symbol   string `protobuf:"bytes,2,opt,name=symbol,proto3" json:"symbol,omitempty"`
}

func (x *QueryTickerRequest) Reset()         { *x = QueryTickerRequest{} }
func (x *QueryTickerRequest) String() string { return "QueryTickerRequest" }
func (x *QueryTickerRequest) ProtoMessage()  {}

// QueryTickerResponse contains the ticker result.
type QueryTickerResponse struct {
	Ticker *Ticker `protobuf:"bytes,1,opt,name=ticker,proto3" json:"ticker,omitempty"`
	Error  *Error  `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *QueryTickerResponse) Reset()         { *x = QueryTickerResponse{} }
func (x *QueryTickerResponse) String() string { return "QueryTickerResponse" }
func (x *QueryTickerResponse) ProtoMessage()  {}

// QueryTickersRequest is a request for querying tickers of multiple symbols.
type QueryTickersRequest struct {
	Exchange string   `protobuf:"bytes,1,opt,name=exchange,proto3" json:"exchange,omitempty"`
	Symbols  []string `protobuf:"bytes,2,rep,name=symbols,proto3" json:"symbols,omitempty"`
}

func (x *QueryTickersRequest) Reset()         { *x = QueryTickersRequest{} }
func (x *QueryTickersRequest) String() string { return "QueryTickersRequest" }
func (x *QueryTickersRequest) ProtoMessage()  {}

// QueryTickersResponse contains the tickers result.
type QueryTickersResponse struct {
	Tickers []*Ticker `protobuf:"bytes,1,rep,name=tickers,proto3" json:"tickers,omitempty"`
	Error   *Error    `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *QueryTickersResponse) Reset()         { *x = QueryTickersResponse{} }
func (x *QueryTickersResponse) String() string { return "QueryTickersResponse" }
func (x *QueryTickersResponse) ProtoMessage()  {}

// MarketDataQueryClient provides gRPC client methods for public data queries.
type MarketDataQueryClient struct {
	CC grpc.ClientConnInterface
}

func NewMarketDataQueryClient(cc grpc.ClientConnInterface) *MarketDataQueryClient {
	return &MarketDataQueryClient{CC: cc}
}

func (c *MarketDataQueryClient) QueryKLines(ctx context.Context, in *QueryKLinesRequest, opts ...grpc.CallOption) (*QueryKLinesResponse, error) {
	client := NewMarketDataServiceClient(c.CC)
	return client.QueryKLines(ctx, in, opts...)
}

func (c *MarketDataQueryClient) QueryTicker(ctx context.Context, in *QueryTickerRequest, opts ...grpc.CallOption) (*QueryTickerResponse, error) {
	out := new(QueryTickerResponse)
	err := c.CC.Invoke(ctx, "/bbgo.MarketDataService/QueryTicker", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *MarketDataQueryClient) QueryTickers(ctx context.Context, in *QueryTickersRequest, opts ...grpc.CallOption) (*QueryTickersResponse, error) {
	out := new(QueryTickersResponse)
	err := c.CC.Invoke(ctx, "/bbgo.MarketDataService/QueryTickers", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MarketDataQueryServer must be implemented by the server to handle query RPCs.
type MarketDataQueryServer interface {
	QueryTicker(context.Context, *QueryTickerRequest) (*QueryTickerResponse, error)
	QueryTickers(context.Context, *QueryTickersRequest) (*QueryTickersResponse, error)
}

// RegisterMarketDataQueryServer registers additional query handlers on an existing gRPC server.
func RegisterMarketDataQueryServer(s *grpc.Server, srv MarketDataQueryServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "bbgo.MarketDataService",
		HandlerType: (*MarketDataQueryServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "QueryTicker",
				Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
					in := new(QueryTickerRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return srv.(MarketDataQueryServer).QueryTicker(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/bbgo.MarketDataService/QueryTicker",
					}
					handler := func(ctx context.Context, req interface{}) (interface{}, error) {
						return srv.(MarketDataQueryServer).QueryTicker(ctx, req.(*QueryTickerRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
			{
				MethodName: "QueryTickers",
				Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
					in := new(QueryTickersRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return srv.(MarketDataQueryServer).QueryTickers(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/bbgo.MarketDataService/QueryTickers",
					}
					handler := func(ctx context.Context, req interface{}) (interface{}, error) {
						return srv.(MarketDataQueryServer).QueryTickers(ctx, req.(*QueryTickersRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "pkg/pb/query_ext.go",
	}, srv)
}

// UnimplementedMarketDataQueryServer returns unimplemented errors.
type UnimplementedMarketDataQueryServer struct{}

func (UnimplementedMarketDataQueryServer) QueryTicker(context.Context, *QueryTickerRequest) (*QueryTickerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method QueryTicker not implemented")
}
func (UnimplementedMarketDataQueryServer) QueryTickers(context.Context, *QueryTickersRequest) (*QueryTickersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method QueryTickers not implemented")
}
