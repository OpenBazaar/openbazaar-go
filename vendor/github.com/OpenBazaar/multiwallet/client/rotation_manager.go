package client

import (
	"net/http"
	"net/url"
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
	reqFunc func(string, string, []byte, url.Values) (*http.Response, error)
)

func newRotationManager(targets []string, proxyDialer proxy.Dialer, doReq reqFunc) (*rotationManager, error) {
	var (
		targetHealth = make(map[RotationTarget]*healthState)
		clients      = make(map[RotationTarget]*blockbook.BlockBookClient)
	)
	for _, apiUrl := range targets {
		c, err := blockbook.NewBlockBookClient(apiUrl, proxyDialer)
		if err != nil {
			return nil, err
		}
		c.RequestFunc = doReq
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

func (r *rotationManager) AcquireCurrent() *blockbook.BlockBookClient {
	for {
		r.RLock()
		if client, ok := r.clientCache[r.currentTarget]; !ok {
			r.RUnlock()
			r.SelectNext()
			continue
		} else {
			return client
		}
	}
}

func (r *rotationManager) ReleaseCurrent() {
	r.RUnlock()
}

func (r *rotationManager) CloseCurrent() {
	r.Lock()
	defer r.Unlock()

	if r.currentTarget != nilTarget {
		if r.started {
			r.clientCache[r.currentTarget].Close()
		}
		r.currentTarget = nilTarget
	}
}

func (r *rotationManager) StartCurrent(done chan<- error) error {
	r.Lock()
	defer r.Unlock()

	if err := r.clientCache[r.currentTarget].Start(done); err != nil {
		return err
	}
	r.started = true
	return nil
}

func (r *rotationManager) FailCurrent() {
	r.Lock()
	defer r.Unlock()

	r.started = false
	r.targetHealth[r.currentTarget].markUnhealthy()
}

func (r *rotationManager) SelectNext() {
	r.Lock()
	defer r.Unlock()

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

func (r *rotationManager) Lock() {
	r.rotateLock.Lock()
}

func (r *rotationManager) Unlock() {
	r.rotateLock.Unlock()
}

func (r *rotationManager) RLock() {
	r.rotateLock.RLock()
}

func (r *rotationManager) RUnlock() {
	r.rotateLock.RUnlock()
}
