-- !txn
-- +up
ALTER TABLE trades ADD COLUMN strategy_instance_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN strategy_instance_id VARCHAR(64) NOT NULL DEFAULT '';

-- +down
ALTER TABLE trades DROP COLUMN strategy_instance_id;
ALTER TABLE orders DROP COLUMN strategy_instance_id;
