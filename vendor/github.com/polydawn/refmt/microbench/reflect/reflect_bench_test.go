package bench

import (
	"reflect"
	"testing"
)

// http://www.darkcoding.net/software/go-the-price-of-interface/

// Just getting the reflect.Value of a string.
// 45ns.
// Or, 60ns with GC enabled.  Yes, quite a difference.
func Benchmark_ReflectGetValueOfString(b *testing.B) {
	var slot string
	for i := 0; i < b.N; i++ {
		reflect.ValueOf(slot)
	}
	_ = slot
}

// This is fascinating because it's actually much faster.
// Like 5ns compared to 60 for the direct value.
// GC: no impact.  0 allocs (!)
func Benchmark_ReflectGetValueOfStringRef(b *testing.B) {
	var slot string
	var slotAddr_rv reflect.Value
	for i := 0; i < b.N; i++ {
		slotAddr_rv = reflect.ValueOf(&slot)
	}
	_ = slot
	_ = slotAddr_rv
}

// Getting a reflect.Value of a string's address, then `Elem()`'ing back to the string type.
// This is the hop you need to do to have an *addressable* value that you can set.
// Like 8ns compared to the 9ns (appears `Elem()` adds ~3.5ns).
// GC: no impact.  0 allocs (!)
func Benchmark_ReflectGetValueOfStringRefElem(b *testing.B) {
	var slot string
	var slot_rav reflect.Value
	for i := 0; i < b.N; i++ {
		_ = reflect.ValueOf(&slot).Elem()
	}
	_ = slot
	_ = slot_rav
}

// About 24ns, by the same rulers as the others.
// GC: no impact.  0 allocs (!)
func Benchmark_ReflectSetValue(b *testing.B) {
	var slot string
	slot_rav := reflect.ValueOf(&slot).Elem()
	var val = "x"
	val_rv := reflect.ValueOf(val)
	for i := 0; i < b.N; i++ {
		slot_rav.Set(val_rv)
	}
	_ = slot
	_ = slot_rav
}

// Just setting something through an address, full types.
// Very fast (obviously): less than a single nano.
// GC: no impact.  0 allocs (!)
func Benchmark_DirectSetValue(b *testing.B) {
	var slot string
	slotAddr_v := &slot
	var val = "x"
	for i := 0; i < b.N; i++ {
		*slotAddr_v = val
	}
	_ = slot
}

// Fit the address of a primitive into an `interface{}`, then type-switch
// it back to a primitive so we can directly set it.
// Still very fast: 2ns.
// GC: no impact.  0 allocs (!)
// Context: looking 4/5x faster than the equivalent ops with reflect
// (but that's maybe less of a margin than I might've expected).
func Benchmark_DirectInterfacedSetValue(b *testing.B) {
	var slot string
	var slotAddr_v interface{} = &slot
	var val = "x"
	for i := 0; i < b.N; i++ {
		switch v2 := slotAddr_v.(type) {
		case *interface{}:
			// sigh
		case *string:
			*v2 = val
		}
	}
	_ = slot
}

// Use a func to get that `interface{}` that's a pointer.
// This is emulating the fastest path we can do with an atlas with user-written functions.
// Still fast: 3ns.
// GC: no impact.  0 allocs (!)
func Benchmark_FuncedDirectInterfacedSetValue(b *testing.B) {
	var slot string
	addrFunc := func() interface{} { return &slot }
	var val = "x"
	var slotAddr_v interface{}
	for i := 0; i < b.N; i++ {
		slotAddr_v = addrFunc()
		switch v2 := slotAddr_v.(type) {
		case *interface{}:
			// sigh
		case *string:
			*v2 = val
		}
	}
	_ = slot
}
