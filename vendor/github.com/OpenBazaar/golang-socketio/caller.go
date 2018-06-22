package gosocketio

import (
	"errors"
	"reflect"
)

type caller struct {
	Func        reflect.Value
	Args        reflect.Type
	ArgsPresent bool
	Out         bool
}

var (
	ErrorCallerNotFunc     = errors.New("f is not function")
	ErrorCallerNot2Args    = errors.New("f should have 1 or 2 args")
	ErrorCallerMaxOneValue = errors.New("f should return not more than one value")
)

/**
Parses function passed by using reflection, and stores its representation
for further call on message or ack
*/
func newCaller(f interface{}) (*caller, error) {
	fVal := reflect.ValueOf(f)
	if fVal.Kind() != reflect.Func {
		return nil, ErrorCallerNotFunc
	}

	fType := fVal.Type()
	if fType.NumOut() > 1 {
		return nil, ErrorCallerMaxOneValue
	}

	curCaller := &caller{
		Func: fVal,
		Out:  fType.NumOut() == 1,
	}
	if fType.NumIn() == 1 {
		curCaller.Args = nil
		curCaller.ArgsPresent = false
	} else if fType.NumIn() == 2 {
		curCaller.Args = fType.In(1)
		curCaller.ArgsPresent = true
	} else {
		return nil, ErrorCallerNot2Args
	}

	return curCaller, nil
}

/**
returns function parameter as it is present in it using reflection
*/
func (c *caller) getArgs() interface{} {
	return reflect.New(c.Args).Interface()
}

/**
calls function with given arguments from its representation using reflection
*/
func (c *caller) callFunc(h *Channel, args interface{}) []reflect.Value {
	//nil is untyped, so use the default empty value of correct type
	if args == nil {
		args = c.getArgs()
	}

	a := []reflect.Value{reflect.ValueOf(h), reflect.ValueOf(args).Elem()}
	if !c.ArgsPresent {
		a = a[0:1]
	}

	return c.Func.Call(a)
}
