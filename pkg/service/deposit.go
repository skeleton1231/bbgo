package service

import (
	"context"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/c9s/bbgo/pkg/exchange"
	"github.com/c9s/bbgo/pkg/exchange/batch"
	"github.com/c9s/bbgo/pkg/types"
)

type DepositService struct {
	DB       *sqlx.DB
	Supabase *SupabaseService
}

// Sync syncs the withdraw records into db
func (s *DepositService) Sync(ctx context.Context, ex types.Exchange, startTime time.Time) error {
	if s.DB == nil {
		return nil
	}
	isMargin, isFutures, isIsolated, _ := exchange.GetSessionAttributes(ex)
	if isMargin || isFutures || isIsolated {
		// only works in spot
		return nil
	}

	transferApi, ok := ex.(types.ExchangeTransferService)
	if !ok {
		return nil
	}

	tasks := []SyncTask{
		{
			Type:   types.Deposit{},
			Select: SelectLastDeposits(ex.Name(), 100),
			BatchQuery: func(ctx context.Context, startTime, endTime time.Time) (interface{}, chan error) {
				query := &batch.DepositBatchQuery{
					ExchangeTransferService: transferApi,
				}
				return query.Query(ctx, "", startTime, endTime)
			},
			Time: func(obj interface{}) time.Time {
				return obj.(types.Deposit).Time.Time()
			},
			ID: func(obj interface{}) string {
				deposit := obj.(types.Deposit)
				return deposit.TransactionID
			},
			Filter: func(obj interface{}) bool {
				deposit := obj.(types.Deposit)
				return len(deposit.TransactionID) != 0
			},
			LogInsert: true,
		},
	}

	for _, sel := range tasks {
		if err := sel.execute(ctx, s.DB, startTime); err != nil {
			return err
		}
	}

	return nil
}

func (s *DepositService) Query(exchangeName types.ExchangeName) ([]types.Deposit, error) {
	if s.Supabase != nil {
		return s.Supabase.QueryDeposits(string(exchangeName))
	}
	args := map[string]interface{}{
		"exchange": exchangeName,
	}
	sql := "SELECT * FROM `deposits` WHERE `exchange` = :exchange ORDER BY `time` ASC"
	rows, err := s.DB.NamedQuery(sql, args)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	return s.scanRows(rows)
}

func (s *DepositService) scanRows(rows *sqlx.Rows) (deposits []types.Deposit, err error) {
	for rows.Next() {
		var deposit types.Deposit
		if err := rows.StructScan(&deposit); err != nil {
			return deposits, err
		}

		deposits = append(deposits, deposit)
	}

	return deposits, rows.Err()
}

func (s *DepositService) Insert(deposit types.Deposit) error {
	if s.Supabase != nil {
		return s.Supabase.InsertDeposit(deposit)
	}
	_, err := s.DB.NamedExec(`INSERT INTO deposits (exchange, asset, address, amount, txn_id, time)
		VALUES (:exchange, :asset, :address, :amount, :txn_id, :time)`, deposit)
	return err
}

func SelectLastDeposits(ex types.ExchangeName, limit uint64) sq.SelectBuilder {
	return sq.Select("*").
		From("deposits").
		Where(sq.And{
			sq.Eq{"exchange": ex},
		}).
		OrderBy("time DESC").
		Limit(limit)
}
