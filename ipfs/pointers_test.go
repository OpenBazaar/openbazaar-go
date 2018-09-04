package ipfs

import (
	"gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"
	"testing"
)

func TestCreatePointerKey(t *testing.T) {
	var (
		hash  = "QmWqVgN2CtEgWgSXTfWHDoEhXXf26oZdfehPsCCWLZ4BB6"
		tests = []struct {
			PrefixLen     int
			ExpectedValue string
		}{
			{2, "Qmc9UEWpojkGZLBmt38atvBBnBVBwgcYGbYwMPJ3qyGSE3"},
			{4, "QmZNXVKUxs9z8h2s65ahg7NdrLy5oZXh2BkHj1wMJB1Vk5"},
			{8, "QmczhV1WEiodc7rW4T16hYade4iqR9MziVSvkAo8KxHVMM"},
			{16, "Qmayzi2PkFKbLKhSqjawCndo8yh55MynKnrkZ1SLv8bKd2"},
			{32, "QmcuE8iD3RpURTXJ1sQBaWDrQEPa45AEyiuibsXYBWY3yk"},
			{64, "QmRMqdH41sSbsVU1LKaAqb6jssK4UDYmsb5D2RMC79hRBs"},
		}
	)

	mh, err := multihash.FromB58String(hash)
	if err != nil {
		t.Error(err)
	}
	for _, test := range tests {
		key := CreatePointerKey(mh, test.PrefixLen)
		if key.B58String() != test.ExpectedValue {
			t.Error("Returned incorrect pointer key")
		}
	}
}
