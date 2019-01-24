package cbor

import (
	"fmt"
	"io"

	"gx/ipfs/QmPAdjGx1huCjnrR26qy9QUUNSqA6EStyZ68RrwbtCTDML/refmt/shared"
	. "gx/ipfs/QmPAdjGx1huCjnrR26qy9QUUNSqA6EStyZ68RrwbtCTDML/refmt/tok"
)

type Decoder struct {
	r shared.SlickReader

	stack []decoderStep // When empty, and step returns done, all done.
	step  decoderStep   // Shortcut to end of stack.
	left  []int         // Statekeeping space for definite-len map and array.
}

func NewDecoder(r io.Reader) (d *Decoder) {
	d = &Decoder{
		r:     shared.NewReader(r),
		stack: make([]decoderStep, 0, 10),
		left:  make([]int, 0, 10),
	}
	d.step = d.step_acceptValue
	return
}

func (d *Decoder) Reset() {
	d.stack = d.stack[0:0]
	d.step = d.step_acceptValue
	d.left = d.left[0:0]
}

type decoderStep func(tokenSlot *Token) (done bool, err error)

func (d *Decoder) Step(tokenSlot *Token) (done bool, err error) {
	done, err = d.step(tokenSlot)
	// If the step errored: out, entirely.
	if err != nil {
		return true, err
	}
	// If the step wasn't done, return same status.
	if !done {
		return false, nil
	}
	// If it WAS done, pop next, or if stack empty, we're entirely done.
	nSteps := len(d.stack) - 1
	if nSteps <= 0 {
		return true, nil // that's all folks
	}
	d.step = d.stack[nSteps]
	d.stack = d.stack[0:nSteps]
	return false, nil
}

func (d *Decoder) pushPhase(newPhase decoderStep) {
	d.stack = append(d.stack, d.step)
	d.step = newPhase
}

// The original step, where any value is accepted, and no terminators for composites are valid.
// ONLY used in the original step; all other steps handle leaf nodes internally.
func (d *Decoder) step_acceptValue(tokenSlot *Token) (done bool, err error) {
	majorByte, err := d.r.Readn1()
	if err != nil {
		return true, err
	}
	tokenSlot.Tagged = false
	return d.stepHelper_acceptValue(majorByte, tokenSlot)
}

// Step in midst of decoding an indefinite-length array.
func (d *Decoder) step_acceptArrValueOrBreak(tokenSlot *Token) (done bool, err error) {
	majorByte, err := d.r.Readn1()
	if err != nil {
		return true, err
	}
	tokenSlot.Tagged = false
	switch majorByte {
	case cborSigilBreak:
		tokenSlot.Type = TArrClose
		return true, nil
	default:
		_, err := d.stepHelper_acceptValue(majorByte, tokenSlot)
		return false, err
	}
}

// Step in midst of decoding an indefinite-length map, key expected up next, or end.
func (d *Decoder) step_acceptMapIndefKey(tokenSlot *Token) (done bool, err error) {
	majorByte, err := d.r.Readn1()
	if err != nil {
		return true, err
	}
	tokenSlot.Tagged = false
	switch majorByte {
	case cborSigilBreak:
		tokenSlot.Type = TMapClose
		return true, nil
	default:
		d.step = d.step_acceptMapIndefValueOrBreak
		_, err := d.stepHelper_acceptValue(majorByte, tokenSlot) // FIXME surely not *any* value?  not composites, at least?
		return false, err
	}
}

// Step in midst of decoding an indefinite-length map, value expected up next.
func (d *Decoder) step_acceptMapIndefValueOrBreak(tokenSlot *Token) (done bool, err error) {
	majorByte, err := d.r.Readn1()
	if err != nil {
		return true, err
	}
	tokenSlot.Tagged = false
	switch majorByte {
	case cborSigilBreak:
		return true, fmt.Errorf("unexpected break; expected value in indefinite-length map")
	default:
		d.step = d.step_acceptMapIndefKey
		_, err = d.stepHelper_acceptValue(majorByte, tokenSlot)
		return false, err
	}
}

// Step in midst of decoding a definite-length array.
func (d *Decoder) step_acceptArrValue(tokenSlot *Token) (done bool, err error) {
	// Yield close token, pop state, and return done flag if expecting no more entries.
	ll := len(d.left) - 1
	if d.left[ll] == 0 {
		d.left = d.left[0:ll]
		tokenSlot.Type = TArrClose
		return true, nil
	}
	d.left[ll]--
	// Read next value.
	majorByte, err := d.r.Readn1()
	if err != nil {
		return true, err
	}
	tokenSlot.Tagged = false
	_, err = d.stepHelper_acceptValue(majorByte, tokenSlot)
	return false, err
}

