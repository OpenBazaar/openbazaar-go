package client

import (
	"errors"
	"sync"
	"time"

	"github.com/OpenBazaar/multiwallet/client/blockbook"
	"golang.org/x/net/proxy"
)

var maximumBackoff = 60 * time.Second

type healthState struct {
	lastFailedAt    time.Time
	backoffDuration time.Duration
}

func (h *healthState) markUnhealthy() {
	var now = time.Now()
	if now.Before(h.nextAvailable()) {
		// can't be unhealthy before it's available
		return
	}
	if now.Before(h.lastFailedAt.Add(5 * time.Minute)) {
		h.backoffDuration *= 2
		if h.backoffDuration > maximumBackoff {
			h.backoffDuration = maximumBackoff
		}
	} else {
		h.backoffDuration = 2 * time.Second
	}
	h.lastFailedAt = now
}

func (h *healthState) isHealthy() bool {
	return time.Now().After(h.nextAvailable())
}

func (h *healthState) nextAvailable() time.Time {
	return h.lastFailedAt.Add(h.backoffDuration)
}

const nilTarget = RotationTarget("")

type (
	RotationTarget  string
	rotationManager struct {
		clientCache   map[RotationTarget]*blockbook.BlockBookClient
		currentTarget RotationTarget
		targetHealth  map[RotationTarget]*healthState
		rotateLock    sync.RWMutex
		started       bool
	}
)

func newRotationManager(targets []string, proxyDialer proxy.Dialer) (*rotationManager, error) {
	var (
		targetHealth = make(map[RotationTarget]*healthState)
		clients      = make(map[RotationTarget]*blockbook.BlockBookClient)
	)
	for _, apiUrl := range targets {
		c, err := blockbook.NewBlockBookClient(apiUrl, proxyDialer)
		if err != nil {
			return nil, err
		}
		clients[RotationTarget(apiUrl)] = c
		targetHealth[RotationTarget(apiUrl)] = &healthState{}
	}
	m := &rotationManager{
		clientCache:   clients,
		currentTarget: nilTarget,
		targetHealth:  targetHealth,
	}
	return m, nil
}

// AcquireCurrent locks the current client for reading and returns a pointer to the current
// client. ReleaseCurrent is required at the end of using the active client to ensure rotation
// does not lock indefinitely.
func (r *rotationManager) AcquireCurrent() *blockbook.BlockBookClient {
	for {
		r.rLock()
		if client, ok := r.clientCache[r.currentTarget]; !ok {
			r.rUnlock()
			r.SelectNext()
			continue
		} else {
			return client
		}
	}
}

// AcquireCurrentWhenReady will block until the current client is ready for use. This method
// should always be used before the AcquireCurrent variety to minimize time within a read lock.
func (r *rotationManager) AcquireCurrentWhenReady() *blockbook.BlockBookClient {
	if r.started {
		return r.AcquireCurrent()
	}
	var t = time.NewTicker(1 * time.Second)
	defer t.Stop()
	for range t.C {
		if r.started {
			break
		}
	}
	return r.AcquireCurrent()
}

// ReleaseCurrent unlocks the current client for reading and cleans up outstanding resources as
// needed.
func (r *rotationManager) ReleaseCurrent() {
	r.rUnlock()
}

// CloseCurrent locks the client for changing which prevents further read locks from being accessed.
func (r *rotationManager) CloseCurrent() {
	r.lock()
	defer r.unlock()

	if r.currentTarget != nilTarget {
		if r.started {
			r.clientCache[r.currentTarget].Close()
		}
		r.started = false
		r.currentTarget = nilTarget
	}
}

// StartCurrent unlocks the client for use after successfully starting the active client.
func (r *rotationManager) StartCurrent(done chan<- error) error {
	r.lock()
	defer r.unlock()

	client, ok := r.clientCache[r.currentTarget]
	if !ok {
		// ensure this isn't the result of r.currentTarget being nilTarget
		r.unlock()
		r.SelectNext()
		r.lock()
		client, ok = r.clientCache[r.currentTarget]
		if !ok {
			return errors.New("current client unavailable")
		}
	}

	if err := client.Start(done); err != nil {
		return err
	}

	r.started = true
	return nil
}

// FailCurrent marks the current client as having failed and ensures it is not rotated into too soon.
func (r *rotationManager) FailCurrent() {
	r.lock()
	defer r.unlock()

	hs, ok := r.targetHealth[r.currentTarget]
	if ok {
		hs.markUnhealthy()
	}
}

// SelectNext finds the next healthy and available server to activate with StartCurrent. This call will
// block until a server is healthy and available.
func (r *rotationManager) SelectNext() {
	r.lock()
	defer r.unlock()

	if r.currentTarget == nilTarget {
		var nextAvailableAt time.Time
		for {
			if time.Now().Before(nextAvailableAt) {
				continue
			}
			for target, health := range r.targetHealth {
				if health.isHealthy() {
					r.currentTarget = target
					return
				}
				if health.nextAvailable().After(nextAvailableAt) {
					nextAvailableAt = health.nextAvailable()
				}
			}
		}
	}
}

func (r *rotationManager) lock() {
	r.rotateLock.Lock()
}

func (r *rotationManager) unlock() {
	r.rotateLock.Unlock()
}

func (r *rotationManager) rLock() {
	r.rotateLock.RLock()
}

func (r *rotationManager) rUnlock() {
	r.rotateLock.RUnlock()
}
