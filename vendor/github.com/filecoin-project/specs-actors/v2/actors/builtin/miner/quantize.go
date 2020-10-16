package miner

import "github.com/filecoin-project/go-state-types/abi"

// A spec for quantization.
type QuantSpec struct {
	unit   abi.ChainEpoch // The unit of quantization
	offset abi.ChainEpoch // The offset from zero from which to base the modulus
}

func NewQuantSpec(unit, offset abi.ChainEpoch) QuantSpec {
	return QuantSpec{unit: unit, offset: offset}
}

func (q QuantSpec) QuantizeUp(e abi.ChainEpoch) abi.ChainEpoch {
	return quantizeUp(e, q.unit, q.offset)
}

var NoQuantization = NewQuantSpec(1, 0)

// Rounds e to the nearest exact multiple of the quantization unit offset by
// offsetSeed % unit, rounding up.
// This function is equivalent to `unit * ceil(e - (offsetSeed % unit) / unit) + (offsetSeed % unit)`
// with the variables/operations are over real numbers instead of ints.
// Precondition: unit >= 0 else behaviour is undefined
func quantizeUp(e abi.ChainEpoch, unit abi.ChainEpoch, offsetSeed abi.ChainEpoch) abi.ChainEpoch {
	offset := offsetSeed % unit

	remainder := (e - offset) % unit
	quotient := (e - offset) / unit
	// Don't round if epoch falls on a quantization epoch
	if remainder == 0 {
		return unit*quotient + offset
	}
	// Negative truncating division rounds up
	if e-offset < 0 {
		return unit*quotient + offset
	}
	return unit*(quotient+1) + offset

}
