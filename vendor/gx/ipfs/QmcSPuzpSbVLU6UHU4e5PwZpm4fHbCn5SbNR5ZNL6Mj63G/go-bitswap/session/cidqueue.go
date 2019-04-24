package session

import cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"

type cidQueue struct {
	elems []cid.Cid
	eset  *cid.Set
}

func newCidQueue() *cidQueue {
	return &cidQueue{eset: cid.NewSet()}
}

func (cq *cidQueue) Pop() cid.Cid {
	for {
		if len(cq.elems) == 0 {
			return cid.Cid{}
		}

		out := cq.elems[0]
		cq.elems = cq.elems[1:]

		if cq.eset.Has(out) {
			cq.eset.Remove(out)
			return out
		}
	}
}

func (cq *cidQueue) Push(c cid.Cid) {
	if cq.eset.Visit(c) {
		cq.elems = append(cq.elems, c)
	}
}

func (cq *cidQueue) Remove(c cid.Cid) {
	cq.eset.Remove(c)
}

func (cq *cidQueue) Has(c cid.Cid) bool {
	return cq.eset.Has(c)
}

func (cq *cidQueue) Len() int {
	return cq.eset.Len()
}
