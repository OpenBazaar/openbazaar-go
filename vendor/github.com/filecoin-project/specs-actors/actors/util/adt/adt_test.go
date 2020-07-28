package adt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/specs-actors/actors/util/adt"
	tutil "github.com/filecoin-project/specs-actors/support/testing"
)

func TestAddrKey(t *testing.T) {
	id_address_1 := tutil.NewIDAddr(t, 101)
	id_address_2 := tutil.NewIDAddr(t, 102)
	actor_address_1 := tutil.NewActorAddr(t, "actor1")
	actor_address_2 := tutil.NewActorAddr(t, "222")

	t.Run("address to key string conversion", func(t *testing.T) {
		assert.Equal(t, "\x00\x65", adt.AddrKey(id_address_1).Key())
		assert.Equal(t, "\x00\x66", adt.AddrKey(id_address_2).Key())
		assert.Equal(t, "\x02\x58\xbe\x4f\xd7\x75\xa0\xc8\xcd\x9a\xed\x86\x4e\x73\xab\xb1\x86\x46\x5f\xef\xe1", adt.AddrKey(actor_address_1).Key())
		assert.Equal(t, "\x02\xaa\xd0\xb2\x98\xa9\xde\xab\xbb\xb6\u007f\x80\x5f\x66\xaa\x68\x8c\xdd\x89\xad\xf5", adt.AddrKey(actor_address_2).Key())
	})
}
