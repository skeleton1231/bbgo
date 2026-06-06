-- !txn
-- +up
ALTER TABLE trades ADD COLUMN strategy_instance_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN strategy_instance_id TEXT NOT NULL DEFAULT '';

-- +down
-- SQLite doesn't support DROP COLUMN before 3.35.0
