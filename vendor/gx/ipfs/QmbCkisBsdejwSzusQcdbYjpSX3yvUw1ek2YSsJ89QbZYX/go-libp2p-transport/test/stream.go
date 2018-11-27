package utils

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	mrand "math/rand"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	smux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	tpt "gx/ipfs/QmbCkisBsdejwSzusQcdbYjpSX3yvUw1ek2YSsJ89QbZYX/go-libp2p-transport"
)

// VerboseDebugging can be set to true to enable verbose debug logging in the
// stream stress tests.
var VerboseDebugging = false

var randomness []byte

func init() {
	// read 1MB of randomness
	randomness = make([]byte, 1<<20)
	if _, err := crand.Read(randomness); err != nil {
		panic(err)
	}
}

type Options struct {
	connNum   int
	streamNum int
	msgNum    int
	msgMin    int
	msgMax    int
}

func randBuf(size int) []byte {
	n := len(randomness) - size
	if size < 1 {
		panic(fmt.Errorf("requested too large buffer (%d). max is %d", size, len(randomness)))
	}

	start := mrand.Intn(n)
	return randomness[start : start+size]
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		debug.PrintStack()
		// TODO: not safe to call in parallel
		t.Fatal(err)
	}
}

func debugLog(t *testing.T, s string, args ...interface{}) {
	if VerboseDebugging {
		t.Logf(s, args...)
	}
}

func echoStream(t *testing.T, s smux.Stream) {
	defer s.Close()
	// echo everything
	var err error
	if VerboseDebugging {
		t.Logf("accepted stream")
		_, err = io.Copy(&logWriter{t, s}, s)
		t.Log("closing stream")
	} else {
		_, err = io.Copy(s, s) // echo everything
	}
	if err != nil {
		t.Error(err)
	}
}

type logWriter struct {
	t *testing.T
	W io.Writer
}

func (lw *logWriter) Write(buf []byte) (int, error) {
	lw.t.Logf("logwriter: writing %d bytes", len(buf))
	return lw.W.Write(buf)
}

func goServe(t *testing.T, l tpt.Listener) (done func()) {
	closed := make(chan struct{}, 1)

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				select {
				case <-closed:
					return // closed naturally.
				default:
					checkErr(t, err)
				}
			}

			debugLog(t, "accepted connection")
			go func() {
				for {
					str, err := c.AcceptStream()
					if err != nil {
						break
					}
					go echoStream(t, str)
				}
			}()
		}
	}()

	return func() {
		closed <- struct{}{}
	}
}

func SubtestStress(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID, opt Options) {
	msgsize := 1 << 11
	errs := make(chan error, 0) // dont block anything.

	rateLimitN := 5000 // max of 5k funcs, because -race has 8k max.
	rateLimitChan := make(chan struct{}, rateLimitN)
	for i := 0; i < rateLimitN; i++ {
		rateLimitChan <- struct{}{}
	}

	rateLimit := func(f func()) {
		<-rateLimitChan
		f()
		rateLimitChan <- struct{}{}
	}

	writeStream := func(s smux.Stream, bufs chan<- []byte) {
		debugLog(t, "writeStream %p, %d msgNum", s, opt.msgNum)

		for i := 0; i < opt.msgNum; i++ {
			buf := randBuf(msgsize)
			bufs <- buf
			debugLog(t, "%p writing %d bytes (message %d/%d #%x)", s, len(buf), i, opt.msgNum, buf[:3])
			if _, err := s.Write(buf); err != nil {
				errs <- fmt.Errorf("s.Write(buf): %s", err)
				continue
			}
		}
	}

	readStream := func(s smux.Stream, bufs <-chan []byte) {
		debugLog(t, "readStream %p, %d msgNum", s, opt.msgNum)

		buf2 := make([]byte, msgsize)
		i := 0
		for buf1 := range bufs {
			i++
			debugLog(t, "%p reading %d bytes (message %d/%d #%x)", s, len(buf1), i-1, opt.msgNum, buf1[:3])

			if _, err := io.ReadFull(s, buf2); err != nil {
				errs <- fmt.Errorf("io.ReadFull(s, buf2): %s", err)
				debugLog(t, "%p failed to read %d bytes (message %d/%d #%x)", s, len(buf1), i-1, opt.msgNum, buf1[:3])
				continue
			}
			if !bytes.Equal(buf1, buf2) {
				errs <- fmt.Errorf("buffers not equal (%x != %x)", buf1[:3], buf2[:3])
			}
		}
	}

	openStreamAndRW := func(c smux.Conn) {
		debugLog(t, "openStreamAndRW %p, %d opt.msgNum", c, opt.msgNum)

		s, err := c.OpenStream()
		if err != nil {
			errs <- fmt.Errorf("Failed to create NewStream: %s", err)
			return
		}

		bufs := make(chan []byte, opt.msgNum)
		go func() {
			writeStream(s, bufs)
			close(bufs)
		}()

		readStream(s, bufs)
		s.Close()
	}

	openConnAndRW := func() {
		debugLog(t, "openConnAndRW")

		l, err := ta.Listen(maddr)
		checkErr(t, err)

		done := goServe(t, l)
		defer done()

		c, err := tb.Dial(context.Background(), l.Multiaddr(), peerA)
		checkErr(t, err)

		// serve the outgoing conn, because some muxers assume
		// that we _always_ call serve. (this is an error?)
		go func() {
			debugLog(t, "serving connection")
			for {
				str, err := c.AcceptStream()
				if err != nil {
					break
				}
				go echoStream(t, str)
			}
		}()

		var wg sync.WaitGroup
		for i := 0; i < opt.streamNum; i++ {
			wg.Add(1)
			go rateLimit(func() {
				defer wg.Done()
				openStreamAndRW(c)
			})
		}
		wg.Wait()
		c.Close()
	}

	openConnsAndRW := func() {
		debugLog(t, "openConnsAndRW, %d conns", opt.connNum)

		var wg sync.WaitGroup
		for i := 0; i < opt.connNum; i++ {
			wg.Add(1)
			go rateLimit(func() {
				defer wg.Done()
				openConnAndRW()
			})
		}
		wg.Wait()
	}

	go func() {
		openConnsAndRW()
		close(errs) // done
	}()

	for err := range errs {
		t.Error(err)
	}

}

