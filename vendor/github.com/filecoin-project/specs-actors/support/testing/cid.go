package testing

import (
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

var DefaultHashFunction = uint64(mh.BLAKE2B_MIN + 31)
var DefaultCidBuilder = cid.V1Builder{Codec: cid.DagCBOR, MhType: DefaultHashFunction}

func MakeCID(input string) cid.Cid {
	c, err := DefaultCidBuilder.Sum([]byte(input))
	if err != nil {
		panic(err)
	}
	return c
}
