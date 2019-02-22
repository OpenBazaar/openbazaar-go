package cidutil

import (
	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mhash "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
)

// InlineBuilder is a cid.Builder that will use the id multihash when the
// size of the content is no more than limit
type InlineBuilder struct {
	cid.Builder     // Parent Builder
	Limit       int // Limit (inclusive)
}

// WithCodec implements the cid.Builder interface
func (p InlineBuilder) WithCodec(c uint64) cid.Builder {
	return InlineBuilder{p.Builder.WithCodec(c), p.Limit}
}

// Sum implements the cid.Builder interface
func (p InlineBuilder) Sum(data []byte) (cid.Cid, error) {
	if len(data) > p.Limit {
		return p.Builder.Sum(data)
	}
	return cid.V1Builder{Codec: p.GetCodec(), MhType: mhash.ID}.Sum(data)
}
