package obj

import (
	"reflect"
	"testing"
)

type T1 struct{}

type T2 struct {
	array []byte
}
type T3 struct {
	array []*T3
}

func TestIsEmptyValue(t *testing.T) {
	isEmpty := func(v interface{}) {
		if !isEmptyValue(reflect.ValueOf(v)) {
			t.Fatalf("expected value of type %T to be empty", v)
		}
	}
	isEmpty(T1{})
	isEmpty(T2{})
	isEmpty(T3{})
}
