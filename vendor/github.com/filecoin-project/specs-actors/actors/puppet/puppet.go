package puppet

import (
	"fmt"
	"io"

	addr "github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"

	abi "github.com/filecoin-project/specs-actors/actors/abi"
	builtin "github.com/filecoin-project/specs-actors/actors/builtin"
	runtime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	adt "github.com/filecoin-project/specs-actors/actors/util/adt"
)

// The Puppet Actor exists to aid testing the runtime and environment in which it's embedded. It provides direct access
// to the runtime methods, including sending arbitrary messages to other actors, without any preconditions or invariants
// to get in the way.
type Actor struct{}

func (a Actor) Exports() []interface{} {
	return []interface{}{
		builtin.MethodConstructor: a.Constructor,
		2:                         a.Send,
		3:                         a.SendMarshalCBORFailure,
		4:                         a.ReturnMarshalCBORFailure,
		5:                         a.RuntimeTransactionMarshalCBORFailure,
	}
}

var _ abi.Invokee = Actor{}

func (a Actor) Constructor(rt runtime.Runtime, _ *adt.EmptyValue) *adt.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	rt.State().Create(&State{})
	return nil
}

type SendParams struct {
	To     addr.Address
	Value  abi.TokenAmount
	Method abi.MethodNum
	Params []byte
}

type SendReturn struct {
	Return runtime.CBORBytes
	Code   exitcode.ExitCode
}

func (a Actor) Send(rt runtime.Runtime, params *SendParams) *SendReturn {
	rt.ValidateImmediateCallerAcceptAny()
	ret, code := rt.Send(
		params.To,
		params.Method,
		runtime.CBORBytes(params.Params),
		params.Value,
	)
	out, err := handleSendReturn(ret)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to unmarshal send return: %v", err)
	}
	return &SendReturn{
		Return: out,
		Code:   code,
	}
}

func (a Actor) SendMarshalCBORFailure(rt runtime.Runtime, params *SendParams) *SendReturn {
	rt.ValidateImmediateCallerAcceptAny()
	ret, code := rt.Send(
		params.To,
		params.Method,
		&FailToMarshalCBOR{},
		params.Value,
	)
	out, err := handleSendReturn(ret)
	if err != nil {
		rt.Abortf(exitcode.ErrIllegalState, "failed to unmarshal send return: %v", err)
	}
	return &SendReturn{
		Return: out,
		Code:   code,
	}
}

func (a Actor) ReturnMarshalCBORFailure(rt runtime.Runtime, _ *adt.EmptyValue) *FailToMarshalCBOR {
	rt.ValidateImmediateCallerAcceptAny()
	return &FailToMarshalCBOR{}
}

func (a Actor) RuntimeTransactionMarshalCBORFailure(rt runtime.Runtime, _ *adt.EmptyValue) *adt.EmptyValue {
	rt.ValidateImmediateCallerAcceptAny()

	var st State

	rt.State().Transaction(&st, func() interface{} {
		st.OptFailToMarshalCBOR = []*FailToMarshalCBOR{{}}
		return nil
	})

	return nil
}

func handleSendReturn(ret runtime.SendReturn) (runtime.CBORBytes, error) {
	if ret != nil {
		var out runtime.CBORBytes
		if err := ret.Into(&out); err != nil {
			return nil, err
		}
		return out, nil
	}
	// nothing was returned
	return nil, nil
}

type FailToMarshalCBOR struct{}

func (t *FailToMarshalCBOR) UnmarshalCBOR(io.Reader) error {
	return fmt.Errorf("failed to unmarshal cbor")
}

func (t *FailToMarshalCBOR) MarshalCBOR(w io.Writer) error {
	return fmt.Errorf("failed to marshal cbor")
}

type State struct {
	// OptFailToMarshalCBOR is to be used as an Option<T> or Maybe<T>, with T
	// specialized to *FailToMarshalCBOR. If the slice contains no values, the
	// State struct will encode as CBOR without issue. If the slice contains
	// more than zero values, the CBOR encoding will fail.
	OptFailToMarshalCBOR []*FailToMarshalCBOR
}

func init() {
	builder := cid.V1Builder{Codec: cid.Raw, MhType: mh.IDENTITY}
	c, err := builder.Sum([]byte("fil/1/puppet"))
	if err != nil {
		panic(err)
	}
	PuppetActorCodeID = c
}

// The actor code ID & Methods
var PuppetActorCodeID cid.Cid

var MethodsPuppet = struct {
	Constructor                          abi.MethodNum
	Send                                 abi.MethodNum
	SendMarshalCBORFailure               abi.MethodNum
	ReturnMarshalCBORFailure             abi.MethodNum
	RuntimeTransactionMarshalCBORFailure abi.MethodNum
}{builtin.MethodConstructor, 2, 3, 4, 5}
