package filecoin

import (
	"github.com/btcsuite/btcd/chaincfg"
	faddr "github.com/filecoin-project/go-address"
	"strings"
)

type FilecoinAddress struct {
	addr faddr.Address
}

func NewFilecoinAddress(addrStr string) (*FilecoinAddress, error) {
	addr, err := faddr.NewFromString(addrStr)
	if err != nil {
		return nil, err
	}
	return &FilecoinAddress{addr: addr}, nil
}

// String returns the string encoding of the transaction output
// destination.
//
// Please note that String differs subtly from EncodeAddress: String
// will return the value as a string without any conversion, while
// EncodeAddress may convert destination types (for example,
// converting pubkeys to P2PKH addresses) before encoding as a
// payment address string.
func (f *FilecoinAddress) String() string {
	return f.addr.String()
}

// EncodeAddress returns the string encoding of the payment address
// associated with the Address value.  See the comment on String
// for how this method differs from String.
func (f *FilecoinAddress) EncodeAddress() string {
	return f.addr.String()
}

// ScriptAddress returns the raw bytes of the address to be used
// when inserting the address into a txout's script.
func (f *FilecoinAddress) ScriptAddress() []byte {
	return nil
}

// IsForNet returns whether or not the address is associated with the
// passed bitcoin network.
func (f *FilecoinAddress) IsForNet(params *chaincfg.Params) bool {
	switch params.Name {
	case chaincfg.MainNetParams.Name:
		return strings.HasPrefix(f.addr.String(), faddr.MainnetPrefix)
	case chaincfg.TestNet3Params.Name:
		return strings.HasPrefix(f.addr.String(), faddr.TestnetPrefix)
	}
	return false
}
