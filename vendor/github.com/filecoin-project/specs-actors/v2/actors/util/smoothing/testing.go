package smoothing

import (
	"github.com/filecoin-project/go-state-types/big"
)

// Returns an estimate with position val and velocity 0
func TestingConstantEstimate(val big.Int) FilterEstimate {
	return NewEstimate(val, big.Zero())
}

// Returns and estimate with postion x and velocity v
func TestingEstimate(x, v big.Int) FilterEstimate {
	return NewEstimate(x, v)
}
