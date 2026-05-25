package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipIfNoCGO(t *testing.T) {
	t.Helper()
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Skip("sqlite3 driver not available")
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skip("sqlite3 requires cgo, skipping")
	}
}

func TestKLineCacheKeyFields(t *testing.T) {
	k1 := klineCacheKey{exchange: "binance", symbol: "BTCUSDT", interval: "1m", startMs: 1000, endMs: 2000, limit: 100}
	k2 := klineCacheKey{exchange: "binance", symbol: "BTCUSDT", interval: "1m", startMs: 1000, endMs: 2000, limit: 100}
	k3 := klineCacheKey{exchange: "binance", symbol: "BTCUSDT", interval: "5m", startMs: 1000, endMs: 2000, limit: 100}
	assert.Equal(t, k1, k2)
	assert.NotEqual(t, k1, k3)
}

func TestKLineTTLByInterval(t *testing.T) {
	assert.Equal(t, 5*time.Second, klineTTL("1m"))
	assert.Equal(t, 15*time.Second, klineTTL("5m"))
	assert.Equal(t, 30*time.Second, klineTTL("15m"))
	assert.Equal(t, 60*time.Second, klineTTL("1h"))
	assert.Equal(t, 120*time.Second, klineTTL("4h"))
	assert.Equal(t, 300*time.Second, klineTTL("1d"))
	assert.Equal(t, 30*time.Second, klineTTL("3m"))
}

func TestMemoryCacheHitAndMiss(t *testing.T) {
	cache := NewKLineCache(nil)

	key := klineCacheKey{exchange: "binance", symbol: "BTCUSDT", interval: "1m", startMs: 1000, endMs: 2000, limit: 10}
	klines := []*pb.KLine{
		{Exchange: "binance", Symbol: "BTCUSDT", Open: "100", High: "110", Low: "99", Close: "105", Volume: "500", EndTime: 1500},
	}

	_, ok := cache.getMemory(key)
	assert.False(t, ok, "cache miss expected")

	cache.setMemory(key, klines, klineTTL("1m"))

	got, ok := cache.getMemory(key)
	assert.True(t, ok, "cache hit expected")
	assert.Len(t, got, 1)
	assert.Equal(t, "100", got[0].Open)
}

func TestMemoryCacheExpiry(t *testing.T) {
	cache := NewKLineCache(nil)

	key := klineCacheKey{exchange: "binance", symbol: "BTCUSDT", interval: "1m", startMs: 1000, endMs: 2000, limit: 10}
	klines := []*pb.KLine{{Exchange: "binance", Symbol: "BTCUSDT", Open: "100"}}

	cache.setMemory(key, klines, 1*time.Nanosecond)

	time.Sleep(10 * time.Millisecond)
	_, ok := cache.getMemory(key)
	assert.False(t, ok, "expired entry should be missed")
}

func TestMemoryCacheMaxSize(t *testing.T) {
	cache := NewKLineCache(nil)
	cache.maxSize = 3

	for i := 0; i < 5; i++ {
		key := klineCacheKey{exchange: "binance", symbol: "BTCUSDT", interval: "1m", startMs: int64(i * 1000), endMs: int64((i + 1) * 1000), limit: 10}
		cache.setMemory(key, []*pb.KLine{{Open: "100"}}, klineTTL("1m"))
	}

	cache.mu.RLock()
	count := len(cache.entries)
	cache.mu.RUnlock()
	assert.LessOrEqual(t, count, 3, "cache should evict entries beyond maxSize")
}

