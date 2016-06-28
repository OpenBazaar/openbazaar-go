package missinggo

import "sync"

// Events are threadsafe boolean flags that provide a channel that's closed
// when its true.
type Event struct {
	mu     sync.Mutex
	ch     chan struct{}
	closed bool
}

func (me *Event) lazyInit() {
	if me.ch == nil {
		me.ch = make(chan struct{})
	}
}

func (me *Event) C() <-chan struct{} {
	me.mu.Lock()
	me.lazyInit()
	ch := me.ch
	me.mu.Unlock()
	return ch
}

func (me *Event) Clear() {
	me.mu.Lock()
	defer me.mu.Unlock()
	me.lazyInit()
	if !me.closed {
		return
	}
	me.ch = make(chan struct{})
	me.closed = false
}

func (me *Event) Set() (first bool) {
	me.mu.Lock()
	defer me.mu.Unlock()
	me.lazyInit()
	if me.closed {
		return false
	}
	close(me.ch)
	me.closed = true
	return true
}

func (me *Event) IsSet() bool {
	me.mu.Lock()
	ch := me.ch
	me.mu.Unlock()
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func (me *Event) Wait() {
	<-me.C()
}
