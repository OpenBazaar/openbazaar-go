package cache

import (
	"fmt"
	"sync"
)

type Cacher interface {
	Set(string, []byte) error
	Get(string) ([]byte, error)
}

func NewMockCacher() Cacher {
	return &exampleWithNoPersistence{
		kv: make(map[string][]byte),
	}
}

type exampleWithNoPersistence struct {
	lock sync.RWMutex
	kv   map[string][]byte
}

func (e *exampleWithNoPersistence) Set(key string, value []byte) error {
	e.lock.Lock()
	e.kv[key] = value
	e.lock.Unlock()
	return nil
}

func (e *exampleWithNoPersistence) Get(key string) ([]byte, error) {
	e.lock.RLock()
	value, ok := e.kv[key]
	e.lock.RUnlock()
	if !ok {
		return nil, fmt.Errorf("cached key not found")
	}
	return value, nil
}
