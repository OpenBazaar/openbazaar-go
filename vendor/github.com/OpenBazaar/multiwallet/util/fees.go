package util

import (
	"github.com/OpenBazaar/wallet-interface"
)

type FeeProvider struct {
	maxFee      uint64
	priorityFee uint64
	normalFee   uint64
	economicFee uint64
}

func NewFeeDefaultProvider(maxFee, priorityFee, normalFee, economicFee uint64) *FeeProvider {
	return &FeeProvider{
		maxFee:      maxFee,
		priorityFee: priorityFee,
		normalFee:   normalFee,
		economicFee: economicFee,
	}
}

func (fp *FeeProvider) GetFeePerByte(feeLevel wallet.FeeLevel) uint64 {
	switch feeLevel {
	case wallet.PRIOIRTY:
		return fp.priorityFee
	case wallet.NORMAL:
		return fp.normalFee
	case wallet.ECONOMIC:
		return fp.economicFee
	case wallet.FEE_BUMP:
		return fp.priorityFee * 2
	default:
		return fp.normalFee
	}
}
