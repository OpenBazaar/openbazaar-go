package encoding

import (
	"sync"

	refmt "gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt"
	"gx/ipfs/QmdBzoMxsBpojBfN1cv5GnKtB7sfYBMoLH7p9qSyEVYXcu/refmt/obj/atlas"
)

// PooledCloner is a thread-safe pooled object cloner.
type PooledCloner struct {
	pool sync.Pool
}

// NewPooledCloner returns a PooledCloner with the given atlas. Do not copy
// after use.
func NewPooledCloner(atl atlas.Atlas) PooledCloner {
	return PooledCloner{
		pool: sync.Pool{
			New: func() interface{} {
				return refmt.NewCloner(atl)
			},
		},
	}
}

// Clone clones a into b using a cloner from the pool.
func (p *PooledCloner) Clone(a, b interface{}) error {
	c := p.pool.Get().(refmt.Cloner)
	err := c.Clone(a, b)
	p.pool.Put(c)
	return err
}
