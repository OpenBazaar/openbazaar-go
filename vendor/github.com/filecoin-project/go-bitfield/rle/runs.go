package rlepluslazy

import (
	"fmt"
	"math"

	"golang.org/x/xerrors"
)

func Or(a, b RunIterator) (RunIterator, error) {
	it := addIt{a: a, b: b}
	return &it, it.prep()
}

type addIt struct {
	a RunIterator
	b RunIterator

	next Run

	arun Run
	brun Run
}

func (it *addIt) prep() error {
	var err error

	fetch := func() error {
		if !it.arun.Valid() && it.a.HasNext() {
			it.arun, err = it.a.NextRun()
			if err != nil {
				return err
			}
		}

		if !it.brun.Valid() && it.b.HasNext() {
			it.brun, err = it.b.NextRun()
			if err != nil {
				return err
			}
		}
		return nil
	}

	if err := fetch(); err != nil {
		return err
	}

	// one is not valid
	if !it.arun.Valid() {
		it.next = it.brun
		it.brun.Len = 0
		return nil
	}

	if !it.brun.Valid() {
		it.next = it.arun
		it.arun.Len = 0
		return nil
	}

	if !it.arun.Val && !it.brun.Val {
		min := it.arun.Len
		if it.brun.Len < min {
			min = it.brun.Len
		}
		it.next = Run{Val: it.arun.Val, Len: min}
		it.arun.Len -= it.next.Len
		it.brun.Len -= it.next.Len

		if err := fetch(); err != nil {
			return err
		}
		trailingRun := func(r1, r2 Run) bool {
			return !r1.Valid() && r2.Val == it.next.Val
		}
		if trailingRun(it.arun, it.brun) || trailingRun(it.brun, it.arun) {
			it.next.Len += it.arun.Len
			it.next.Len += it.brun.Len
			it.arun.Len = 0
			it.brun.Len = 0
		}

		return nil
	}

	it.next = Run{Val: true}
	// different vals, 'true' wins
	for (it.arun.Val && it.arun.Valid()) || (it.brun.Val && it.brun.Valid()) {
		min := it.arun.Len
		if it.brun.Len < min && it.brun.Valid() || !it.arun.Valid() {
			min = it.brun.Len
		}
		it.next.Len += min
		if it.arun.Valid() {
			it.arun.Len -= min
		}
		if it.brun.Valid() {
			it.brun.Len -= min
		}
		if err := fetch(); err != nil {
			return err
		}
	}

	return nil
}

func (it *addIt) HasNext() bool {
	return it.next.Valid()
}

func (it *addIt) NextRun() (Run, error) {
	next := it.next
	return next, it.prep()
}

func Count(ri RunIterator) (uint64, error) {
	var length uint64
	var count uint64

	for ri.HasNext() {
		r, err := ri.NextRun()
		if err != nil {
			return 0, err
		}

		if math.MaxUint64-r.Len < length {
			return 0, xerrors.New("RLE+ overflows")
		}
		length += r.Len

		if r.Val {
			count += r.Len
		}
	}
	return count, nil
}

