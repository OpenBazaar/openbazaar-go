// response.go - Generic response handler
//
// To the extent possible under law, Yawning Angel waived all copyright
// and related or neighboring rights to bulb, using the creative
// commons "cc0" public domain dedication. See LICENSE or
// <http://creativecommons.org/publicdomain/zero/1.0/> for full details.

package bulb

import (
	"log"
	"net/textproto"
	"strconv"
	"strings"
)

// Response is a response to a control port command, or an asyncrhonous event.
type Response struct {
	// Err is the status code and string representation associated with a
	// response.  Responses that have completed successfully will also have
	// Err set to indicate such.
	Err *textproto.Error

	// Reply is the text on the EndReplyLine of the response.
	Reply string

	// Data is the MidReplyLines/DataReplyLines of the response.  Dot encoded
	// data is "decoded" and presented as a single string (terminal ".CRLF"
	// removed, all intervening CRs stripped).
	Data []string

	// RawLines is all of the lines of a response, without CRLFs.
	RawLines []string
}

// IsOk returns true if the response status code indicates success or
// an asynchronous event.
func (r *Response) IsOk() bool {
	switch r.Err.Code {
	case StatusOk, StatusOkUnneccecary, StatusAsyncEvent:
		return true
	default:
		return false
	}
}

// IsAsync returns true if the response is an asyncrhonous event.
func (r *Response) IsAsync() bool {
	return r.Err.Code == StatusAsyncEvent
}

// ReadResponse returns the next response object. Calling this
// simultaniously with Read, Request, or StartAsyncReader will lead to
// undefined behavior
func (c *Conn) ReadResponse() (*Response, error) {
	var resp *Response
	var statusCode int
	for {
		line, err := c.conn.ReadLine()
		if err != nil {
			return nil, err
		}
		if c.debugLog {
			log.Printf("S: %s", line)
		}

		// Parse the line that was just read.
		if len(line) < 4 {
			return nil, newProtocolError("truncated response: '%s'", line)
		}
		if code, err := strconv.Atoi(line[0:3]); err != nil {
			return nil, newProtocolError("invalid status code: '%s'", line[0:3])
		} else if code < 100 {
			return nil, newProtocolError("invalid status code: '%s'", line[0:3])
		} else if resp == nil {
			resp = new(Response)
			statusCode = code
		} else if code != statusCode {
			// The status code should stay fixed for all lines of the
			// response, since events can't be interleaved with response
			// lines.
			return nil, newProtocolError("status code changed: %03d != %03d", code, statusCode)
		}
		if resp.RawLines == nil {
			resp.RawLines = make([]string, 0, 1)
		}

		if line[3] == ' ' {
			// Final line in the response.
			resp.Reply = line[4:]
			resp.Err = statusCodeToError(statusCode, resp.Reply)
			resp.RawLines = append(resp.RawLines, line)
			return resp, nil
		}

		if resp.Data == nil {
			resp.Data = make([]string, 0, 1)
		}
		switch line[3] {
		case '-':
			// Continuation, keep reading.
			resp.Data = append(resp.Data, line[4:])
			resp.RawLines = append(resp.RawLines, line)
		case '+':
			// A "dot-encoded" payload follows.
			resp.Data = append(resp.Data, line[4:])
			resp.RawLines = append(resp.RawLines, line)
			dotBody, err := c.conn.ReadDotBytes()
			if err != nil {
				return nil, err
			}
			if c.debugLog {
				log.Printf("S: [dot encoded data]")
			}
			resp.Data = append(resp.Data, strings.TrimRight(string(dotBody), "\n\r"))
			dotLines := strings.Split(string(dotBody), "\n")
			for _, dotLine := range dotLines[:len(dotLines)-1] {
				resp.RawLines = append(resp.RawLines, dotLine)
			}
			resp.RawLines = append(resp.RawLines, ".")
		default:
			return nil, newProtocolError("invalid separator: '%c'", line[3])
		}
	}
}
