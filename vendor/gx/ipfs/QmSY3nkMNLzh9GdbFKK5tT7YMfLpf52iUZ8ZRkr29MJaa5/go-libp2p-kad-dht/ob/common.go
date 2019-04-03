package ob

import (
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"time"
)

/*
This package is a few small utilities used by OpenBazaar to modify the DHT.
DHT modifications are recorded in the comments here.
*/

// OpenBazaar: PointerAddrTTL is used by handlers.handleAddProvider to specify the time
// to hold on to the pointer addr
var PointerAddrTTL = time.Hour * 24 * 7

// OpenBazaar: Normal IPFS providers should drop out of the DHT after 24 hours as
// defined by ProvideValidity above. However, `pointers`, which are special providers
// used by the offline message system, should stick around for one week.
var PointerValidity = time.Hour * 24 * 7

// OpenBazaar: `Pointers`, which are special providers used by the OpenBazaar messaging system,
// are prefixed with this string.
const MagicProviderID string = "000000000000000000000000"

// OpenBazaar: IsPointer is used to check if a peer ID inside a provider object should be interpreted as a pointer
// This is used in handlers.handleAddProvider and ProviderManager.run()
func IsPointer(id peer.ID) bool {
	hexID := peer.IDHexEncode(id)
	return hexID[4:28] == MagicProviderID
}
