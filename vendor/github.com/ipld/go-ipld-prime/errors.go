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

// ErrInvalidKey indicates a key is invalid for some reason.
//
// This is only possible for typed nodes; specifically, it may show up when
// handling struct types, or maps with interesting key types.
// (Other kinds of key invalidity that happen for untyped maps
// fall under ErrRepeatedMapKey or ErrWrongKind.)
// (Union types use ErrInvalidUnionDiscriminant instead of ErrInvalidKey,
// even when their representation strategy is maplike.)
type ErrInvalidKey struct {
	// TypeName will indicate the named type of a node the function was called on.
	TypeName string

	// Key is the key that was rejected.
	Key Node

	// Reason, if set, may provide details (for example, the reason a key couldn't be converted to a type).
	// If absent, it'll be presumed "no such field".
	// ErrUnmatchable may show up as a reason for typed maps with complex keys.
	Reason error
}

func (e ErrInvalidKey) Error() string {
	if e.Reason == nil {
		return fmt.Sprintf("invalid key for map %s: \"%s\": no such field", e.TypeName, e.Key)
	} else {
		return fmt.Sprintf("invalid key for map %s: \"%s\": %s", e.TypeName, e.Key, e.Reason)
	}
}

// ErrInvalidSegmentForList is returned when using Node.LookupBySegment and the
// given PathSegment can't be applied to a list because it's unparsable as a number.
type ErrInvalidSegmentForList struct {
	// TypeName may indicate the named type of a node the function was called on,
	// or be empty string if working on untyped data.
	TypeName string

	// TroubleSegment is the segment we couldn't use.
	TroubleSegment PathSegment

	// Reason may explain more about why the PathSegment couldn't be used;
	// in practice, it's probably a 'strconv.NumError'.
	Reason error
}

func (e ErrInvalidSegmentForList) Error() string {
	v := "invalid segment for lookup on a list"
	if e.TypeName != "" {
		v += " of type " + e.TypeName
	}
	return v + fmt.Sprintf(": %q: %s", e.TroubleSegment.s, e.Reason)
}

// ErrUnmatchable is the catch-all type for parse errors in schema representation work.
//
// REVIEW: are builders at type level ever going to return this?  i don't think so.
// REVIEW: can this ever be triggered during the marshalling direction?  perhaps not.
// REVIEW: do things like ErrWrongKind end up being wrapped by this?  that doesn't seem pretty.
// REVIEW: do natural representations ever trigger this?  i don't think so.  maybe that's a hint towards a better name.
// REVIEW: are user validation functions encouraged to return this?  or something else?
//
type ErrUnmatchable struct {
	// TypeName will indicate the named type of a node the function was called on.
	TypeName string

	// Reason must always be present.  ErrUnmatchable doesn't say much otherwise.
	Reason error
}

func (e ErrUnmatchable) Error() string {
	return fmt.Sprintf("parsing of %s rejected: %s", e.TypeName, e.Reason)
}

// ErrIteratorOverread is returned when calling 'Next' on a MapIterator or
// ListIterator when it is already done.
type ErrIteratorOverread struct{}

func (e ErrIteratorOverread) Error() string {
	return "iterator overread"
}

type ErrCannotBeNull struct{} // Review: arguably either ErrInvalidKindForNodePrototype.

type ErrMissingRequiredField struct{}     // only possible for typed nodes -- specifically, struct types.
type ErrListOverrun struct{}              // only possible for typed nodes -- specifically, struct types with list (aka tuple) representations.
type ErrInvalidUnionDiscriminant struct{} // only possible for typed nodes -- specifically, union types.
