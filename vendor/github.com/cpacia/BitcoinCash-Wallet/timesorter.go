// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bitcoincash

// timeSorter implements sort.Interface to allow a slice of timestamps to
// be sorted.
type timeSorter []int64

// Len returns the number of timestamps in the slice.  It is part of the
// sort.Interface implementation.
func (s timeSorter) Len() int {
	return len(s)
}

// Swap swaps the timestamps at the passed indices.  It is part of the
// sort.Interface implementation.
func (s timeSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the timstamp with index i should sort before the
// timestamp with index j.  It is part of the sort.Interface implementation.
func (s timeSorter) Less(i, j int) bool {
	return s[i] < s[j]
}

// timeSorter implements sort.Interface to allow a slice of block headers to
// be sorted by timestamp.
type blockSorter []StoredHeader

// Len returns the number of timestamps in the slice.  It is part of the
// sort.Interface implementation.
func (s blockSorter) Len() int {
	return len(s)
}

// Swap swaps the timestamps at the passed indices.  It is part of the
// sort.Interface implementation.
func (s blockSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the timstamp with index i should sort before the
// timestamp with index j.  It is part of the sort.Interface implementation.
func (s blockSorter) Less(i, j int) bool {
	if s[i].header.Timestamp.Before(s[j].header.Timestamp) || s[i].header.Timestamp.Equal(s[j].header.Timestamp) {
		return true
	}
	return false
}
