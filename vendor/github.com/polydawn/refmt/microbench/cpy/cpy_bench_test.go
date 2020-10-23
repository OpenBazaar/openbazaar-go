/*
	Guides the design of `tok.Token`:
	test whether it's important to yield pointers to the values of interest,
	versus simply putting the values in an `interface{}` slot.

	(It is.)

	Though syntactically irritating to use of pointers to primitives for all tokens,
	this bench demonstrates that doing so avoids a source of allocs,
	and thus has significant performance implications.
*/
package bench

import (
	"testing"
)

// Std:  Benchmark_CopyByBoxedValue-8         30000000                43.4 ns/op
// noGC: Benchmark_CopyByBoxedValue-8         30000000                34.0 ns/op
// mem:  Benchmark_CopyByBoxedValue-8         30000000                44.4 ns/op             8 B/op          1 allocs/op
func Benchmark_CopyByBoxedValue(b *testing.B) {
	type Alias interface{}
	var slot Alias
	type StructA struct {
		field int
	}
	type StructB struct {
		field int
	}
	valA := StructA{4}
	valB := StructB{}

	for i := 0; i < b.N; i++ {
		slot = valA.field
		valB.field = slot.(int)
	}
	if valB.field != 4 {
		b.Error("final value of valB wrong")
	}
}

// Std:  Benchmark_CopyByBoxedRef-8           2000000000               0.59 ns/op
// noGC: Benchmark_CopyByBoxedRef-8           2000000000               0.59 ns/op
// mem:  Benchmark_CopyByBoxedRef-8           2000000000               0.59 ns/op            0 B/op          0 allocs/op
func Benchmark_CopyByBoxedRef(b *testing.B) {
	type Alias interface{}
	var slot Alias
	type StructA struct {
		field int
	}
	type StructB struct {
		field int
	}
	valA := StructA{4}
	valB := StructB{}

	for i := 0; i < b.N; i++ {
		slot = &(valA.field)
		valB.field = *(slot.(*int))
	}
	if valB.field != 4 {
		b.Error("final value of valB wrong")
	}
}

// Sanity check: strings are not noticably different:
//
//	Benchmark_CopyByBoxedValue-8                 30000000                45.5 ns/op
//	Benchmark_CopyByBoxedRef-8                   2000000000               0.59 ns/op
//	Benchmark_CopyByBoxedValue_String-8          20000000                72.3 ns/op
//	Benchmark_CopyByBoxedRef_String-8            2000000000               0.60 ns/op
//
// BUT, mind what you're observing here: this is essentially the cost of boxing
// a value into an `interface{}` handle: substantial CPU and also a malloc.
// See https://www.darkcoding.net/software/go-the-price-of-interface/ for discussion.
// The copy-by-value test *without* `interface{}` boxing demonstrates this:
//
//	Benchmark_CopyByRef_String-8                 2000000000               0.61 ns/op
//
// Interestingly, whether or not there's a cast doesn't seem to produce any
// difference in performance between the direct use of string and the boxed ref.
//
func Benchmark_CopyByBoxedValue_String(b *testing.B) {
	type Alias interface{}
	var slot Alias
	type StructA struct {
		field string
	}
	type StructB struct {
		field string
	}
	valA := StructA{"alksjdlkjweoihgowihehgioijerg"}
	valB := StructB{}

	for i := 0; i < b.N; i++ {
		slot = valA.field
		valB.field = slot.(string)
	}
	if valB.field != valA.field {
		b.Error("final value of valB wrong")
	}
}

func Benchmark_CopyByValue_String(b *testing.B) {
	var slot string
	type StructA struct {
		field string
	}
	type StructB struct {
		field string
	}
	valA := StructA{"alksjdlkjweoihgowihehgioijerg"}
	valB := StructB{}

	for i := 0; i < b.N; i++ {
		slot = valA.field
		valB.field = slot
	}
	if valB.field != valA.field {
		b.Error("final value of valB wrong")
	}
}

func Benchmark_CopyByBoxedRef_String(b *testing.B) {
	type Alias interface{}
	var slot Alias
	type StructA struct {
		field string
	}
	type StructB struct {
		field string
	}
	valA := StructA{"alksjdlkjweoihgowihehgioijerg"}
	valB := StructB{}

	for i := 0; i < b.N; i++ {
		slot = &(valA.field)
		valB.field = *(slot.(*string))
	}
	if valB.field != valA.field {
		b.Error("final value of valB wrong")
	}
}

// One more quick check -- let's be sure that it's not some bizzare hyperoptimization
// that only works on pointers when they're the same: let's flip two different types
// of pointers through the interface slot:
//
//	Benchmark_CopyByBoxedValue_Mixed-8           20000000               117 ns/op              24 B/op          2 allocs/op
//	Benchmark_CopyByBoxedRef_Mixed-8             2000000000               0.90 ns/op            0 B/op          0 allocs/op
//
// Nope.  Same.  Alloc size and count matches straight addition for copy-by-boxed-value,
// and remains zero zero and fast for copy-by-ref.
func Benchmark_CopyByBoxedValue_Mixed(b *testing.B) {
	type Alias interface{}
	var slot Alias
	type StructA struct {
		field string
		other int
	}
	type StructB struct {
		field string
		other int
	}
	valA := StructA{"alksjdlkjweoihgowihehgioijerg", 4}
	valB := StructB{}

	for i := 0; i < b.N; i++ {
		slot = valA.field
		valB.field = slot.(string)
		slot = valA.other
		valB.other = slot.(int)
	}
	if valB.field != valA.field {
		b.Error("final value of valB str wrong")
	}
	if valB.other != valA.other {
		b.Error("final value of valB int wrong")
	}
}

func Benchmark_CopyByBoxedRef_Mixed(b *testing.B) {
	type Alias interface{}
	var slot Alias
	type StructA struct {
		field string
		other int
	}
	type StructB struct {
		field string
		other int
	}
	valA := StructA{"alksjdlkjweoihgowihehgioijerg", 4}
	valB := StructB{}

	for i := 0; i < b.N; i++ {
		slot = &(valA.field)
		valB.field = *(slot.(*string))
		slot = &(valA.other)
		valB.other = *(slot.(*int))
	}
	if valB.field != valA.field {
		b.Error("final value of valB str wrong")
	}
	if valB.other != valA.other {
		b.Error("final value of valB int wrong")
	}
}
