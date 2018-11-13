package loggabletracer

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	opentrace "gx/ipfs/QmWLWmRVSiagqP15jczsGME1qpob6HDbtbHAY2he9W5iUo/opentracing-go"
	writer "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log/writer"
)

func assertEqual(t *testing.T, expected interface{}, actual interface{}) {
	if expected != actual {
		t.Fatalf("%s != %s", expected, actual)
	}
}

func TestSpanRecorder(t *testing.T) {
	// Set up a writer to send spans to
	pr, pw := io.Pipe()
	writer.WriterGroup.AddWriter(pw)

	// create a span recorder
	recorder := NewLoggableRecorder()

	// generate a span
	var apiRecorder SpanRecorder = recorder
	rt := opentrace.Tags{
		"key": "value",
	}
	rs := RawSpan{
		Context:   SpanContext{},
		Operation: "test-span",
		Start:     time.Now(),
		Duration:  -1,
		Tags:      rt,
	}

	// record the span
	apiRecorder.RecordSpan(rs)

	// decode the LoggableSpan from
	var ls LoggableSpan
	evtDecoder := json.NewDecoder(pr)
	evtDecoder.Decode(&ls)

	// validate
	assertEqual(t, rs.Operation, ls.Operation)
	assertEqual(t, rs.Duration, ls.Duration)
	assertEqual(t, rs.Start.Nanosecond(), ls.Start.Nanosecond())
	assertEqual(t, rs.Tags["key"], ls.Tags["key"])

}