func IsSet(ri RunIterator, x uint64) (bool, error) {
	var i uint64
	for ri.HasNext() {
		r, err := ri.NextRun()
		if err != nil {
			return false, err
		}

		if i+r.Len > x {
			return r.Val, nil
		}

		i += r.Len
	}
	return false, nil
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

type andIter struct {
	a, b   RunIterator
	ar, br Run
}

func (ai *andIter) HasNext() bool {
	return (ai.ar.Valid() || ai.a.HasNext()) && (ai.br.Valid() || ai.b.HasNext())
}

func (ai *andIter) NextRun() (run Run, err error) {
	for {
		// Ensure we have two valid runs.
		if !ai.ar.Valid() {
			if !ai.a.HasNext() {
				break
			}
			ai.ar, err = ai.a.NextRun()
			if err != nil {
				return Run{}, err
			}
		}

		if !ai.br.Valid() {
			if !ai.b.HasNext() {
				break
			}
			ai.br, err = ai.b.NextRun()
			if err != nil {
				return Run{}, err
			}
		}

		// &&
		newVal := ai.ar.Val && ai.br.Val

		// Check to see if we have an ongoing run and if we've changed
		// value.
		if run.Len > 0 && run.Val != newVal {
			return run, nil
		}

		newLen := min(ai.ar.Len, ai.br.Len)

		run.Val = newVal
		run.Len += newLen
		ai.ar.Len -= newLen
		ai.br.Len -= newLen
	}

	if run.Valid() {
		return run, nil
	}

	return Run{}, fmt.Errorf("end of runs")
}

func And(a, b RunIterator) (RunIterator, error) {
	return &andIter{a: a, b: b}, nil
}

type RunSliceIterator struct {
	Runs []Run
	i    int
}

func (ri *RunSliceIterator) HasNext() bool {
	return ri.i < len(ri.Runs)
}

func (ri *RunSliceIterator) NextRun() (Run, error) {
	if ri.i >= len(ri.Runs) {
		return Run{}, fmt.Errorf("end of runs")
	}

	out := ri.Runs[ri.i]
	ri.i++
	return out, nil
}

type notIter struct {
	it RunIterator
}

func (ni *notIter) HasNext() bool {
	return true
}

func (ni *notIter) NextRun() (Run, error) {
	if !ni.it.HasNext() {
		return Run{
			Val: true,
			Len: 40_000_000_000_000, // close enough to infinity
		}, nil
	}

	nr, err := ni.it.NextRun()
	if err != nil {
		return Run{}, err
	}

	nr.Val = !nr.Val
	return nr, nil
}

func Subtract(a, b RunIterator) (RunIterator, error) {
	return And(a, &notIter{it: b})
}

type nextRun struct {
	set bool
	run Run
	err error
}

type peekIter struct {
	it    RunIterator
	stash nextRun
}

func (it *peekIter) HasNext() bool {
	if it.stash.set {
		return true
	}
	return it.it.HasNext()
}

func (it *peekIter) NextRun() (Run, error) {
	if it.stash.set {
		run := it.stash.run
		err := it.stash.err
		it.stash = nextRun{}
		return run, err
	}

	return it.it.NextRun()
}

func (it *peekIter) peek() (Run, error) {
	run, err := it.NextRun()
	it.put(run, err)
	return run, err
}

func (it *peekIter) put(run Run, err error) {
	it.stash = nextRun{
		set: true,
		run: run,
		err: err,
	}
}

// normIter trims the last run of 0s
type normIter struct {
	it *peekIter
}

func newNormIter(it RunIterator) *normIter {
	if nit, ok := it.(*normIter); ok {
		return nit
	}
	return &normIter{
		it: &peekIter{
			it: it,
		},
	}
}

func (it *normIter) HasNext() bool {
	if !it.it.HasNext() {
		return false
	}

	// check if this is the last run
	cur, err := it.it.NextRun()
	if err != nil {
		it.it.put(cur, err)
		return true
	}

	notLast := it.it.HasNext()
	it.it.put(cur, err)
	if notLast {
		return true
	}

	return cur.Val
}

func (it *normIter) NextRun() (Run, error) {
	return it.it.NextRun()
}

func LastIndex(iter RunIterator, val bool) (uint64, error) {
	var at uint64
	var max uint64
	for iter.HasNext() {
		r, err := iter.NextRun()
		if err != nil {
			return 0, err
		}

		at += r.Len

		if r.Val == val {
			max = at
		}
	}

	return max, nil
}

// Returns iterator with all bits up to the last bit set:
// in:  11100000111010001110000
// out: 1111111111111111111
func Fill(i RunIterator) (RunIterator, error) {
	max, err := LastIndex(i, true)
	if err != nil {
		return nil, err
	}

	var runs []Run
	if max > 0 {
		runs = append(runs, Run{
			Val: true,
			Len: max,
		})
	}

	return &RunSliceIterator{Runs: runs}, nil
}
