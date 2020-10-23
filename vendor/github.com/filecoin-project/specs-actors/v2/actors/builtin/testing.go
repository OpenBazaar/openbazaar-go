package builtin

import (
	"fmt"
)

// Accumulates a sequence of messages (e.g. validation failures).
type MessageAccumulator struct {
	// Accumulated messages.
	// This is a pointer to support accumulators derived from `WithPrefix()` accumulating to
	// the same underlying collection.
	msgs *[]string
	// Optional prefix to all new messages, e.g. describing higher level context.
	prefix string
}

// Returns a new accumulator backed by the same collection, that will prefix each new message with
// a formatted string.
func (ma *MessageAccumulator) WithPrefix(format string, args ...interface{}) *MessageAccumulator {
	ma.initialize()
	return &MessageAccumulator{
		msgs:   ma.msgs,
		prefix: ma.prefix + fmt.Sprintf(format, args...),
	}
}

func (ma *MessageAccumulator) IsEmpty() bool {
	return ma.msgs == nil || len(*ma.msgs) == 0
}

func (ma *MessageAccumulator) Messages() []string {
	if ma.msgs == nil {
		return nil
	}
	return (*ma.msgs)[:]
}

// Adds messages to the accumulator.
func (ma *MessageAccumulator) Add(msg string) {
	ma.initialize()
	*ma.msgs = append(*ma.msgs, ma.prefix+msg)
}

// Adds a message to the accumulator
func (ma *MessageAccumulator) Addf(format string, args ...interface{}) {
	ma.Add(fmt.Sprintf(format, args...))
}

// Adds messages from another accumulator to this one.
func (ma *MessageAccumulator) AddAll(other *MessageAccumulator) {
	if other.msgs == nil {
		return
	}
	for _, msg := range *other.msgs {
		ma.Add(msg)
	}
}

// Adds a message if predicate is false.
func (ma *MessageAccumulator) Require(predicate bool, msg string, args ...interface{}) {
	if !predicate {
		ma.Add(fmt.Sprintf(msg, args...))
	}
}

func (ma *MessageAccumulator) RequireNoError(err error, msg string, args ...interface{}) {
	if err != nil {
		msg = msg + ": %v"
		args = append(args, err)
		ma.Addf(msg, args...)
	}
}

func (ma *MessageAccumulator) initialize() {
	if ma.msgs == nil {
		ma.msgs = &[]string{}
	}
}
