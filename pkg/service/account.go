package service

import (
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/multierr"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/asset"
)

type AccountService struct {
	DB          *sqlx.DB
	TablePrefix string
	UserID      string
}

func (s *AccountService) tableName(base string) string { return s.TablePrefix + base }

func NewAccountService(db *sqlx.DB) *AccountService {
	return &AccountService{DB: db}
}

func (s *AccountService) InsertAsset(
	time time.Time, session string, name types.ExchangeName, account string, isMargin bool, isIsolatedMargin bool,
	isolatedMarginSymbol string, assets asset.Map,
) error {
	var err error
	tableName := s.tableName("nav_history_details")

	for _, v := range assets {
		var _err error
		if s.DB != nil {
			if s.DB.DriverName() == "postgres" {
				_, _err = s.DB.Exec(`INSERT INTO "`+tableName+`" (
					session, exchange, subaccount, time, currency,
					net_asset_in_usd, net_asset_in_btc, balance, available, locked,
					borrowed, net_asset, price_in_usd,
					is_margin, is_isolated, isolated_symbol, user_id)
				values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
					session, name, account, time, v.Currency,
					v.NetAssetInUSD, v.NetAssetInBTC, v.Total, v.Available, v.Locked,
					v.Borrowed, v.NetAsset, v.PriceInUSD,
					isMargin, isIsolatedMargin, isolatedMarginSymbol, s.UserID)
			} else {
				_, _err = s.DB.Exec(`
					INSERT INTO `+tableName+` (
						session, exchange, subaccount, time, currency,
						net_asset_in_usd, net_asset_in_btc, balance, available, locked,
						borrowed, net_asset, price_in_usd,
						is_margin, is_isolated, isolated_symbol)
					values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);`,
					session, name, account, time, v.Currency,
					v.NetAssetInUSD, v.NetAssetInBTC, v.Total, v.Available, v.Locked,
					v.Borrowed, v.NetAsset, v.PriceInUSD,
					isMargin, isIsolatedMargin, isolatedMarginSymbol)
			}
		}
		err = multierr.Append(err, _err)
	}
	return err
}