func TestSQLiteLayerQuery(t *testing.T) {
	skipIfNoCGO(t)
	db := sqlx.MustOpen("sqlite3", ":memory:")
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE binance_klines (
			gid INTEGER PRIMARY KEY AUTOINCREMENT,
			exchange VARCHAR(10) NOT NULL,
			start_time DATETIME(3) NOT NULL,
			end_time DATETIME(3) NOT NULL,
			interval VARCHAR(10) NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			open DECIMAL(16,8) NOT NULL DEFAULT 0,
			high DECIMAL(16,8) NOT NULL DEFAULT 0,
			low DECIMAL(16,8) NOT NULL DEFAULT 0,
			close DECIMAL(16,8) NOT NULL DEFAULT 0,
			volume DECIMAL(16,8) NOT NULL DEFAULT 0,
			quote_volume DECIMAL(16,8) NOT NULL DEFAULT 0,
			taker_buy_base_volume DECIMAL(16,8) NOT NULL DEFAULT 0,
			taker_buy_quote_volume DECIMAL(16,8) NOT NULL DEFAULT 0,
			closed BOOLEAN NOT NULL DEFAULT TRUE,
			last_trade_id INT NOT NULL DEFAULT 0,
			num_trades INT NOT NULL DEFAULT 0
		)
	`)
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Minute)
	for i := 0; i < 5; i++ {
		k := types.KLine{
			Exchange:  types.ExchangeName("binance"),
			Symbol:    "BTCUSDT",
			StartTime: types.Time(now.Add(time.Duration(i) * time.Minute)),
			EndTime:   types.Time(now.Add(time.Duration(i+1) * time.Minute).Add(-time.Millisecond)),
			Interval:  types.Interval1m,
			Open:      fixedpoint.NewFromFloat(100.0 + float64(i)),
			High:      fixedpoint.NewFromFloat(110.0 + float64(i)),
			Low:       fixedpoint.NewFromFloat(99.0 + float64(i)),
			Close:     fixedpoint.NewFromFloat(105.0 + float64(i)),
			Volume:    fixedpoint.NewFromFloat(500.0),
			Closed:    true,
		}
		_, err := db.NamedExec(
			`INSERT INTO binance_klines (exchange, start_time, end_time, interval, symbol, open, high, low, close, volume, quote_volume, closed)
			 VALUES (:exchange, :start_time, :end_time, :interval, :symbol, :open, :high, :low, :close, :volume, :quote_volume, :closed)`, k)
		require.NoError(t, err)
	}

	cache := NewKLineCache(db)

	klines, err := cache.querySQLite(context.Background(), "binance", "BTCUSDT", "1m", now, now.Add(5*time.Minute), 10)
	require.NoError(t, err)
	assert.Len(t, klines, 5, "should read 5 klines from SQLite")
	assert.Equal(t, fixedpoint.NewFromFloat(100.0), klines[0].Open)
	assert.Equal(t, fixedpoint.NewFromFloat(104.0), klines[4].Open)
}

func TestSQLiteLayerNoDB(t *testing.T) {
	cache := NewKLineCache(nil)
	klines, err := cache.querySQLite(context.Background(), "binance", "BTCUSDT", "1m", time.Now(), time.Now(), 10)
	assert.NoError(t, err)
	assert.Nil(t, klines, "should return nil when no DB configured")
}

func TestSQLiteLayerWrongExchange(t *testing.T) {
	skipIfNoCGO(t)
	db := sqlx.MustOpen("sqlite3", ":memory:")
	defer db.Close()

	cache := NewKLineCache(db)
	klines, err := cache.querySQLite(context.Background(), "binance", "BTCUSDT", "1m", time.Now(), time.Now(), 10)
	assert.NoError(t, err)
	assert.Nil(t, klines, "should return nil when table does not exist")
}

func TestSQLiteWriteBack(t *testing.T) {
	skipIfNoCGO(t)
	db := sqlx.MustOpen("sqlite3", ":memory:")
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE binance_klines (
			gid INTEGER PRIMARY KEY AUTOINCREMENT,
			exchange VARCHAR(10) NOT NULL,
			start_time DATETIME(3) NOT NULL,
			end_time DATETIME(3) NOT NULL,
			interval VARCHAR(10) NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			open DECIMAL(16,8) NOT NULL DEFAULT 0,
			high DECIMAL(16,8) NOT NULL DEFAULT 0,
			low DECIMAL(16,8) NOT NULL DEFAULT 0,
			close DECIMAL(16,8) NOT NULL DEFAULT 0,
			volume DECIMAL(16,8) NOT NULL DEFAULT 0,
			quote_volume DECIMAL(16,8) NOT NULL DEFAULT 0,
			taker_buy_base_volume DECIMAL(16,8) NOT NULL DEFAULT 0,
			taker_buy_quote_volume DECIMAL(16,8) NOT NULL DEFAULT 0,
			closed BOOLEAN NOT NULL DEFAULT TRUE,
			last_trade_id INT NOT NULL DEFAULT 0,
			num_trades INT NOT NULL DEFAULT 0
		)
	`)
	require.NoError(t, err)

	cache := NewKLineCache(db)

	now := time.Now().UTC().Truncate(time.Minute)
	klines := []types.KLine{
		{
			Exchange:  types.ExchangeName("binance"),
			Symbol:    "BTCUSDT",
			StartTime: types.Time(now),
			EndTime:   types.Time(now.Add(time.Minute).Add(-time.Millisecond)),
			Interval:  types.Interval1m,
			Open:      fixedpoint.NewFromFloat(100.0),
			High:      fixedpoint.NewFromFloat(110.0),
			Low:       fixedpoint.NewFromFloat(99.0),
			Close:     fixedpoint.NewFromFloat(105.0),
			Volume:    fixedpoint.NewFromFloat(500.0),
			Closed:    true,
		},
	}

	err = cache.writeSQLite(context.Background(), "binance", klines)
	require.NoError(t, err)

	result, err := cache.querySQLite(context.Background(), "binance", "BTCUSDT", "1m", now, now.Add(time.Minute), 10)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, fixedpoint.NewFromFloat(100.0), result[0].Open)
}

func TestKLineTableName(t *testing.T) {
	assert.Equal(t, "binance_klines", klineTableName("binance"))
	assert.Equal(t, "okex_klines", klineTableName("okex"))
	assert.Equal(t, "max_klines", klineTableName("max"))
}
