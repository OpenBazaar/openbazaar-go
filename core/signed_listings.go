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

func SetAcceptedCurrencies(sl *pb.SignedListing, currencies []string) *pb.SignedListing {
	sl.Listing.Metadata.AcceptedCurrencies = currencies
	return sl
}

func AssignMatchingCoupons(savedCoupons []repo.Coupon, sl *pb.SignedListing) (*pb.SignedListing, error) {
	for _, coupon := range sl.Listing.Coupons {
		for _, c := range savedCoupons {
			if coupon.GetHash() == c.Hash {
				coupon.Code = &pb.Listing_Coupon_DiscountCode{c.Code}
				break
			}
		}
	}
	return sl, nil
}

func AssignMatchingQuantities(inventory map[int]int64, sl *pb.SignedListing) (*pb.SignedListing, error) {
	for variant, count := range inventory {
		for i, s := range sl.Listing.Item.Skus {
			if variant == i {
				s.Quantity = count
				break
			}
		}
	}
	return sl, nil
}

func ApplyCouponsToListing(n *OpenBazaarNode, sl *pb.SignedListing) (*pb.SignedListing, error) {
	savedCoupons, err := n.Datastore.Coupons().Get(sl.Listing.Slug)
	if err != nil {
		return nil, err
	}

	sl, err = AssignMatchingCoupons(savedCoupons, sl)
	if err != nil {
		return nil, err
	}

	return sl, nil
}

func ApplyShippingOptions(sl *pb.SignedListing) (*pb.SignedListing, error) {
	for _, so := range sl.Listing.ShippingOptions {
		for _, ser := range so.Services {
			ser.AdditionalItemPrice = ser.Price
		}
	}
	return sl, nil
}

func UpdateInventoryQuantities(n *OpenBazaarNode, sl *pb.SignedListing) (*pb.SignedListing, error) {
	inventory, err := n.Datastore.Inventory().Get(sl.Listing.Slug)
	if err != nil {
		return nil, err
	}

	// Build the inventory list
	sl, err = AssignMatchingQuantities(inventory, sl)
	if err != nil {
		return nil, err
	}

	return sl, nil
}
