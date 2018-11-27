package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestHandleReturnValue(t *testing.T) {
	// one value
	v, err := handleReturnValue([]reflect.Value{reflect.ValueOf(1)})
	if v.(int) != 1 {
		t.Fatal("expected value")
	}
	if err != nil {
		t.Fatal(err)
	}

	// Nil value
	v, err = handleReturnValue([]reflect.Value{reflect.ValueOf(nil)})
	if v != nil {
		t.Fatal("expected no value")
	}
	if err == nil {
		t.Fatal("expected an error")
	}

	// Nil value, nil err
	v, err = handleReturnValue([]reflect.Value{reflect.ValueOf(nil), reflect.ValueOf(nil)})
	if v != nil {
		t.Fatal("expected no value")
	}
	if err == nil {
		t.Fatal("expected an error")
	}

	// two values
	v, err = handleReturnValue([]reflect.Value{reflect.ValueOf(1), reflect.ValueOf(nil)})
	if v, ok := v.(int); !ok || v != 1 {
		t.Fatalf("expected value of 1, got %v", v)
	}
	if err != nil {
		t.Fatal("expected no error")
	}

	// an error
	myError := errors.New("my error")
	_, err = handleReturnValue([]reflect.Value{reflect.ValueOf(1), reflect.ValueOf(myError)})
	if err != myError {
		t.Fatal(err)
	}

	for _, vals := range [][]reflect.Value{
		{reflect.ValueOf(1), reflect.ValueOf("not an error")},
		{},
		{reflect.ValueOf(1), reflect.ValueOf(myError), reflect.ValueOf(myError)},
	} {
		func() {
			defer func() { recover() }()
			handleReturnValue(vals)
			t.Fatal("expected a panic")
		}()
	}
}

type foo interface {
	foo() foo
}

var fooType = reflect.TypeOf((*foo)(nil)).Elem()

func TestCheckReturnType(t *testing.T) {
	for i, fn := range []interface{}{
		func() { panic("") },
		func() error { panic("") },
		func() (error, error) { panic("") },
		func() (foo, error, error) { panic("") },
		func() (foo, foo) { panic("") },
	} {
		if checkReturnType(reflect.TypeOf(fn), fooType) == nil {
			t.Errorf("expected falure for case %d (type %T)", i, fn)
		}
	}

	for i, fn := range []interface{}{
		func() foo { panic("") },
		func() (foo, error) { panic("") },
	} {
		if err := checkReturnType(reflect.TypeOf(fn), fooType); err != nil {
			t.Errorf("expected success for case %d (type %T), got: %s", i, fn, err)
		}
	}
}

func constructFoo() foo {
	return nil
}

type fooImpl struct{}

func (f *fooImpl) foo() foo { return nil }

func TestCallConstructor(t *testing.T) {
	_, err := callConstructor(reflect.ValueOf(constructFoo), nil)
	if err == nil {
		t.Fatal("expected constructor to fail")
	}

	if !strings.Contains(err.Error(), "constructFoo") {
		t.Errorf("expected error to contain the constructor name: %s", err)
	}

	v, err := callConstructor(reflect.ValueOf(func() foo { return &fooImpl{} }), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(*fooImpl); !ok {
		t.Fatal("expected a fooImpl")
	}

	v, err = callConstructor(reflect.ValueOf(func() *fooImpl { return new(fooImpl) }), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(*fooImpl); !ok {
		t.Fatal("expected a fooImpl")
	}

	_, err = callConstructor(reflect.ValueOf(func() (*fooImpl, error) { return nil, nil }), nil)
	if err == nil {
		t.Fatal("expected error")
	}

	v, err = callConstructor(reflect.ValueOf(func() (*fooImpl, error) { return new(fooImpl), nil }), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(*fooImpl); !ok {
		t.Fatal("expected a fooImpl")
	}
}
