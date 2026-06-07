package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradesOrdersStrategyInstanceId, down_main_tradesOrdersStrategyInstanceId)
}

func up_main_tradesOrdersStrategyInstanceId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades ADD COLUMN strategy_instance_id TEXT NOT NULL DEFAULT '';")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE orders ADD COLUMN strategy_instance_id TEXT NOT NULL DEFAULT '';")
	if err != nil {
		return err
	}
	return err
}

func down_main_tradesOrdersStrategyInstanceId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	return err
}
