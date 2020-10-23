package tokconv

import "testing"

func BenchmarkFatUnionSingleAssignment(b *testing.B) {
	var slot FatToken
	for i := 0; i < b.N; i++ {
		// Common subexpression elimination is going to turn this into trash.
		slot.Type = TMapOpen
		slot.Length = 1
		slot.Type = TMapClose
		slot.Type = TArrOpen
		slot.Length = 1
		slot.Type = TArrClose
		slot.Type = TNull
		slot.Type = TString
		slot.Str = "foo"
		slot.Type = TBytes
		slot.Bytes = []byte{'f', 'o', 'o'}
		slot.Type = TBool
		slot.Bool = true
		slot.Type = TInt
		slot.Int = 99
		slot.Type = TUint
		slot.Uint = 99
		slot.Type = TFloat64
		slot.Float64 = 9.9
	}
}

func BenchmarkIffyUnionSingleAssignment(b *testing.B) {
	var slot IffyToken
	for i := 0; i < b.N; i++ {
		slot = TIMapOpen{1}
		slot = TIMapClose{}
		slot = TIArrOpen{1}
		slot = TIArrClose{}
		slot = TINull{}
		slot = TIString{"foo"}
		slot = TIBytes{[]byte{'f', 'o', 'o'}}
		slot = TIBool{true}
		slot = TIInt{99}
		slot = TIUint{99}
		slot = TIFloat64{9.9}
	}
	_ = slot
}

func BenchmarkIffyUnionSingleRefAssignment(b *testing.B) {
	var uff IffyToken
	var slot = &uff
	for i := 0; i < b.N; i++ {
		*slot = TIMapOpen{1}
		*slot = TIMapClose{}
		*slot = TIArrOpen{1}
		*slot = TIArrClose{}
		*slot = TINull{}
		*slot = TIString{"foo"}
		*slot = TIBytes{[]byte{'f', 'o', 'o'}}
		*slot = TIBool{true}
		*slot = TIInt{99}
		*slot = TIUint{99}
		*slot = TIFloat64{9.9}
	}
	_ = slot
}

// You'll notice this is an awful lot like single assignment without the ptrs.
// That's because it **is**: storing the structures in an iface forces ptrism.
// And n.b. you can't avoid this: since interfaces aren't possible to "close"
// in golang (well, in a way that the compiler will Take Note of), even if
// all possible inhabitants of an interface have fixed size, it'll still do
// this forced conversion to ptrism.
func BenchmarkIffyUnionSingleAssignmentRef(b *testing.B) {
	var slot IffyToken
	for i := 0; i < b.N; i++ {
		slot = &TIMapOpen{1}
		slot = &TIMapClose{}
		slot = &TIArrOpen{1}
		slot = &TIArrClose{}
		slot = &TINull{}
		slot = &TIString{"foo"}
		slot = &TIBytes{[]byte{'f', 'o', 'o'}}
		slot = &TIBool{true}
		slot = &TIInt{99}
		slot = &TIUint{99}
		slot = &TIFloat64{9.9}
	}
	_ = slot
}

// Okay, but... it forces ptr creation *to point to structs with a size*.
// Did you notice how the things above all have *9* allocs per op?
// Count 'em.  That's every *struct **with members*** and one for the bytes.
// You see the key bit?  Structs with no members didn't count.
//
// So what about primitives that also fit in a word already?
func BenchmarkBoxingPrimitives(b *testing.B) {
	var slot interface{}
	for i := 0; i < b.N; i++ {
		slot = 1
		slot = 2
		slot = 3
		slot = "four" // already a "pointer", and without alloc
	}
	_ = slot
}

func BenchmarkBoxingPrimitives2(b *testing.B) {
	var slot interface{}
	for i := 0; i < b.N; i++ {
		slot = 1
		slot = 2
		slot = 3
		slot = []byte{'f', 'o', 'o'} // should make this 1alloc, and thus comparible to BenchmarkFatUnionSingleAssignment
		// two, somehow.  didn't expect that.
		// i guess the handle of a byte slice is more than one word of memory, so, yeah, okay.
	}
	_ = slot
}

func BenchmarkBoxingPrimitives3(b *testing.B) {
	var slot interface{}
	for i := 0; i < b.N; i++ {
		slot = 1
		slot = "four"
		slot = 5.5
		slot = float64(9.9)
		slot = true
	}
	_ = slot
}

// This is cool.
// As long as your domain of values can entirely and unambiguously be crammed into go's existing primitive types...
// or that domain plus some compile-time enumerable set of additional states (because you can wedge those in via the typeinfo of zero-field structs)...
// then you can actually use interfaces without hitting the runtime.convt2e speedbump.
// Wow.  TIL.
//
// (So, that comment on BenchmarkIffyUnionSingleAssignmentRef about interfaces
// being impossible to 'close' in a way the compiler will Take Note of turns out
// to have been irrelevant.  This is definitely a "happy to be wrong" incident.)
//
// Of course, this doesn't actually help for refmt tokens.
// - MapOpen is both a state flag and as the length parameter.
// - ArrOpen is the same.
// - and cbor Tags of course make huge mess.
// - also, bytes.
//
// ... Could we pull a huge barrel roll and force all ints into the 64 bit spaces,
// and then declare that our map and array support is limited to 2 billion elements
// (and they already are; OOM is real), and then abuse the 32bit int and uint as
// the "open" token types?  Oh boy.  I dunno.  I bet we could.
// Not gonna touch 'should'.
// Would also still leave tags and bytes unmagick'd.
// Interesting thought, though.

// But wait.  There's more!
func BenchmarkBoxingPrimitives4(b *testing.B) {
	var slot interface{}
	for i := 0; i < b.N; i++ {
		slot = 1
		slot = complex(float64(2), 3) // complex128 // ... I am actually pretty shocked this fits
		// switch slot.(type) {
		// case complex128:
		// 	// fine
		// default:
		// 	panic("?")
		// }
		slot = complex(float32(4), 5) // complex64
		// switch slot.(type) {
		// case complex64:
		// 	// fine
		// default:
		// 	panic("?")
		// }
	}
	_ = slot
}

// One more interesting note.
//
// re: "structs with a size" earlier... that's... not actually the check.
// And this is super unfortunate, because this would almost catapult us
// straight out of the weeds for most of the usecases I have around.
//
// Notice how `struct{string}` and `struct{int}` types are still incurring the
// wrath of convt2e... even though `string` and `int` fit in the second
// word of the interface, and thus *don't*.
//
// IIUC, those structs should be the same size as their single member (and
// their type info isn't adding any size; it's the first word of the interface),
// and so I'd think they should *also* fit without incurrent convt2e.  But no?
//
// This seems like a potentially improvable thing in the runtime.

// Also note, these convt2e elisions seem to have happened somewhere between
// 1.7 and 1.8 as far as I can tell.  E.g. sometime ~early 2017.
