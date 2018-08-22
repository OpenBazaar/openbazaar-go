package repo

import "errors"

type PayoutRatio struct{ Buyer, Vendor float32 }

func (r PayoutRatio) Validate() error {
	if r.Buyer+r.Vendor != 100 {
		return errors.New("payout ratio does not sum to 100%")
	}
	if r.Buyer < 0 {
		return errors.New("buyer percentage is negative")
	}
	if r.Vendor < 0 {
		return errors.New("vendor percentage is negative")
	}
	return nil
}

func (r PayoutRatio) BuyerAny() bool       { return r.Buyer > 0 }
func (r PayoutRatio) VendorAny() bool      { return r.Vendor > 0 }
func (r PayoutRatio) BuyerMajority() bool  { return r.Buyer > r.Vendor }
func (r PayoutRatio) VendorMajority() bool { return r.Vendor > r.Buyer }
func (r PayoutRatio) EvenMajority() bool   { return r.Vendor == r.Buyer }
