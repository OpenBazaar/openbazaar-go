package shared

import (
	"errors"

	"github.com/hannahhoward/go-pubsub"
)

// ReadyFunc is function that gets called once when an event is ready
type ReadyFunc func(error)

// ReadyDispatcher is just an pubsub dispatcher where the callback is ReadyFunc
func ReadyDispatcher(evt pubsub.Event, fn pubsub.SubscriberFn) error {
	migrateErr, ok := evt.(error)
	if !ok && evt != nil {
		return errors.New("wrong type of event")
	}
	cb, ok := fn.(ReadyFunc)
	if !ok {
		return errors.New("wrong type of event")
	}
	cb(migrateErr)
	return nil
}