// Step in midst of decoding an definite-length map, key expected up next.
func (d *Decoder) step_acceptMapKey(tokenSlot *Token) (done bool, err error) {
	// Yield close token, pop state, and return done flag if expecting no more entries.
	ll := len(d.left) - 1
	if d.left[ll] == 0 {
		d.left = d.left[0:ll]
		tokenSlot.Type = TMapClose
		return true, nil
	}
	d.left[ll]--
	// Read next key.
	majorByte, err := d.r.Readn1()
	if err != nil {
		return true, err
	}
	d.step = d.step_acceptMapValue
	tokenSlot.Tagged = false
	_, err = d.stepHelper_acceptValue(majorByte, tokenSlot) // FIXME surely not *any* value?  not composites, at least?
	return false, err
}

// Step in midst of decoding an definite-length map, value expected up next.
func (d *Decoder) step_acceptMapValue(tokenSlot *Token) (done bool, err error) {
	// Read next value.
	majorByte, err := d.r.Readn1()
	if err != nil {
		return true, err
	}
	d.step = d.step_acceptMapKey
	tokenSlot.Tagged = false
	_, err = d.stepHelper_acceptValue(majorByte, tokenSlot)
	return false, err
}

func (d *Decoder) stepHelper_acceptValue(majorByte byte, tokenSlot *Token) (done bool, err error) {
	switch majorByte {
	case cborSigilNil:
		tokenSlot.Type = TNull
		return true, nil
	case cborSigilFalse:
		tokenSlot.Type = TBool
		tokenSlot.Bool = false
		return true, nil
	case cborSigilTrue:
		tokenSlot.Type = TBool
		tokenSlot.Bool = true
		return true, nil
	case cborSigilFloat16, cborSigilFloat32, cborSigilFloat64:
		tokenSlot.Type = TFloat64
		tokenSlot.Float64, err = d.decodeFloat(majorByte)
		return true, err
	case cborSigilIndefiniteBytes:
		tokenSlot.Type = TBytes
		tokenSlot.Bytes, err = d.decodeBytesIndefinite(nil)
		return true, err
	case cborSigilIndefiniteString:
		tokenSlot.Type = TString
		tokenSlot.Str, err = d.decodeStringIndefinite()
		return true, err
	case cborSigilIndefiniteArray:
		tokenSlot.Type = TArrOpen
		tokenSlot.Length = -1
		d.pushPhase(d.step_acceptArrValueOrBreak)
		return false, nil
	case cborSigilIndefiniteMap:
		tokenSlot.Type = TMapOpen
		tokenSlot.Length = -1
		d.pushPhase(d.step_acceptMapIndefKey)
		return false, nil
	default:
		switch {
		case majorByte >= cborMajorUint && majorByte < cborMajorNegInt:
			tokenSlot.Type = TUint
			tokenSlot.Uint, err = d.decodeUint(majorByte)
			return true, err
		case majorByte >= cborMajorNegInt && majorByte < cborMajorBytes:
			tokenSlot.Type = TInt
			tokenSlot.Int, err = d.decodeNegInt(majorByte)
			return true, err
		case majorByte >= cborMajorBytes && majorByte < cborMajorString:
			tokenSlot.Type = TBytes
			tokenSlot.Bytes, err = d.decodeBytes(majorByte)
			return true, err
		case majorByte >= cborMajorString && majorByte < cborMajorArray:
			tokenSlot.Type = TString
			tokenSlot.Str, err = d.decodeString(majorByte)
			return true, err
		case majorByte >= cborMajorArray && majorByte < cborMajorMap:
			var n int
			n, err = d.decodeLen(majorByte)
			tokenSlot.Type = TArrOpen
			tokenSlot.Length = n
			d.left = append(d.left, n)
			d.pushPhase(d.step_acceptArrValue)
			return false, err
		case majorByte >= cborMajorMap && majorByte < cborMajorTag:
			var n int
			n, err = d.decodeLen(majorByte)
			tokenSlot.Type = TMapOpen
			tokenSlot.Length = n
			d.left = append(d.left, n)
			d.pushPhase(d.step_acceptMapKey)
			return false, err
		case majorByte >= cborMajorTag && majorByte < cborMajorSimple:
			// CBOR tags are, frankly, bonkers, and should not be used.
			// They break isomorphism to basic standards like JSON.
			// We'll parse basic integer tag values -- SINGLE layer only.
			// We will NOT parse the full gamut of recursive tags: doing so
			// would mean allowing an unbounded number of allocs *during
			// *processing of a single token*, which is _not reasonable_.
			if tokenSlot.Tagged {
				return true, fmt.Errorf("unsupported multiple tags on a single data item")
			}
			tokenSlot.Tagged = true
			tokenSlot.Tag, err = d.decodeLen(majorByte)
			if err != nil {
				return true, err
			}
			// Okay, we slurped a tag.
			// Read next value.
			majorByte, err := d.r.Readn1()
			if err != nil {
				return true, err
			}
			return d.stepHelper_acceptValue(majorByte, tokenSlot)
		default:
			return true, fmt.Errorf("Invalid majorByte: 0x%x", majorByte)
		}
	}
}
