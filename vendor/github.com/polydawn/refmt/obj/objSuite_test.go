package obj

import (
	"testing"

	. "github.com/warpfork/go-wish"

	"github.com/polydawn/refmt/obj/atlas"
	"github.com/polydawn/refmt/tok"
)

func checkUnmarshalling(t *testing.T, atl atlas.Atlas, slot interface{}, sequence []tok.Token, expect interface{}, expectErr error) {
	t.Helper()
	unmarshaller := NewUnmarshaller(atl)
	err := unmarshaller.Bind(slot)
	Wish(t, err, ShouldEqual, nil)

	// Run steps, advancing through the token sequence.
	//  If it stops early, just report how many steps in; we Wish on that value.
	//  If it doesn't stop in time, just report that bool; we Wish on that value.
	var nStep int
	var done bool
	for _, tok := range sequence {
		nStep++
		done, err = unmarshaller.Step(&tok)
		if done || err != nil {
			break
		}
	}

	// Assert final result.
	Wish(t, done, ShouldEqual, true)
	Wish(t, nStep, ShouldEqual, len(sequence))
	Wish(t, err, ShouldEqual, expectErr)
	Wish(t, slot, ShouldEqual, expect)
}

func checkMarshalling(t *testing.T, atl atlas.Atlas, value interface{}, expectSequence []tok.Token, expectErr error) {
	t.Helper()
	marshaller := NewMarshaller(atl)
	err := marshaller.Bind(value)
	Wish(t, err, ShouldEqual, nil)

	// Run steps, advancing until the marshaller reports it's done.
	//  If the marshaller keeps yielding more tokens than we expect, that's fine...
	//  we just keep recording them, and we'll diff later.
	//  There's a cutoff when it overshoots by 10 tokens because generally
	//  that indicates we've found some sort of loop bug and 10 extra token
	//  yields is typically enough info to diagnose with.
	var nStep int
	var done bool
	var yield = make([]tok.Token, len(expectSequence)+10)
	for ; nStep <= len(expectSequence)+10; nStep++ {
		done, err = marshaller.Step(&yield[nStep])
		if done || err != nil {
			break
		}
	}
	nStep++
	yield = yield[:nStep]

	// Assert final result.
	Wish(t, done, ShouldEqual, true)
	Wish(t, nStep, ShouldEqual, len(expectSequence))
	Wish(t, yield, ShouldEqual, expectSequence)
	Wish(t, err, ShouldEqual, expectErr)
}
