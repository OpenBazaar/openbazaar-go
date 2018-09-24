package cache

import "fmt"

type Cacher interface {
	Set(string, []byte) error
	Get(string) ([]byte, error)
}

func NewMockCacher() Cacher {
	return exampleWithNoPersistence{
		kv: make(map[string][]byte),
	}
}

type exampleWithNoPersistence struct {
	kv map[string][]byte
}

func (e exampleWithNoPersistence) Set(key string, value []byte) error {
	e.kv[key] = value
	return nil
}

func (e exampleWithNoPersistence) Get(key string) ([]byte, error) {
	value, ok := e.kv[key]
	if !ok {
		return nil, fmt.Errorf("cached key not found")
	}
	return value, nil
}
