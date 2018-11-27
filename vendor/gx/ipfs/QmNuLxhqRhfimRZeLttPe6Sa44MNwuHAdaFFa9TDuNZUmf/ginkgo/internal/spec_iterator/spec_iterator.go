package spec_iterator

import (
	"errors"

	"gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo/internal/spec"
)

var ErrClosed = errors.New("no more specs to run")

type SpecIterator interface {
	Next() (*spec.Spec, error)
	NumberOfSpecsPriorToIteration() int
	NumberOfSpecsToProcessIfKnown() (int, bool)
	NumberOfSpecsThatWillBeRunIfKnown() (int, bool)
}

type Counter struct {
	Index int `json:"index"`
}
