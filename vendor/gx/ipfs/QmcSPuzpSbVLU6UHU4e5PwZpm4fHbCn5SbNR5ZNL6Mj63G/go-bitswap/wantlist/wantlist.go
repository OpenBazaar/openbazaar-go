// Package wantlist implements an object for bitswap that contains the keys
// that a given peer wants.
package wantlist

import (
	"sort"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
)

type SessionTrackedWantlist struct {
	set map[cid.Cid]*sessionTrackedEntry
}

type Wantlist struct {
	set map[cid.Cid]Entry
}

type Entry struct {
	Cid      cid.Cid
	Priority int
}

type sessionTrackedEntry struct {
	Entry
	sesTrk map[uint64]struct{}
}

// NewRefEntry creates a new reference tracked wantlist entry.
func NewRefEntry(c cid.Cid, p int) Entry {
	return Entry{
		Cid:      c,
		Priority: p,
	}
}

type entrySlice []Entry

func (es entrySlice) Len() int           { return len(es) }
func (es entrySlice) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es entrySlice) Less(i, j int) bool { return es[i].Priority > es[j].Priority }

func NewSessionTrackedWantlist() *SessionTrackedWantlist {
	return &SessionTrackedWantlist{
		set: make(map[cid.Cid]*sessionTrackedEntry),
	}
}

func New() *Wantlist {
	return &Wantlist{
		set: make(map[cid.Cid]Entry),
	}
}

// Add adds the given cid to the wantlist with the specified priority, governed
// by the session ID 'ses'.  if a cid is added under multiple session IDs, then
// it must be removed by each of those sessions before it is no longer 'in the
// wantlist'. Calls to Add are idempotent given the same arguments. Subsequent
// calls with different values for priority will not update the priority.
// TODO: think through priority changes here
// Add returns true if the cid did not exist in the wantlist before this call
// (even if it was under a different session).
func (w *SessionTrackedWantlist) Add(c cid.Cid, priority int, ses uint64) bool {

	if e, ok := w.set[c]; ok {
		e.sesTrk[ses] = struct{}{}
		return false
	}

	w.set[c] = &sessionTrackedEntry{
		Entry:  Entry{Cid: c, Priority: priority},
		sesTrk: map[uint64]struct{}{ses: {}},
	}

	return true
}

// AddEntry adds given Entry to the wantlist. For more information see Add method.
func (w *SessionTrackedWantlist) AddEntry(e Entry, ses uint64) bool {
	if ex, ok := w.set[e.Cid]; ok {
		ex.sesTrk[ses] = struct{}{}
		return false
	}
	w.set[e.Cid] = &sessionTrackedEntry{
		Entry:  e,
		sesTrk: map[uint64]struct{}{ses: {}},
	}
	return true
}

// Remove removes the given cid from being tracked by the given session.
// 'true' is returned if this call to Remove removed the final session ID
// tracking the cid. (meaning true will be returned iff this call caused the
// value of 'Contains(c)' to change from true to false)
func (w *SessionTrackedWantlist) Remove(c cid.Cid, ses uint64) bool {
	e, ok := w.set[c]
	if !ok {
		return false
	}

	delete(e.sesTrk, ses)
	if len(e.sesTrk) == 0 {
		delete(w.set, c)
		return true
	}
	return false
}

// Contains returns true if the given cid is in the wantlist tracked by one or
// more sessions.
func (w *SessionTrackedWantlist) Contains(k cid.Cid) (Entry, bool) {
	e, ok := w.set[k]
	if !ok {
		return Entry{}, false
	}
	return e.Entry, true
}

func (w *SessionTrackedWantlist) Entries() []Entry {
	es := make([]Entry, 0, len(w.set))
	for _, e := range w.set {
		es = append(es, e.Entry)
	}
	return es
}

func (w *SessionTrackedWantlist) SortedEntries() []Entry {
	es := w.Entries()
	sort.Sort(entrySlice(es))
	return es
}

func (w *SessionTrackedWantlist) Len() int {
	return len(w.set)
}

func (w *SessionTrackedWantlist) CopyWants(to *SessionTrackedWantlist) {
	for _, e := range w.set {
		for k := range e.sesTrk {
			to.AddEntry(e.Entry, k)
		}
	}
}

func (w *Wantlist) Len() int {
	return len(w.set)
}

func (w *Wantlist) Add(c cid.Cid, priority int) bool {
	if _, ok := w.set[c]; ok {
		return false
	}

	w.set[c] = Entry{
		Cid:      c,
		Priority: priority,
	}

	return true
}

func (w *Wantlist) AddEntry(e Entry) bool {
	if _, ok := w.set[e.Cid]; ok {
		return false
	}
	w.set[e.Cid] = e
	return true
}

func (w *Wantlist) Remove(c cid.Cid) bool {
	_, ok := w.set[c]
	if !ok {
		return false
	}

	delete(w.set, c)
	return true
}

func (w *Wantlist) Contains(c cid.Cid) (Entry, bool) {
	e, ok := w.set[c]
	return e, ok
}

func (w *Wantlist) Entries() []Entry {
	es := make([]Entry, 0, len(w.set))
	for _, e := range w.set {
		es = append(es, e)
	}
	return es
}

func (w *Wantlist) SortedEntries() []Entry {
	es := w.Entries()
	sort.Sort(entrySlice(es))
	return es
}
