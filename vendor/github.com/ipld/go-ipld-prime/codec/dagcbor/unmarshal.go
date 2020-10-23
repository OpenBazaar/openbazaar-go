package dagcbor

import (
	"errors"
	"fmt"
	"math"

	cid "github.com/ipfs/go-cid"
	"github.com/polydawn/refmt/shared"
	"github.com/polydawn/refmt/tok"

	ipld "github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

var (
	ErrInvalidMultibase = errors.New("invalid multibase on IPLD link")
)

// This should be identical to the general feature in the parent package,
// except for the `case tok.TBytes` block,
// which has dag-cbor's special sauce for detecting schemafree links.

func Unmarshal(na ipld.NodeAssembler, tokSrc shared.TokenSource) error {
	var tk tok.Token
	done, err := tokSrc.Step(&tk)
	if err != nil {
		return err
	}
	if done && !tk.Type.IsValue() {
		return fmt.Errorf("unexpected eof")
	}
	return unmarshal(na, tokSrc, &tk)
}

// starts with the first token already primed.  Necessary to get recursion
//  to flow right without a peek+unpeek system.
func unmarshal(na ipld.NodeAssembler, tokSrc shared.TokenSource, tk *tok.Token) error {
	// FUTURE: check for schema.TypedNodeBuilder that's going to parse a Link (they can slurp any token kind they want).
	switch tk.Type {
	case tok.TMapOpen:
		expectLen := tk.Length
		allocLen := tk.Length
		if tk.Length == -1 {
			expectLen = math.MaxInt32
			allocLen = 0
		}
		ma, err := na.BeginMap(allocLen)
		if err != nil {
			return err
		}
		observedLen := 0
		for {
			_, err := tokSrc.Step(tk)
			if err != nil {
				return err
			}
			switch tk.Type {
			case tok.TMapClose:
				if expectLen != math.MaxInt32 && observedLen != expectLen {
					return fmt.Errorf("unexpected mapClose before declared length")
				}
				return ma.Finish()
			case tok.TString:
				// continue
			default:
				return fmt.Errorf("unexpected %s token while expecting map key", tk.Type)
			}
			observedLen++
			if observedLen > expectLen {
				return fmt.Errorf("unexpected continuation of map elements beyond declared length")
			}
			mva, err := ma.AssembleEntry(tk.Str)
			if err != nil { // return in error if the key was rejected
				return err
			}
			err = Unmarshal(mva, tokSrc)
			if err != nil { // return in error if some part of the recursion errored
				return err
			}
		}
	case tok.TMapClose:
		return fmt.Errorf("unexpected mapClose token")
	case tok.TArrOpen:
		expectLen := tk.Length
		allocLen := tk.Length
		if tk.Length == -1 {
			expectLen = math.MaxInt32
			allocLen = 0
		}
		la, err := na.BeginList(allocLen)
		if err != nil {
			return err
		}
		observedLen := 0
		for {
			_, err := tokSrc.Step(tk)
			if err != nil {
				return err
			}
			switch tk.Type {
			case tok.TArrClose:
				if expectLen != math.MaxInt32 && observedLen != expectLen {
					return fmt.Errorf("unexpected arrClose before declared length")
				}
				return la.Finish()
			default:
				observedLen++
				if observedLen > expectLen {
					return fmt.Errorf("unexpected continuation of array elements beyond declared length")
				}
				err := unmarshal(la.AssembleValue(), tokSrc, tk)
				if err != nil { // return in error if some part of the recursion errored
					return err
				}
			}
		}
	case tok.TArrClose:
		return fmt.Errorf("unexpected arrClose token")
	case tok.TNull:
		return na.AssignNull()
	case tok.TString:
		return na.AssignString(tk.Str)
	case tok.TBytes:
		if !tk.Tagged {
			return na.AssignBytes(tk.Bytes)
		}
		switch tk.Tag {
		case linkTag:
			if tk.Bytes[0] != 0 {
				return ErrInvalidMultibase
			}
			elCid, err := cid.Cast(tk.Bytes[1:])
			if err != nil {
				return err
			}
			return na.AssignLink(cidlink.Link{elCid})
		default:
			return fmt.Errorf("unhandled cbor tag %d", tk.Tag)
		}
	case tok.TBool:
		return na.AssignBool(tk.Bool)
	case tok.TInt:
		return na.AssignInt(int(tk.Int)) // FIXME overflow check
	case tok.TUint:
		return na.AssignInt(int(tk.Uint)) // FIXME overflow check
	case tok.TFloat64:
		return na.AssignFloat(tk.Float64)
	default:
		panic("unreachable")
	}
}
