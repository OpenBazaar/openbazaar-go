package zcash

import (
	"bytes"
	"sort"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// Sort in-place sorts the inputs and outputs for BIP 69
func (t *Transaction) Sort() {
	sort.Sort(sortableInputSlice(t.Inputs))
	sort.Sort(sortableOutputSlice(t.Outputs))
}

type sortableInputSlice []Input

func (s sortableInputSlice) Len() int      { return len(s) }
func (s sortableInputSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Input comparison function. From BIP 69
// First sort based on input hash (reversed / rpc-style), then index.
func (s sortableInputSlice) Less(i, j int) bool {
	// Input hashes are the same, so compare the index.
	ihash := s[i].PreviousOutPoint.Hash
	jhash := s[j].PreviousOutPoint.Hash
	if ihash == jhash {
		return s[i].PreviousOutPoint.Index < s[j].PreviousOutPoint.Index
	}

	// At this point, the hashes are not equal, so reverse them to
	// big-endian and return the result of the comparison.
	const hashSize = chainhash.HashSize
	for b := 0; b < hashSize/2; b++ {
		ihash[b], ihash[hashSize-1-b] = ihash[hashSize-1-b], ihash[b]
		jhash[b], jhash[hashSize-1-b] = jhash[hashSize-1-b], jhash[b]
	}
	return bytes.Compare(ihash[:], jhash[:]) == -1
}

type sortableOutputSlice []Output

func (s sortableOutputSlice) Len() int      { return len(s) }
func (s sortableOutputSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Output comparison function. From BIP 69
// First sort based on amount (smallest first), then PkScript.
func (s sortableOutputSlice) Less(i, j int) bool {
	if s[i].Value == s[j].Value {
		return bytes.Compare(s[i].ScriptPubKey, s[j].ScriptPubKey) < 0
	}
	return s[i].Value < s[j].Value
}
