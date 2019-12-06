package core

import (
	"io/ioutil"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
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
