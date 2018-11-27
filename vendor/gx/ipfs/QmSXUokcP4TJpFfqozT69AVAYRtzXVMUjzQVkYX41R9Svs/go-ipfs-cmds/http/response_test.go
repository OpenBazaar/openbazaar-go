package http

import (
	"testing"

	"gx/ipfs/QmSXUokcP4TJpFfqozT69AVAYRtzXVMUjzQVkYX41R9Svs/go-ipfs-cmds"
)

type testResponseType struct {
	a int
	b int
}

type testDecoder struct {
	a *int
	b *int
}

func (td *testDecoder) Decode(value interface{}) error {
	me := value.(*cmds.MaybeError)
	o := me.Value.(*testResponseType)

	if td.a != nil {
		o.a = *td.a
	}

	if td.b != nil {
		o.b = *td.b
	}

	return nil
}

func TestNextDecodesIntoNewStruct(t *testing.T) {
	a1 := 1
	b1 := 2
	testCommand := &cmds.Command{
		Type: &testResponseType{},
	}
	decoder := &testDecoder{
		a: &a1,
		b: &b1,
	}
	r := &cmds.Request{
		Command: testCommand,
	}
	response := &Response{
		req: r,
		dec: decoder,
	}

	v, err := response.Next()
	if err != nil {
		t.Fatal("error decoding response", err)
	}

	tv := v.(*testResponseType)
	if tv.a != 1 {
		t.Errorf("tv.a is %#v, expected 1", tv.a)
	}
	if tv.b != 2 {
		t.Errorf("tv.b is %#v, expected 2", tv.b)
	}

	a2 := 3
	decoder.a = &a2
	decoder.b = nil

	v2, err := response.Next()
	if err != nil {
		t.Fatal("error decoding response", err)
	}

	tv2 := v2.(*testResponseType)
	if tv2.a != 3 {
		t.Errorf("tv2.a is %#v, expected 3", tv2.a)
	}
	if tv2.b != 0 {
		t.Errorf("tv.b is %#v, expected it to be reset to 0", tv2.b)
	}
}
