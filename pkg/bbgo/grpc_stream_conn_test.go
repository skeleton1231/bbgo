package bbgo

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/c9s/bbgo/pkg/pb"
)

// closedTCPPort binds a listener on an ephemeral port and immediately closes
// it, yielding an address that refuses connections deterministically and fast
// (no OS-dependent filtering or timeouts). A grpc.ClientConn pointed at this
// address reports codes.Unavailable on RPCs, which is exactly what we need to
// distinguish "unreachable but open" from "closed".
func closedTCPPort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())
	return addr
}

// grpcQueryErr issues a QueryTicker unary RPC against conn with a short
// timeout and returns the resulting error. The error code distinguishes the
// two states this test cares about:
//   - codes.Unavailable -> the conn is open but the server is unreachable
//   - codes.Canceled ("grpc: the client connection is closing") -> the conn
//     has been Close()d and is permanently unusable
func grpcQueryErr(t *testing.T, conn *grpc.ClientConn) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := pb.NewMarketDataQueryClient(conn).QueryTicker(ctx, &pb.QueryTickerRequest{
		Exchange: "binance",
		Symbol:   "BTCUSDT",
	})
	return err
}

// TestGRPCStreamSwapConnPreservesBorrowedConn is the regression test for the
// per-minute "grpc: the client connection is closing" storm observed across
// every strategy container.
//
// SharedServiceSource hands the SAME *grpc.ClientConn to both the query proxy
// (grpcExchangeProxy) and the market-data stream (GRPCStream). When the
// stream's reconnectLoop swapped in a freshly-dialed connection it used to
// Close() the previous one — but that previous conn was the borrowed/shared
// conn, so every subsequent proxy QueryTicker/QueryKLines failed permanently
// with "the client connection is closing". The shared k-line cache was
// bypassed for the entire lifetime of every container.
//
// Fix contract: the stream must only Close() connections it dialed itself
// (ownsConn); a borrowed/shared conn is left untouched.
func TestGRPCStreamSwapConnPreservesBorrowedConn(t *testing.T) {
	// Borrowed conn — shared with the query proxy in production. The server
	// is unreachable but the conn itself is open/healthy.
	borrowed, err := grpc.NewClient(closedTCPPort(t),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = borrowed.Close() })

	// Precondition: an unreachable (but NOT closed) conn reports Unavailable.
	require.Equal(t, codes.Unavailable, status.Code(grpcQueryErr(t, borrowed)),
		"precondition: borrowed conn should be unreachable, not closed")

	s := NewGRPCStream(borrowed, "binance", "127.0.0.1:1")
	require.False(t, s.ownsConn, "constructor-supplied conn is borrowed, not owned")

	// Simulate reconnectLoop dialing its own conn and swapping it in.
	owned, err := grpc.NewClient(closedTCPPort(t),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	streamCtx, streamCancel := context.WithCancel(context.Background())
	s.swapConn(owned, streamCtx, streamCancel)

	// The borrowed/shared conn must still be OPEN: its RPCs still return
	// Unavailable (dead server), NOT "the client connection is closing".
	assert.Equal(t, codes.Unavailable, status.Code(grpcQueryErr(t, borrowed)),
		"borrowed conn must NOT be closed by stream reconnect — the query proxy still depends on it")

	// The stream now owns the conn it dialed.
	s.mu.Lock()
	ownsAfterSwap := s.ownsConn
	s.mu.Unlock()
	assert.True(t, ownsAfterSwap, "stream should own the conn it dialed during reconnect")

	// Closing the stream must release the OWNED conn (no leak) and must leave
	// the borrowed conn untouched.
	require.NoError(t, s.Close())
	assert.Equal(t, codes.Canceled, status.Code(grpcQueryErr(t, owned)),
		"owned conn should be closed when the stream closes")
	// Borrowed conn is still fine — and closing it now (via cleanup) is a no-op-ish.
	assert.Equal(t, codes.Unavailable, status.Code(grpcQueryErr(t, borrowed)),
		"borrowed conn must remain open after stream Close()")
}

// TestGRPCStreamSwapConnClosesPreviouslyOwnedConn ensures that on a SECOND
// reconnect the stream does close its own previously-dialed conn — otherwise
// every reconnect would leak a connection.
func TestGRPCStreamSwapConnClosesPreviouslyOwnedConn(t *testing.T) {
	s := NewGRPCStream(nil, "binance", "127.0.0.1:1")

	first, err := grpc.NewClient(closedTCPPort(t),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	ctxA, cancelA := context.WithCancel(context.Background())
	s.swapConn(first, ctxA, cancelA)

	// first is now owned; an RPC returns Unavailable (unreachable, not closed).
	require.Equal(t, codes.Unavailable, status.Code(grpcQueryErr(t, first)))

	second, err := grpc.NewClient(closedTCPPort(t),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = second.Close() })

	ctxB, cancelB := context.WithCancel(context.Background())
	s.swapConn(second, ctxB, cancelB)

	// After the second swap, the FIRST (owned) conn must be closed.
	assert.Equal(t, codes.Canceled, status.Code(grpcQueryErr(t, first)),
		"a previously-owned conn should be closed when swapped out, to avoid leaking")

	require.NoError(t, s.Close())
}

// TestGRPCStreamSwapConnRejectsInstallAfterClose covers the Close() vs
// reconnectLoop() race (review H1/H2): when the stream is already shutting
// down, a conn freshly dialed by an in-flight reconnect must NOT be installed
// — swapConn closes it itself, so it is never orphaned.
func TestGRPCStreamSwapConnRejectsInstallAfterClose(t *testing.T) {
	s := NewGRPCStream(nil, "binance", "127.0.0.1:1")
	require.NoError(t, s.Close()) // rootCtx now canceled

	fresh, err := grpc.NewClient(closedTCPPort(t),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	streamCtx, streamCancel := context.WithCancel(context.Background())
	s.swapConn(fresh, streamCtx, streamCancel)

	// fresh must have been closed by swapConn (not installed), since rootCtx is done.
	assert.Equal(t, codes.Canceled, status.Code(grpcQueryErr(t, fresh)),
		"fresh conn must be closed (not orphaned) when the stream is shutting down")

	// And nothing is left installed as the stream's conn.
	s.mu.Lock()
	connAfter := s.conn
	s.mu.Unlock()
	assert.Nil(t, connAfter, "no conn should be installed after Close()")
}

// TestGRPCStreamCloseIsIdempotent ensures Close can be called twice without
// double-closing or panicking (the owned conn is cleared on the first call).
func TestGRPCStreamCloseIsIdempotent(t *testing.T) {
	s := NewGRPCStream(nil, "binance", "127.0.0.1:1")
	require.NoError(t, s.Close())
	require.NoError(t, s.Close())

	// Close after a real owned conn is in place: closes once, second is a no-op.
	owned, err := grpc.NewClient(closedTCPPort(t),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	s2 := NewGRPCStream(nil, "binance", "127.0.0.1:1")
	s2.swapConn(owned, ctx, cancel)
	require.NoError(t, s2.Close())
	require.NoError(t, s2.Close()) // no panic, no double-close side effects
}
