package service

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func TestSupabaseDirectDBConnection(t *testing.T) {
	dsn := os.Getenv("SUPABASE_DB_URL")
	if dsn == "" {
		t.Skip("SUPABASE_DB_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Log("connected to Supabase PostgreSQL")

	// Test query: orders with JOIN trades (average_price)
	rows, err := db.Query(`
		SELECT o.order_id, o.symbol, o.side, o.price, o.status,
		       COALESCE(SUM(t.price::numeric * t.quantity::numeric) / NULLIF(SUM(t.quantity::numeric), 0), o.price::numeric) AS average_price
		FROM orders o
		LEFT JOIN trades t ON t.order_id = o.order_id AND t.user_id = o.user_id
		WHERE o.user_id IS NOT NULL
		GROUP BY o.order_id, o.symbol, o.side, o.price, o.status
		ORDER BY o.order_id DESC
		LIMIT 5
	`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var orderID, symbol, side, price, status string
		var avgPrice *string
		if err := rows.Scan(&orderID, &symbol, &side, &price, &status, &avgPrice); err != nil {
			t.Fatalf("scan: %v", err)
		}
		count++
		t.Logf("order_id=%s symbol=%s side=%s price=%s status=%s avgPrice=%v", orderID, symbol, side, price, status, avgPrice)
	}
	t.Logf("queried %d orders with JOIN trades", count)

	// Test paper_orders table
	var paperCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM paper_orders WHERE user_id IS NOT NULL`).Scan(&paperCount)
	if err != nil {
		t.Logf("paper_orders query failed (table may not exist): %v", err)
	} else {
		t.Logf("paper_orders count: %d", paperCount)
	}

	// Show PostgreSQL version
	var version string
	db.QueryRow("SELECT version()").Scan(&version)
	t.Logf("pg version: %s", version)
}

func TestSupabaseDirectDBViaDSN(t *testing.T) {
	dsn := os.Getenv("SUPABASE_DB_URL")
	if dsn == "" {
		t.Skip("SUPABASE_DB_URL not set")
	}

	// lib/pq requires sslmode for Supabase
	if dsn[len(dsn)-1] != '?' {
		dsn += "?sslmode=require"
	} else {
		dsn += "&sslmode=require"
	}

	t.Logf("connecting with sslmode=require")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping with sslmode: %v", err)
	}
	t.Log("connected with sslmode=require")

	rows, err := db.Query(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		ORDER BY table_name
		LIMIT 20
	`)
	if err != nil {
		t.Fatalf("schema query: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	t.Logf("tables: %v", tables)
}

func TestSupabaseInsertViaDirectSQL(t *testing.T) {
	dsn := os.Getenv("SUPABASE_DB_URL")
	if dsn == "" {
		t.Skip("SUPABASE_DB_URL not set")
	}
	dsn += "?sslmode=require"

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	testOrderID := fmt.Sprintf("test_connectivity_%d", os.Getpid())

	result, err := db.Exec(`
		INSERT INTO orders (user_id, order_id, symbol, side, price, quantity, status, order_type, exchange, created_at, updated_at)
		VALUES ('00000000-0000-0000-0000-000000000000', $1, 'TESTUSDT', 'BUY', '1.00', '1.00', 'NEW', 'LIMIT', 'binance', NOW(), NOW())
		ON CONFLICT (user_id, order_id) DO UPDATE SET status = EXCLUDED.status, updated_at = NOW()
	`, testOrderID)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	affected, _ := result.RowsAffected()
	t.Logf("upsert affected rows: %d", affected)

	db.Exec(`DELETE FROM orders WHERE user_id = '00000000-0000-0000-0000-000000000000' AND order_id = $1`, testOrderID)
	t.Log("cleanup done")
}