func SubtestStreamOpenStress(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	l, err := ta.Listen(maddr)
	checkErr(t, err)
	defer l.Close()

	count := 10000
	go func() {
		c, err := l.Accept()
		checkErr(t, err)
		stress := func() {
			for i := 0; i < count; i++ {
				s, err := c.OpenStream()
				if err != nil {
					panic(err)
				}
				s.Close()
			}
		}

		go stress()
		go stress()
		go stress()
		go stress()
		go stress()
	}()

	b, err := tb.Dial(context.Background(), l.Multiaddr(), peerA)
	checkErr(t, err)

	time.Sleep(time.Millisecond * 50)

	recv := make(chan struct{})
	go func() {
		for {
			str, err := b.AcceptStream()
			if err != nil {
				break
			}
			go func() {
				recv <- struct{}{}
				str.Close()
			}()
		}
	}()

	limit := time.After(time.Second * 10)
	for i := 0; i < count*5; i++ {
		select {
		case <-recv:
		case <-limit:
			t.Fatal("timed out receiving streams")
		}
	}
}

func SubtestStreamReset(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	l, err := ta.Listen(maddr)
	checkErr(t, err)

	done := make(chan struct{}, 2)
	go func() {
		muxa, err := l.Accept()
		checkErr(t, err)

		s, err := muxa.OpenStream()
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Millisecond * 50)

		_, err = s.Write([]byte("foo"))
		if err == nil {
			t.Error("should have failed to write")
		}

		s.Close()
		done <- struct{}{}
	}()

	muxb, err := tb.Dial(context.Background(), l.Multiaddr(), peerA)
	checkErr(t, err)

	go func() {
		str, err := muxb.AcceptStream()
		checkErr(t, err)
		str.Reset()
		done <- struct{}{}
	}()

	<-done
	<-done
}

func SubtestStress1Conn1Stream1Msg(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	SubtestStress(t, ta, tb, maddr, peerA, Options{
		connNum:   1,
		streamNum: 1,
		msgNum:    1,
		msgMax:    100,
		msgMin:    100,
	})
}

func SubtestStress1Conn1Stream100Msg(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	SubtestStress(t, ta, tb, maddr, peerA, Options{
		connNum:   1,
		streamNum: 1,
		msgNum:    100,
		msgMax:    100,
		msgMin:    100,
	})
}

func SubtestStress1Conn100Stream100Msg(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	SubtestStress(t, ta, tb, maddr, peerA, Options{
		connNum:   1,
		streamNum: 100,
		msgNum:    100,
		msgMax:    100,
		msgMin:    100,
	})
}

func SubtestStress50Conn10Stream50Msg(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	SubtestStress(t, ta, tb, maddr, peerA, Options{
		connNum:   50,
		streamNum: 10,
		msgNum:    50,
		msgMax:    100,
		msgMin:    100,
	})
}

func SubtestStress1Conn1000Stream10Msg(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	SubtestStress(t, ta, tb, maddr, peerA, Options{
		connNum:   1,
		streamNum: 1000,
		msgNum:    10,
		msgMax:    100,
		msgMin:    100,
	})
}

func SubtestStress1Conn100Stream100Msg10MB(t *testing.T, ta, tb tpt.Transport, maddr ma.Multiaddr, peerA peer.ID) {
	SubtestStress(t, ta, tb, maddr, peerA, Options{
		connNum:   1,
		streamNum: 100,
		msgNum:    100,
		msgMax:    10000,
		msgMin:    1000,
	})
}
