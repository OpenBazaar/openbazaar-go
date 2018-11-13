package cid

import (
	"testing"
)

// BenchmarkCidMap_CidStr estimates how fast it is to insert primitives into a map
// keyed by CidStr (concretely).
//
// We do 100 insertions per benchmark run to make sure the map initialization
// doesn't dominate the results.
//
// Sample results on linux amd64 go1.11beta:
//
//   BenchmarkCidMap_CidStr-8          100000             16317 ns/op
//   BenchmarkCidMap_CidIface-8        100000             20516 ns/op
//
// With benchmem on:
//
//   BenchmarkCidMap_CidStr-8          100000             15579 ns/op           11223 B/op        207 allocs/op
//   BenchmarkCidMap_CidIface-8        100000             19500 ns/op           12824 B/op        307 allocs/op
//   BenchmarkCidMap_StrPlusHax-8      200000             10451 ns/op            7589 B/op        202 allocs/op
//
// We can see here that the impact of interface boxing is significant:
// it increases the time taken to do the inserts to 133%, largely because
// the implied `runtime.convT2E` calls cause another malloc each.
//
// There are also significant allocations in both cases because
// A) we cannot create a multihash without allocations since they are []byte;
// B) the map has to be grown several times;
// C) something I haven't quite put my finger on yet.
// Ideally we'd drive those down further as well.
//
// Pre-allocating the map reduces allocs by a very small percentage by *count*,
// but reduces the time taken by 66% overall (presumably because when a map
// re-arranges itself, it involves more or less an O(n) copy of the content
// in addition to the alloc itself).  This isn't topical to the question of
// whether or not interfaces are a good idea; just for contextualizing.
//
func BenchmarkCidMap_CidStr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mp := map[CidStr]int{}
		for x := 0; x < 100; x++ {
			mp[NewCidStr(0, uint64(x), []byte{})] = x
		}
	}
}

// BenchmarkCidMap_CidIface is in the family of BenchmarkCidMap_CidStr:
// it is identical except the map key type is declared as an interface
// (which forces all insertions to be boxed, changing performance).
func BenchmarkCidMap_CidIface(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mp := map[Cid]int{}
		for x := 0; x < 100; x++ {
			mp[NewCidStr(0, uint64(x), []byte{})] = x
		}
	}
}

// BenchmarkCidMap_CidStrAvoidMapGrowth is in the family of BenchmarkCidMap_CidStr:
// it is identical except the map is created with a size hint that removes
// some allocations (5, in practice, apparently).
func BenchmarkCidMap_CidStrAvoidMapGrowth(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mp := make(map[CidStr]int, 100)
		for x := 0; x < 100; x++ {
			mp[NewCidStr(0, uint64(x), []byte{})] = x
		}
	}
}
