package log

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	tracer "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log/tracer"
	writer "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log/writer"
)

func assertEqual(t *testing.T, expected interface{}, actual interface{}) {
	if expected != actual {
		t.Fatalf("%s != %s", expected, actual)
	}
}

func assertNotZero(t *testing.T, a interface{}) {
	if a == 0 {
		t.Fatalf("%s = 0", a)
	}
}

// Test add remove writer
func TestChangeWriter(t *testing.T) {

	if writer.WriterGroup.Active() {
		panic("here")
	}

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	// start an event
	ctx1 := lgr.Start(ctx, "event1")

	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)

	ctx2 := lgr.Start(ctx1, "event2")

	lgr.Finish(ctx2)
	lgr.Finish(ctx1)

	// decode the log event
	var ls tracer.LoggableSpan
	evtDecoder := json.NewDecoder(lgs)
	evtDecoder.Decode(&ls)

	// event name and system should be
	assertEqual(t, "event2", ls.Operation)
	assertEqual(t, "test", ls.Tags["system"])
	// greater than zero should work for now
	assertNotZero(t, ls.Duration)
	assertNotZero(t, ls.Start)
	assertNotZero(t, ls.TraceID)
	assertNotZero(t, ls.SpanID)
}

func TestSingleEvent(t *testing.T) {
	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	// start an event
	ctx = lgr.Start(ctx, "event1")

	// finish the event
	lgr.Finish(ctx)

	// decode the log event
	var ls tracer.LoggableSpan
	evtDecoder := json.NewDecoder(lgs)
	evtDecoder.Decode(&ls)

	// event name and system should be
	assertEqual(t, "event1", ls.Operation)
	assertEqual(t, "test", ls.Tags["system"])
	// greater than zero should work for now
	assertNotZero(t, ls.Duration)
	assertNotZero(t, ls.Start)
	assertNotZero(t, ls.TraceID)
	assertNotZero(t, ls.SpanID)
}

func TestSingleEventWithErr(t *testing.T) {

	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	// start an event
	ctx = lgr.Start(ctx, "event1")

	// finish the event
	lgr.FinishWithErr(ctx, errors.New("rawer im an error"))

	// decode the log event
	var ls tracer.LoggableSpan
	evtDecoder := json.NewDecoder(lgs)
	evtDecoder.Decode(&ls)

	// event name and system should be
	assertEqual(t, "event1", ls.Operation)
	assertEqual(t, "test", ls.Tags["system"])
	assertEqual(t, true, ls.Tags["error"])
	assertEqual(t, ls.Logs[0].Field[0].Value, "rawer im an error")
	// greater than zero should work for now
	assertNotZero(t, ls.Duration)
	assertNotZero(t, ls.Start)
	assertNotZero(t, ls.TraceID)
	assertNotZero(t, ls.SpanID)
}

func TestEventWithTag(t *testing.T) {
	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	// start an event
	ctx = lgr.Start(ctx, "event1")
	lgr.SetTag(ctx, "tk", "tv")

	// finish the event
	lgr.Finish(ctx)

	// decode the log event
	var ls tracer.LoggableSpan
	evtDecoder := json.NewDecoder(lgs)
	evtDecoder.Decode(&ls)

	// event name and system should be
	assertEqual(t, "event1", ls.Operation)
	assertEqual(t, "test", ls.Tags["system"])
	assertEqual(t, "tv", ls.Tags["tk"])
	// greater than zero should work for now
	assertNotZero(t, ls.Duration)
	assertNotZero(t, ls.Start)
	assertNotZero(t, ls.TraceID)
	assertNotZero(t, ls.SpanID)
}

func TestEventWithTags(t *testing.T) {
	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	// start an event
	ctx = lgr.Start(ctx, "event1")
	lgr.SetTags(ctx, map[string]interface{}{
		"tk1": "tv1",
		"tk2": "tv2",
	})

	// finish the event
	lgr.Finish(ctx)

	// decode the log event
	var ls tracer.LoggableSpan
	evtDecoder := json.NewDecoder(lgs)
	evtDecoder.Decode(&ls)

	// event name and system should be
	assertEqual(t, "event1", ls.Operation)
	assertEqual(t, "test", ls.Tags["system"])
	assertEqual(t, "tv1", ls.Tags["tk1"])
	assertEqual(t, "tv2", ls.Tags["tk2"])
	// greater than zero should work for now
	assertNotZero(t, ls.Duration)
	assertNotZero(t, ls.Start)
	assertNotZero(t, ls.TraceID)
	assertNotZero(t, ls.SpanID)
}

func TestEventWithLogs(t *testing.T) {
	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	// start an event
	ctx = lgr.Start(ctx, "event1")
	lgr.LogKV(ctx, "log1", "logv1", "log2", "logv2")
	lgr.LogKV(ctx, "treeLog", []string{"Pine", "Juniper", "Spruce", "Ginkgo"})

	// finish the event
	lgr.Finish(ctx)

	// decode the log event
	var ls tracer.LoggableSpan
	evtDecoder := json.NewDecoder(lgs)
	evtDecoder.Decode(&ls)

	// event name and system should be
	assertEqual(t, "event1", ls.Operation)
	assertEqual(t, "test", ls.Tags["system"])

	assertEqual(t, "log1", ls.Logs[0].Field[0].Key)
	assertEqual(t, "logv1", ls.Logs[0].Field[0].Value)
	assertEqual(t, "log2", ls.Logs[0].Field[1].Key)
	assertEqual(t, "logv2", ls.Logs[0].Field[1].Value)

	// Should be a differnt log (different timestamp)
	assertEqual(t, "treeLog", ls.Logs[1].Field[0].Key)
	assertEqual(t, "[Pine Juniper Spruce Ginkgo]", ls.Logs[1].Field[0].Value)

	// greater than zero should work for now
	assertNotZero(t, ls.Duration)
	assertNotZero(t, ls.Start)
	assertNotZero(t, ls.TraceID)
	assertNotZero(t, ls.SpanID)
}

