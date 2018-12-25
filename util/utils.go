package util

import (
	"strings"
)

func NormalizeAddress(addr string) string {
	return strings.Replace(addr, "0x", "", 1)
}

func AreAddressesEqual(addr1, addr2 string) bool {
	return NormalizeAddress(addr1) == NormalizeAddress(addr2)
}
