package util

import wi "github.com/OpenBazaar/wallet-interface"

// TxnSorter implements sort.Interface to allow a slice of timestamps to
// be sorted.
type TxnSorter []wi.Txn

// Len returns the number of timestamps in the slice.  It is part of the
// sort.Interface implementation.
func (s TxnSorter) Len() int {
	return len(s)
}

// Swap swaps the timestamps at the passed indices.  It is part of the
// sort.Interface implementation.
func (s TxnSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the timstamp with index i should sort before the
// timestamp with index j.  It is part of the sort.Interface implementation.
func (s TxnSorter) Less(i, j int) bool {
	return s[i].Timestamp.Before(s[j].Timestamp)
}