func TestMultiEvent(t *testing.T) {
	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)
	evtDecoder := json.NewDecoder(lgs)

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	ctx = lgr.Start(ctx, "root")

	doEvent(ctx, "e1", lgr)
	doEvent(ctx, "e2", lgr)

	lgr.Finish(ctx)

	e1 := getEvent(evtDecoder)
	assertEqual(t, "e1", e1.Operation)
	assertEqual(t, "test", e1.Tags["system"])
	assertNotZero(t, e1.Duration)
	assertNotZero(t, e1.Start)

	// I hope your clocks work...
	e2 := getEvent(evtDecoder)
	assertEqual(t, "e2", e2.Operation)
	assertEqual(t, "test", e2.Tags["system"])
	assertNotZero(t, e2.Duration)
	assertEqual(t, e1.TraceID, e2.TraceID)

	er := getEvent(evtDecoder)
	assertEqual(t, "root", er.Operation)
	assertEqual(t, "test", er.Tags["system"])
	assertNotZero(t, er.Start)
	assertNotZero(t, er.TraceID)
	assertNotZero(t, er.SpanID)

}

func TestEventSerialization(t *testing.T) {
	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)
	evtDecoder := json.NewDecoder(lgs)

	// create a logger
	lgr := Logger("test")

	// start an event
	sndctx := lgr.Start(context.Background(), "send")

	// **imagine** that we are putting `bc` (byte context) into a protobuf message
	// and send the message to another peer on the network
	bc, err := lgr.SerializeContext(sndctx)
	if err != nil {
		t.Fatal(err)
	}

	// now  **imagine** some peer getting a protobuf message and extracting
	// `bc` from the message to continue the operation
	rcvctx, err := lgr.StartFromParentState(context.Background(), "recv", bc)
	if err != nil {
		t.Fatal(err)
	}

	// at some point the sender completes their operation
	lgr.Finish(sndctx)
	e := getEvent(evtDecoder)
	assertEqual(t, "send", e.Operation)
	assertEqual(t, "test", e.Tags["system"])
	assertNotZero(t, e.Start)
	assertNotZero(t, e.Start)

	// and then the receiver finishes theirs
	lgr.Finish(rcvctx)
	e = getEvent(evtDecoder)
	assertEqual(t, "recv", e.Operation)
	assertEqual(t, "test", e.Tags["system"])
	assertNotZero(t, e.Start)
	assertNotZero(t, e.Duration)
	assertNotZero(t, e.TraceID)
	assertNotZero(t, e.SpanID)

}

func doEvent(ctx context.Context, name string, el EventLogger) context.Context {
	ctx = el.Start(ctx, name)
	defer func() {
		el.Finish(ctx)
	}()
	return ctx
}

func getEvent(ed *json.Decoder) tracer.LoggableSpan {
	// decode the log event
	var ls tracer.LoggableSpan
	ed.Decode(&ls)
	return ls
}

// DEPRECATED methods tested below
func TestEventBegin(t *testing.T) {

	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)
	evtDecoder := json.NewDecoder(lgs)

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	// start an event in progress with metadata
	eip := lgr.EventBegin(ctx, "event", LoggableMap{"key": "val"})

	// append more metadata
	eip.Append(LoggableMap{"foo": "bar"})

	// set an error
	eip.SetError(errors.New("gerrr im an error"))

	// finish the event
	eip.Done()

	// decode the log event
	var ls tracer.LoggableSpan
	evtDecoder.Decode(&ls)

	assertEqual(t, "event", ls.Operation)
	assertEqual(t, "test", ls.Tags["system"])
	assertEqual(t, ls.Logs[0].Field[0].Value, "val")
	assertEqual(t, ls.Logs[1].Field[0].Value, "bar")
	assertEqual(t, ls.Logs[2].Field[0].Value, "gerrr im an error")
	// greater than zero should work for now
	assertNotZero(t, ls.Duration)
	assertNotZero(t, ls.Start)
	assertNotZero(t, ls.SpanID)
	assertNotZero(t, ls.TraceID)
}

func TestEventBeginWithErr(t *testing.T) {

	// Set up a pipe to use as backend and log stream
	lgs, lgb := io.Pipe()
	// event logs will be written to lgb
	// event logs will be read from lgs
	writer.WriterGroup.AddWriter(lgb)
	evtDecoder := json.NewDecoder(lgs)

	// create a logger
	lgr := Logger("test")

	// create a root context
	ctx := context.Background()

	// start an event in progress with metadata
	eip := lgr.EventBegin(ctx, "event", LoggableMap{"key": "val"})

	// append more metadata
	eip.Append(LoggableMap{"foo": "bar"})

	// finish the event with an error
	eip.DoneWithErr(errors.New("gerrr im an error"))

	// decode the log event
	var ls tracer.LoggableSpan
	evtDecoder.Decode(&ls)

	assertEqual(t, "event", ls.Operation)
	assertEqual(t, "test", ls.Tags["system"])
	assertEqual(t, ls.Logs[0].Field[0].Value, "val")
	assertEqual(t, ls.Logs[1].Field[0].Value, "bar")
	assertEqual(t, ls.Logs[2].Field[0].Value, "gerrr im an error")
	// greater than zero should work for now
	assertNotZero(t, ls.Duration)
	assertNotZero(t, ls.Start)
	assertNotZero(t, ls.SpanID)
	assertNotZero(t, ls.TraceID)
}
