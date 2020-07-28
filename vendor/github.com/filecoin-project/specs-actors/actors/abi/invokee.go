package abi

type Invokee interface {
	Exports() []interface{}
}
