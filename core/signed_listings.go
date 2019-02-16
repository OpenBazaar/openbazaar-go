package core

import (
	"io/ioutil"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

func GetSignedListingFromPath(p string) (*pb.SignedListing, error) {
	file, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	sl := new(pb.SignedListing)
	err = jsonpb.UnmarshalString(string(file), sl)
	if err != nil {
		return nil, err
	}
	return sl, nil
}

func SetAcceptedCurrencies(sl *pb.SignedListing, currencies []string) {
	sl.Listing.Metadata.AcceptedCurrencies = currencies
}

func AssignMatchingCoupons(savedCoupons []repo.Coupon, sl *pb.SignedListing) error {
	for _, coupon := range sl.Listing.Coupons {
		for _, c := range savedCoupons {
			if coupon.GetHash() == c.Hash {
				coupon.Code = &pb.Listing_Coupon_DiscountCode{c.Code}
				break
			}
		}
	}
	return nil
}

func AssignMatchingQuantities(inventory map[int]int64, sl *pb.SignedListing) error {
	for variant, count := range inventory {
		for i, s := range sl.Listing.Item.Skus {
			if variant == i {
				s.Quantity = count
				break
			}
		}
	}
	return nil
}

func ApplyShippingOptions(sl *pb.SignedListing) error {
	for _, so := range sl.Listing.ShippingOptions {
		for _, ser := range so.Services {
			ser.AdditionalItemPrice = ser.Price
		}
	}
	return nil
}
