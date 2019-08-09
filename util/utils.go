package util

import (
	"strings"
)

// NormalizeAddress is used to strip the 0x prefix
func NormalizeAddress(addr string) string {
	return strings.Replace(addr, "0x", "", 1)
}

// AreAddressesEqual - check if addresses are equal after normalizing them
func AreAddressesEqual(addr1, addr2 string) bool {
	return NormalizeAddress(addr1) == NormalizeAddress(addr2)
}
