package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_genOrderSQL(t *testing.T) {
	t.Run("accept empty options", func(t *testing.T) {
		o := QueryOrdersOptions{}
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", "orders", o))
	})

	t.Run("different ordering ", func(t *testing.T) {
		o := QueryOrdersOptions{}
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", "orders", o))
		o.Ordering = "ASC"
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid ASC LIMIT 500", genOrderSQL("sqlite", "orders", o))
		o.Ordering = "DESC"
		assert.Equal(t, "SELECT orders.*, IFNULL(SUM(t.price * t.quantity)/SUM(t.quantity), orders.price) AS average_price FROM orders LEFT JOIN trades AS t ON (t.order_id = orders.order_id) GROUP BY orders.gid  ORDER BY orders.gid DESC LIMIT 500", genOrderSQL("sqlite", "orders", o))
	})

	t.Run("postgres uses COALESCE", func(t *testing.T) {
		o := QueryOrdersOptions{}
		sql := genOrderSQL("postgres", "orders", o)
		assert.Contains(t, sql, "COALESCE(SUM(t.price * t.quantity)/NULLIF(SUM(t.quantity), 0), orders.price)")
		assert.Contains(t, sql, "FROM orders")
	})

	t.Run("prefixed table name", func(t *testing.T) {
		o := QueryOrdersOptions{}
		sql := genOrderSQL("postgres", "paper_orders", o)
		assert.Contains(t, sql, "FROM paper_orders")
	})
}
