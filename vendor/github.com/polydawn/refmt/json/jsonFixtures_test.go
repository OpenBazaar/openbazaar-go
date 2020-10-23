package json

import (
	"bytes"
	"testing"

	. "github.com/warpfork/go-wish"

	"github.com/polydawn/refmt/tok/fixtures"
)

// note: we still put all tests in one func so we control order.
// this will let us someday refactor all `fixtures.SequenceMap` refs to use a
// func which quietly records which sequences have tests aimed at them, and we
// can read that back at out the end of the tests and use the info to
// proactively warn ourselves when we have unreferenced tok fixtures.

func Test(t *testing.T) {
	testBool(t)
	testString(t)
	testMap(t)
	testArray(t)
	testComposite(t)
	testNumber(t)
}

func checkCanonical(t *testing.T, sequence fixtures.Sequence, serial string) {
	t.Run("encode canonical", func(t *testing.T) {
		checkEncoding(t, sequence, serial, nil)
	})
	t.Run("decode canonical", func(t *testing.T) {
		checkDecoding(t, sequence, serial, nil)
	})
}

func checkEncoding(t *testing.T, sequence fixtures.Sequence, expectSerial string, expectErr error) {
	t.Helper()
	outputBuf := &bytes.Buffer{}
	tokenSink := NewEncoder(outputBuf, EncodeOptions{})

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
	Wish(t, outputBuf.String(), ShouldEqual, expectSerial)
}

func checkDecoding(t *testing.T, expectSequence fixtures.Sequence, serial string, expectErr error) {
	// Decoding JSON is *never* going to yield length info on tokens,
	//  so we'll strip that here rather than forcing all our fixtures to say it.
	expectSequence = expectSequence.SansLengthInfo()

	t.Helper()
	inputBuf := bytes.NewBufferString(serial)
	tokenSrc := NewDecoder(inputBuf)

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
