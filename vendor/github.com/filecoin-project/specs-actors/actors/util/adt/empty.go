package adt

import (
	"fmt"
	"io"

	runtime "github.com/filecoin-project/specs-actors/actors/runtime"
)

// The empty value represents absence of a value. It is used for parameter and return types for actor methods
// that don't take/return any data. This saves a byte in serialization of messages and receipts: the serialized
// form is an empty byte slice, rather than a byte slice containing a single byte CBOR encoding of nil/empty/etc.
//
// The only expected use of this is as the type of a nil reference. Don't instantiate this type.
//
// This is primarily necessary due to Go's lack of a void type and our interface-based serialization scheme.
type EmptyValue struct{}

// A typed nil pointer to EmptyValue.
var Empty *EmptyValue = nil

var _ runtime.CBORMarshaler = (*EmptyValue)(nil)
var _ runtime.CBORUnmarshaler = (*EmptyValue)(nil)

func (v *EmptyValue) MarshalCBOR(_ io.Writer) error {
	// An attempt to serialize a non-nil value indicates a caller mis-using this type.
	if v != nil {
		return fmt.Errorf("cannot marshal empty value, try nil instead")
	}
	// Allow nil to write zero bytes as a convenience so callers don't need to nil-check all values before
	// attempting serialization.
	return nil
}

func (v *EmptyValue) UnmarshalCBOR(_ io.Reader) error {
	// Read zero bytes.
	return nil
}
