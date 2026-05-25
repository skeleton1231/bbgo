package grpc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/pb"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/jmoiron/sqlx"
)

type klineCacheKey struct {
	exchange string
	symbol   string
	interval string
	startMs  int64
	endMs    int64
	limit    int32
}

type klineCacheEntry struct {
	klines    []*pb.KLine
	expiresAt time.Time
}

type KLineCache struct {
	mu      sync.RWMutex
	entries map[klineCacheKey]*klineCacheEntry
	db      *sqlx.DB
	maxSize int
}

func NewKLineCache(db *sqlx.DB) *KLineCache {
	return &KLineCache{
		entries: make(map[klineCacheKey]*klineCacheEntry),
		db:      db,
		maxSize: 500,
	}
}

func klineTTL(interval string) time.Duration {
	switch types.Interval(interval) {
	case types.Interval1m:
		return 5 * time.Second
	case types.Interval5m:
		return 15 * time.Second
	case types.Interval15m:
		return 30 * time.Second
	case types.Interval1h:
		return 60 * time.Second
	case types.Interval4h:
		return 120 * time.Second
	case types.Interval1d:
		return 300 * time.Second
	default:
		return 30 * time.Second
	}
}

func klineTableName(exchange string) string {
	return strings.ToLower(exchange) + "_klines"
}

func (c *KLineCache) getMemory(key klineCacheKey) ([]*pb.KLine, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	out := make([]*pb.KLine, len(entry.klines))
	copy(out, entry.klines)
	return out, true
}

func (c *KLineCache) setMemory(key klineCacheKey, klines []*pb.KLine, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.maxSize {
		c.evictLocked()
	}
	cp := make([]*pb.KLine, len(klines))
	copy(cp, klines)
	c.entries[key] = &klineCacheEntry{klines: cp, expiresAt: time.Now().Add(ttl)}
}

func (c *KLineCache) evictLocked() {
	now := time.Now()
	for k, e := range c.entries {
		if e.expiresAt.Before(now) {
			delete(c.entries, k)
			return
		}
	}
	var oldest klineCacheKey
	oldestTime := time.Now().Add(1 << 62)
	for k, e := range c.entries {
		if e.expiresAt.Before(oldestTime) {
			oldestTime = e.expiresAt
			oldest = k
		}
	}
	delete(c.entries, oldest)
}

func (c *KLineCache) querySQLite(ctx context.Context, exchange, symbol, interval string, startTime, endTime time.Time, limit int) ([]types.KLine, error) {
	if c.db == nil {
		return nil, nil
	}
	tableName := klineTableName(exchange)
	query := fmt.Sprintf(
		"SELECT * FROM `%s` WHERE `exchange` = ? AND `symbol` = ? AND `interval` = ? AND `end_time` >= ? AND `start_time` <= ? ORDER BY `start_time` ASC LIMIT ?",
		tableName,
	)

	rows, err := c.db.QueryxContext(ctx, query, exchange, symbol, interval, startTime, endTime, limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("kline cache sqlite query: %w", err)
	}
	defer rows.Close()

	var klines []types.KLine
	for rows.Next() {
		var k types.KLine
		if err := rows.StructScan(&k); err != nil {
			return nil, fmt.Errorf("kline cache sqlite scan: %w", err)
		}
		klines = append(klines, k)
	}
	return klines, nil
}

func (c *KLineCache) writeSQLite(ctx context.Context, exchange string, klines []types.KLine) error {
	if c.db == nil || len(klines) == 0 {
		return nil
	}
	tableName := klineTableName(exchange)
	sql := fmt.Sprintf(
		"INSERT OR IGNORE INTO `%s` (`exchange`, `start_time`, `end_time`, `symbol`, `interval`, `open`, `high`, `low`, `close`, `closed`, `volume`, `quote_volume`, `taker_buy_base_volume`, `taker_buy_quote_volume`) VALUES (:exchange, :start_time, :end_time, :symbol, :interval, :open, :high, :low, :close, :closed, :volume, :quote_volume, :taker_buy_base_volume, :taker_buy_quote_volume)",
		tableName,
	)
	_, err := c.db.NamedExecContext(ctx, sql, klines)
	if err != nil {
		log.WithError(err).Warn("kline cache: failed to write back to sqlite")
	}
	return err
}
