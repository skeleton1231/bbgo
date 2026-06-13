package types

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type FundingRate struct {
	Symbol      string
	FundingRate fixedpoint.Value
	FundingTime time.Time
	Time        time.Time
}

// FundingPayment records a single funding-rate settlement event.
// Signed: positive = received by holder, negative = paid.
type FundingPayment struct {
	Exchange ExchangeName     `json:"exchange" db:"exchange"`
	Symbol   string           `json:"symbol" db:"symbol"`
	Asset    string           `json:"asset" db:"asset"`
	Amount   fixedpoint.Value `json:"amount" db:"amount"`
	Rate     fixedpoint.Value `json:"rate" db:"rate"`
	Time     Time             `json:"time" db:"time"`
}
