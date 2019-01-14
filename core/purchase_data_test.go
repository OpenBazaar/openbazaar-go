package core_test

import (
	"reflect"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/core"
	v "github.com/RussellLuo/validating"
)

func newPurchaseData() *core.PurchaseData {
	var purchaseItem = core.Item{
		ListingHash: "Qzhashvalue",
		Quantity:    1,
		Options:     []core.Option{},
		Shipping: core.ShippingOption{
			Name:    "Priority",
			Service: "FastRUS",
		},
		Memo:           "ASAP Plz",
		Coupons:        []string{},
		PaymentAddress: "1longaddresshash",
	}
	return &core.PurchaseData{
		ShipTo:               "A. Purchaser",
		Address:              "123 Hometown Street",
		City:                 "Nowheresville",
		State:                "AX",
		PostalCode:           "12345",
		CountryCode:          "US",
		AddressNotes:         "",
		Moderator:            "QmPeerID",
		Items:                []core.Item{purchaseItem},
		AlternateContactInfo: "",
		RefundAddress:        nil,
		PaymentCoin:          "BTC",
	}
}

func makeErrsMap(errs v.Errors) map[string]v.Error {
	if errs == nil {
		return nil
	}

	formatted := make(map[string]v.Error, len(errs))
	for _, err := range errs {
		formatted[err.Field()] = err
	}
	return formatted
}

func TestPurchaseDataValidate(t *testing.T) {
	var examples = []struct {
		subject func() *core.PurchaseData
		errs    v.Errors
	}{
		{
			subject: func() *core.PurchaseData {
				var s = newPurchaseData()
				s.ShipTo = ""
				return s
			},
			errs: v.NewErrors("shipTo", v.ErrInvalid, "is zero valued"),
		},
		{
			subject: func() *core.PurchaseData {
				var s = newPurchaseData()
				s.Address = ""
				return s
			},
			errs: v.NewErrors("address", v.ErrInvalid, "is zero valued"),
		},
	}
	for _, e := range examples {
		var (
			s    = e.subject()
			errs = v.Validate(s.Schema())
		)
		if !reflect.DeepEqual(makeErrsMap(errs), makeErrsMap(e.errs)) {
			t.Errorf("(%+v) != (%+v)", errs, e.errs)
		}
	}

}
