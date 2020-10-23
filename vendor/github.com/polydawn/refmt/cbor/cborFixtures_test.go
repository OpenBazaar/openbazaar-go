package cbor

import (
	"bytes"
	"encoding/base64"
	"testing"

	. "github.com/warpfork/go-wish"

	"github.com/polydawn/refmt/tok/fixtures"
)

func Test(t *testing.T) {
	testBool(t)
	testString(t)
	testMap(t)
	testArray(t)
	testComposite(t)
	testNumber(t)
	testBytes(t)
	testTags(t)
}

func checkEncoding(t *testing.T, sequence fixtures.Sequence, expectSerial []byte, expectErr error) {
	t.Helper()
	outputBuf := &bytes.Buffer{}
	tokenSink := NewEncoder(outputBuf)

	// Run steps, advancing through the token sequence.
	//  If it stops early, just report how many steps in; we Wish on that value.
	//  If it doesn't stop in time, just report that bool; we Wish on that value.
	var nStep int
	var done bool
	var err error
	for _, tok := range sequence.Tokens {
		nStep++
		done, err = tokenSink.Step(&tok)
		if done || err != nil {
			break
		}
	}

	// Assert final result.
	Wish(t, done, ShouldEqual, true)
	Wish(t, nStep, ShouldEqual, len(sequence.Tokens))
	Wish(t, err, ShouldEqual, expectErr)
	Wish(t, outputBuf.Bytes(), ShouldEqual, expectSerial)
}

func checkDecoding(t *testing.T, expectSequence fixtures.Sequence, serial []byte, expectErr error) {
	t.Helper()
	inputBuf := bytes.NewBuffer(serial)
	tokenSrc := NewDecoder(DecodeOptions{}, inputBuf)

	// Run steps, advancing until the decoder reports it's done.
	//  If the decoder keeps yielding more tokens than we expect, that's fine...
	//  we just keep recording them, and we'll diff later.
	//  There's a cutoff when it overshoots by 10 tokens because generally
	//  that indicates we've found some sort of loop bug and 10 extra token
	//  yields is typically enough info to diagnose with.
	var nStep int
	var done bool
	var yield = make(fixtures.Tokens, len(expectSequence.Tokens)+10)
	var err error
	for ; nStep <= len(expectSequence.Tokens)+10; nStep++ {
		done, err = tokenSrc.Step(&yield[nStep])
		if done || err != nil {
			break
		}
	}
	nStep++
	yield = yield[:nStep]

	// Assert final result.
	Wish(t, done, ShouldEqual, true)
	Wish(t, nStep, ShouldEqual, len(expectSequence.Tokens))
	Wish(t, yield, ShouldEqual, expectSequence.Tokens)
	Wish(t, err, ShouldEqual, expectErr)
}

func bcat(bss ...[]byte) []byte {
	l := 0
	for _, bs := range bss {
		l += len(bs)
	}
	rbs := make([]byte, 0, l)
	for _, bs := range bss {
		rbs = append(rbs, bs...)
	}
	return rbs
}

func b(b byte) []byte { return []byte{b} }

func deB64(s string) []byte {
	bs, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return bs
}
