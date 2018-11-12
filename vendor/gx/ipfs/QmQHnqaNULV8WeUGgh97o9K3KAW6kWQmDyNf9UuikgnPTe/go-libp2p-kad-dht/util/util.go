package util

import (
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"time"
)

/*
This package is a few small utilities used by OpenBazaar to modify the DHT.
DHT modifications are recorded in the comments here.
*/

// Used in ProviderManager.run(). Providers should only be deleted if !IsPointer && time>ProvideValidity
// or IsPointer && time>PointerValidity
var PointerValidity = time.Hour * 24 * 7

// Used by handlers.handleAddProvider to specify the time to hold on to the pointer addr
var PointerAddrTTL = time.Hour * 24 * 7

// Used to check if a peer ID inside a provider object should be interpreted as a pointer
// This is used in handlers.handleAddProvider and ProviderManager.run()
func IsPointer(id peer.ID) bool {
	hexID := peer.IDHexEncode(id)
	return hexID[4:28] == MAGIC
}

// Pointers are prefixed with this string
const MAGIC string = "000000000000000000000000"

// Max record age is increased to one week
const MaxRecordAge = time.Hour * 24 * 7

// Used in routing to specify the query size
var QuerySize = 16

// The provider manage in dht.go must use the non-gx package
