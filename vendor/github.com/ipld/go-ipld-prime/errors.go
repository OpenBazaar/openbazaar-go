package ipld

import (
	"fmt"
)

// ErrWrongKind may be returned from functions on the Node interface when
// a method is invoked which doesn't make sense for the Kind and/or ReprKind
// that node concretely contains.
//
// For example, calling AsString on a map will return ErrWrongKind.
// Calling Lookup on an int will similarly return ErrWrongKind.
type ErrWrongKind struct {
	// TypeName may optionally indicate the named type of a node the function
	// was called on (if the node was typed!), or, may be the empty string.
	TypeName string

	// MethodName is literally the string for the operation attempted, e.g.
	// "AsString".
	//
	// For methods on nodebuilders, we say e.g. "NodeBuilder.CreateMap".
	MethodName string

	// ApprorpriateKind describes which ReprKinds the erroring method would
	// make sense for.
	AppropriateKind ReprKindSet

	// ActualKind describes the ReprKind of the node the method was called on.
	//
	// In the case of typed nodes, this will typically refer to the 'natural'
	// data-model kind for such a type (e.g., structs will say 'map' here).
	ActualKind ReprKind
}

func (e ErrWrongKind) Error() string {
	if e.TypeName == "" {
		return fmt.Sprintf("func called on wrong kind: %s called on a %s node, but only makes sense on %s", e.MethodName, e.ActualKind, e.AppropriateKind)
	} else {
		return fmt.Sprintf("func called on wrong kind: %s called on a %s node (kind: %s), but only makes sense on %s", e.MethodName, e.TypeName, e.ActualKind, e.AppropriateKind)
	}
}

// ErrNotExists may be returned from the lookup functions of the Node interface
// to indicate a missing value.
//
// Note that schema.ErrNoSuchField is another type of error which sometimes
// occurs in similar places as ErrNotExists.  ErrNoSuchField is preferred
// when handling data with constraints provided by a schema that mean that
// a field can *never* exist (as differentiated from a map key which is
// simply absent in some data).
type ErrNotExists struct {
	Segment PathSegment
}

func (e ErrNotExists) Error() string {
	return fmt.Sprintf("key not found: %q", e.Segment)
}

// ErrRepeatedMapKey is an error indicating that a key was inserted
// into a map that already contains that key.
//
// This error may be returned by any methods that add data to a map --
// any of the methods on a NodeAssembler that was yielded by MapAssembler.AssignKey(),
// or from the MapAssembler.AssignDirectly() method.
type ErrRepeatedMapKey struct {
	Key Node
}

func (e ErrRepeatedMapKey) Error() string {
	return fmt.Sprintf("cannot repeat map key (\"%s\")", e.Key)
}

// ErrIteratorOverread is returned when calling 'Next' on a MapIterator or
// ListIterator when it is already done.
type ErrIteratorOverread struct{}

func (e ErrIteratorOverread) Error() string {
	return "iterator overread"
}

type ErrCannotBeNull struct{} // Review: arguably either ErrInvalidKindForNodeStyle.

type ErrInvalidStructKey struct{}         // only possible for typed nodes -- specifically, struct types.
type ErrMissingRequiredField struct{}     // only possible for typed nodes -- specifically, struct types.
type ErrListOverrun struct{}              // only possible for typed nodes -- specifically, struct types with list (aka tuple) representations.
type ErrInvalidUnionDiscriminant struct{} // only possible for typed nodes -- specifically, union types.
