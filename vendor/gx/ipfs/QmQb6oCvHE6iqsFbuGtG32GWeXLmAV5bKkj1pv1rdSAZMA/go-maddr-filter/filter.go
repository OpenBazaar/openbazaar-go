package filter

import (
	"net"
	"sync"

	manet "gx/ipfs/QmT6Cp31887FpAc25z25YHgpFJohZedrYLWPPspRtj1Brp/go-multiaddr-net"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
)

type Filters struct {
	mu      sync.RWMutex
	filters map[string]*net.IPNet
}

func NewFilters() *Filters {
	return &Filters{
		filters: make(map[string]*net.IPNet),
	}
}

func (fs *Filters) AddDialFilter(f *net.IPNet) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.filters[f.String()] = f
}

func (f *Filters) AddrBlocked(a ma.Multiaddr) bool {
	maddr := ma.Split(a)
	if len(maddr) == 0 {
		return false
	}
	netaddr, err := manet.ToNetAddr(maddr[0])
	if err != nil {
		// if we cant parse it, its probably not blocked
		return false
	}
	netip := net.ParseIP(netaddr.String())
	if netip == nil {
		return false
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, ft := range f.filters {
		if ft.Contains(netip) {
			return true
		}
	}
	return false
}

func (f *Filters) Filters() []*net.IPNet {
	var out []*net.IPNet
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, ff := range f.filters {
		out = append(out, ff)
	}
	return out
}

func (f *Filters) Remove(ff *net.IPNet) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.filters, ff.String())
}
