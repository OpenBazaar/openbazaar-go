package hamt

import (
	"fmt"
	"io"

	cid "github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var keyZero = []byte("0")
var keyOne = []byte("1")

func (t *Pointer) MarshalCBOR(w io.Writer) error {
	if t.Link != cid.Undef && len(t.KVs) > 0 {
		return fmt.Errorf("hamt Pointer cannot have both a link and KVs")
	}

	scratch := make([]byte, 9)

	if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajMap, 1); err != nil {
		return err
	}

	if t.Link != cid.Undef {
		// key for links is "0"
		// Refmt (and the general IPLD data model currently) can't deal
		// with non string keys. So we have this weird restriction right now
		// hoping to be able to use integer keys soon
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, 1); err != nil {
			return err
		}

		if _, err := w.Write(keyZero); err != nil {
			return err
		}

		if err := cbg.WriteCidBuf(scratch, w, t.Link); err != nil {
			return err
		}
	} else {
		// key for KVs is "1"
		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajTextString, 1); err != nil {
			return err
		}

		if _, err := w.Write(keyOne); err != nil {
			return err
		}

		if err := cbg.WriteMajorTypeHeaderBuf(scratch, w, cbg.MajArray, uint64(len(t.KVs))); err != nil {
			return err
		}

		for _, kv := range t.KVs {
			if err := kv.MarshalCBOR(w); err != nil {
				return err
			}
		}
	}

	return nil
}

func (t *Pointer) UnmarshalCBOR(br io.Reader) error {
	scratch := make([]byte, 8)

	maj, extra, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}
	if maj != cbg.MajMap {
		return fmt.Errorf("cbor input should be of map")
	}

	if extra != 1 {
		return fmt.Errorf("Pointers should be a single element map")
	}

	maj, val, err := cbg.CborReadHeaderBuf(br, scratch)
	if err != nil {
		return err
	}

	if maj != cbg.MajTextString {
		return fmt.Errorf("expected text string key")
	}

	if val != 1 {
		return fmt.Errorf("map keys in pointers must be a single byte long")
	}

	if _, err := io.ReadAtLeast(br, scratch[:1], 1); err != nil {
		return err
	}

	switch scratch[0] {
	case '0':
		c, err := cbg.ReadCid(br)
		if err != nil {
			return err
		}
		t.Link = c
		return nil
	case '1':
		maj, length, err := cbg.CborReadHeaderBuf(br, scratch)
		if err != nil {
			return err
		}

		if maj != cbg.MajArray {
			return fmt.Errorf("expected an array of KVs in cbor input")
		}

		if length > 32 {
			return fmt.Errorf("KV array in cbor input for pointer was too long")
		}

		t.KVs = make([]*KV, length)
		for i := 0; i < int(length); i++ {
			var kv KV
			if err := kv.UnmarshalCBOR(br); err != nil {
				return err
			}

			t.KVs[i] = &kv
		}

		return nil
	default:
		return fmt.Errorf("invalid pointer map key in cbor input: %d", val)
	}
}
