package slicegrowth

import (
	"reflect"
	"testing"
)

// okay so this append method *is* amortizing sanely.
// but we still have two allocs per set here, why?
func BenchmarkReflectAppend_Naive(b *testing.B) {
	x := []int{}
	rv := reflect.ValueOf(&x).Elem()
	rt_val := reflect.TypeOf(0)

	for i := 0; i < b.N; i++ {
		rv.Set(reflect.Append(rv, reflect.Zero(rt_val)))
	}
	b.Logf("len, cap = %d, %d", len(x), cap(x))
}

// yep that was one of 'em.  reflect.Zero causes malloc woo.
func BenchmarkReflectAppend_ConstantZero(b *testing.B) {
	x := []int{}
	rv := reflect.ValueOf(&x).Elem()
	rt_val := reflect.TypeOf(0)
	rv_valZero := reflect.Zero(rt_val)

	for i := 0; i < b.N; i++ {
		rv.Set(reflect.Append(rv, rv_valZero))
	}
	b.Logf("len, cap = %d, %d", len(x), cap(x))
}

// faster.  but still averaging an alloc per append, how and why.
func BenchmarkReflectAppend_ConstZeroAndFinalSet(b *testing.B) {
	x := []int{}
	rv := reflect.ValueOf(&x).Elem()
	rt_val := reflect.TypeOf(0)
	rv_valZero := reflect.Zero(rt_val)

	rv_moving := rv
	for i := 0; i < b.N; i++ {
		rv_moving = reflect.Append(rv_moving, rv_valZero)
	}
	rv.Set(rv_moving)
	b.Logf("x  len, cap = %d, %d", len(x), cap(x))
}

// this did still shave off another perceptible 4ns.  but not the rogue alloc.
func BenchmarkReflectAppend_ConstZeroAndFinalSetAndNoVararg(b *testing.B) {
	x := []int{}
	rv := reflect.ValueOf(&x).Elem()
	rt_val := reflect.TypeOf(0)
	rv_valZero := reflect.Zero(rt_val)

	rv_moving := rv
	for i := 0; i < b.N; i++ {
		rv_moving = appendOne(rv_moving, rv_valZero)
	}
	rv.Set(rv_moving)
	b.Logf("x  len, cap = %d, %d", len(x), cap(x))
}

func appendOne(s reflect.Value, v reflect.Value) reflect.Value {
	s, i0, _ := grow(s, 1)
	s.Index(i0).Set(v)
	return s
}

func grow(s reflect.Value, extra int) (reflect.Value, int, int) {
	i0 := s.Len()
	i1 := i0 + extra
	if i1 < i0 {
		panic("reflect.Append: slice overflow")
	}
	m := s.Cap()
	if i1 <= m {
		return s.Slice(0, i1), i0, i1
	}
	if m == 0 {
		m = extra
	} else {
		for m < i1 {
			if i0 < 1024 {
				m += m
			} else {
				m += m / 4
			}
		}
	}
	t := reflect.MakeSlice(s.Type(), i1, m)
	reflect.Copy(t, s)
	return t, i0, i1
}

// this did still shave off another perceptible 4ns.  but not the rogue alloc.
func BenchmarkReflectAppend_wats(b *testing.B) {
	x := []int{}
	rv := reflect.ValueOf(&x).Elem()
	rt_val := reflect.TypeOf(0)
	rv_valZero := reflect.Zero(rt_val)

	rv_moving := rv
	for i := 0; i < b.N; i++ {
		rv_moving, _, _ = grow(rv_moving, 1)
		rv_moving.Index(i).Set(rv_valZero)
	}
	rv.Set(rv_moving)
	b.Logf("x  len, cap = %d, %d", len(x), cap(x))
}
