package types

import "github.com/c9s/bbgo/pkg/fixedpoint"

// Spot position actions
const (
	PositionActionOpen   = "OPEN"
	PositionActionAdd    = "ADD"
	PositionActionReduce = "REDUCE"
	PositionActionClose  = "CLOSE"
)

// Futures position actions
const (
	PositionActionOpenLong        = "OPEN_LONG"
	PositionActionAddLong         = "ADD_LONG"
	PositionActionReduceLong      = "REDUCE_LONG"
	PositionActionCloseLong       = "CLOSE_LONG"
	PositionActionOpenShort       = "OPEN_SHORT"
	PositionActionAddShort        = "ADD_SHORT"
	PositionActionReduceShort     = "REDUCE_SHORT"
	PositionActionCloseShort      = "CLOSE_SHORT"
	PositionActionFlipLongToShort = "FLIP_LONG_TO_SHORT"
	PositionActionFlipShortToLong = "FLIP_SHORT_TO_LONG"
)

// ComputePositionAction determines the position action for a trade
// based on the current position state BEFORE the trade is applied.
// It does NOT modify the position.
func (p *Position) ComputePositionAction(td Trade) string {
	quantity := td.Quantity
	if quantity.IsZero() {
		return ""
	}

	p.Lock()
	defer p.Unlock()

	return p.computePositionActionUnlocked(td)
}

func (p *Position) computePositionActionUnlocked(td Trade) string {
	quantity := td.Quantity

	// Compute the resulting base after this trade
	var virtualBase = p.Base

	switch td.Side {
	case SideTypeBuy:
		virtualBase = virtualBase.Add(quantity)
	case SideTypeSell:
		virtualBase = virtualBase.Sub(quantity)
	}

	prevSign := p.Base.Sign()
	nextSign := virtualBase.Sign()

	if td.IsFutures {
		return computeFuturesAction(td.Side, prevSign, nextSign)
	}

	return computeSpotAction(td.Side, prevSign, nextSign)
}

func computeSpotAction(side SideType, prevSign, nextSign int) string {
	switch {
	case prevSign == 0 && nextSign > 0:
		return PositionActionOpen
	case prevSign > 0 && nextSign == 0:
		return PositionActionClose
	case prevSign > 0 && nextSign > 0:
		if side == SideTypeBuy {
			return PositionActionAdd
		}
		return PositionActionReduce
	case prevSign < 0 && nextSign == 0:
		return PositionActionClose
	case prevSign < 0 && nextSign < 0:
		if side == SideTypeSell {
			return PositionActionAdd
		}
		return PositionActionReduce
	case prevSign == 0 && nextSign < 0:
		return PositionActionOpen
	default:
		return ""
	}
}

func computeFuturesAction(side SideType, prevSign, nextSign int) string {
	switch {
	// Flat -> Long
	case prevSign == 0 && nextSign > 0:
		return PositionActionOpenLong
	// Flat -> Short
	case prevSign == 0 && nextSign < 0:
		return PositionActionOpenShort
	// Long -> Flat
	case prevSign > 0 && nextSign == 0:
		return PositionActionCloseLong
	// Long -> Long (still long after trade)
	case prevSign > 0 && nextSign > 0:
		if side == SideTypeBuy {
			return PositionActionAddLong
		}
		return PositionActionReduceLong
	// Long -> Short (flip)
	case prevSign > 0 && nextSign < 0:
		return PositionActionFlipLongToShort
	// Short -> Flat
	case prevSign < 0 && nextSign == 0:
		return PositionActionCloseShort
	// Short -> Short (still short after trade)
	case prevSign < 0 && nextSign < 0:
		if side == SideTypeSell {
			return PositionActionAddShort
		}
		return PositionActionReduceShort
	// Short -> Long (flip)
	case prevSign < 0 && nextSign > 0:
		return PositionActionFlipShortToLong
	default:
		return ""
	}
}

// ComputePositionActionFromState computes the position action from raw state values.
// Used by the paper trade engine which tracks position state independently.
// currentBase is the position base BEFORE the trade, side is the trade side,
// quantity is the trade quantity, and isFutures indicates futures mode.
func ComputePositionActionFromState(currentBase fixedpoint.Value, side SideType, quantity fixedpoint.Value, isFutures bool) string {
	if quantity.IsZero() {
		return ""
	}

	var virtualBase fixedpoint.Value
	switch side {
	case SideTypeBuy:
		virtualBase = currentBase.Add(quantity)
	case SideTypeSell:
		virtualBase = currentBase.Sub(quantity)
	}

	prevSign := currentBase.Sign()
	nextSign := virtualBase.Sign()

	if isFutures {
		return computeFuturesAction(side, prevSign, nextSign)
	}

	return computeSpotAction(side, prevSign, nextSign)
}
