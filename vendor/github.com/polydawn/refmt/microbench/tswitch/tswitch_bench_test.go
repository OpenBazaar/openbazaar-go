package bench

import (
	"reflect"
	"testing"
)

// Yielding a value based on a type switch at compile time,
// where all types in the switch are concrete.
//
// Std:  0.58 ns/op
// noGC: 0.63 ns/op
// mem:  0.58 ns/op            0 B/op          0 allocs/op
func Benchmark_SwitchConcrete(b *testing.B) {
	var dump interface{}
	var switcher interface{}
	for i := 0; i < b.N; i++ {
		switch switcher.(type) {
		case string:
			dump = switcher
		case int:
			dump = switcher
		default:
			dump = switcher
		}
	}
	_ = dump
}

// Yielding a value based on a type switch at compile time,
// where some types in the switch are interfaces.
//
// Std:  1.74 ns/op
// noGC: 1.77 ns/op
// mem:  1.75 ns/op            0 B/op          0 allocs/op
func Benchmark_SwitchInterfaces(b *testing.B) {
	var dump interface{}
	//	dump = "asdf"
	var switcher interface{}
	//	switcher = "asdf"
	for i := 0; i < b.N; i++ {
		switch switcher.(type) {
		case string:
			dump = switcher
		case int:
			dump = switcher
		case interface {
			Wow(int)
		}:
			dump = switcher
		default:
			dump = switcher
		}
	}
	_ = dump
}

// Yielding a value based on a lookup into a map keyed by `reflect.Type`.
// The `reflect.Type` for the value-to-switch-on is looked up once
// (it's hard to directly compare this with the compile-time switch behavior,
// which cannot separate the switch and the key derive process).
//
// Std:  16.3 ns/op
// noGC: 16.4 ns/op
// mem:  16.3 ns/op             0 B/op          0 allocs/op
func Benchmark_ReflectTypeMapLookupSplit(b *testing.B) {
	var dump interface{}
	var switcher interface{}
	leMap := map[reflect.Type]interface{}{
		reflect.TypeOf(""):         "",
		reflect.TypeOf(1):          "",
		reflect.TypeOf(struct{}{}): "",
	}
	rt := reflect.TypeOf(switcher)
	for i := 0; i < b.N; i++ {
		dump = leMap[rt]
	}
	_ = dump
}

// Yielding a value based on a lookup into a map keyed by `reflect.Type`.
// The `reflect.TypeOf` for the value-to-switch-on is derived each time.
//
// Std:  17.5 ns/op
// noGC: 17.5 ns/op
// mem:  17.5 ns/op             0 B/op          0 allocs/op
//
// Note: yeah, remarkably, `reflect.TypeOf` does not necessarily imply an alloc.
func Benchmark_ReflectTypeMapLookup(b *testing.B) {
	var dump interface{}
	var switcher interface{}
	leMap := map[reflect.Type]interface{}{
		reflect.TypeOf(""):         "",
		reflect.TypeOf(1):          "",
		reflect.TypeOf(struct{}{}): "",
	}
	for i := 0; i < b.N; i++ {
		dump = leMap[reflect.TypeOf(switcher)]
	}
	_ = dump
}

// Same, but with non-nil value, which yes, is costlier (but still no-alloc).
//
// Std:  26.4 ns/op
// noGC: 26.7 ns/op
// mem:  26.7 ns/op             0 B/op          0 allocs/op
func Benchmark_ReflectTypeMapLookupMessier(b *testing.B) {
	var dump interface{}
	var switcher interface{}
	//switcher = "aasdf" // same
	switcher = struct{ x int }{}
	leMap := map[reflect.Type]interface{}{
		reflect.TypeOf(""):         "",
		reflect.TypeOf(1):          "",
		reflect.TypeOf(struct{}{}): "",
	}
	for i := 0; i < b.N; i++ {
		dump = leMap[reflect.TypeOf(switcher)]
	}
	_ = dump
}
