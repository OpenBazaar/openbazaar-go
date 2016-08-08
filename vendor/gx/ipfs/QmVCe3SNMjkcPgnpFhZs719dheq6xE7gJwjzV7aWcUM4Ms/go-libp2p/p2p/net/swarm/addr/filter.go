package addrutil

import (
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	mafmt "gx/ipfs/QmeLQ13LftT9XhNn22piZc3GP56fGqhijuL5Y8KdUaRn1g/mafmt"
)

// SubtractFilter returns a filter func that filters all of the given addresses
func SubtractFilter(addrs ...ma.Multiaddr) func(ma.Multiaddr) bool {
	addrmap := make(map[string]bool)
	for _, a := range addrs {
		addrmap[string(a.Bytes())] = true
	}

	return func(a ma.Multiaddr) bool {
		return !addrmap[string(a.Bytes())]
	}
}

// IsFDCostlyTransport returns true for transports that require a new file
// descriptor per connection created
func IsFDCostlyTransport(a ma.Multiaddr) bool {
	return mafmt.TCP.Matches(a)
}

// FilterNeg returns a negated version of the passed in filter
func FilterNeg(f func(ma.Multiaddr) bool) func(ma.Multiaddr) bool {
	return func(a ma.Multiaddr) bool {
		return !f(a)
	}
}
