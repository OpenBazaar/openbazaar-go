package gosocketio

import (
	"errors"
	"sync"
)

var (
	ErrorWaiterNotFound = errors.New("Waiter not found")
)

/**
Processes functions that require answers, also known as acknowledge or ack
*/
type ackProcessor struct {
	counter     int
	counterLock sync.Mutex

	resultWaiters     map[int](chan string)
	resultWaitersLock sync.RWMutex
}

/**
get next id of ack call
*/
func (a *ackProcessor) getNextId() int {
	a.counterLock.Lock()
	defer a.counterLock.Unlock()

	a.counter++
	return a.counter
}

/**
Just before the ack function called, the waiter should be added
to wait and receive response to ack call
*/
func (a *ackProcessor) addWaiter(id int, w chan string) {
	a.resultWaitersLock.Lock()
	a.resultWaiters[id] = w
	a.resultWaitersLock.Unlock()
}

/**
removes waiter that is unnecessary anymore
*/
func (a *ackProcessor) removeWaiter(id int) {
	a.resultWaitersLock.Lock()
	delete(a.resultWaiters, id)
	a.resultWaitersLock.Unlock()
}

/**
check if waiter with given ack id is exists, and returns it
*/
func (a *ackProcessor) getWaiter(id int) (chan string, error) {
	a.resultWaitersLock.RLock()
	defer a.resultWaitersLock.RUnlock()

	if waiter, ok := a.resultWaiters[id]; ok {
		return waiter, nil
	}
	return nil, ErrorWaiterNotFound
}
