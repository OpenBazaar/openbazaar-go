package registry_test

import (
	"testing"

	"github.com/filecoin-project/go-data-transfer/registry"
	"github.com/filecoin-project/go-data-transfer/testutil"
	"github.com/stretchr/testify/require"
)

func TestRegistry(t *testing.T) {
	r := registry.NewRegistry()
	t.Run("it registers", func(t *testing.T) {
		err := r.Register(&testutil.FakeDTType{}, func() {})
		require.NoError(t, err)
	})
	t.Run("it errors when registred again", func(t *testing.T) {
		err := r.Register(&testutil.FakeDTType{}, func() {})
		require.EqualError(t, err, "identifier already registered: FakeDTType")
	})
	t.Run("it errors when decoder setup fails", func(t *testing.T) {
		err := r.Register(testutil.FakeDTType{}, func() {})
		require.EqualError(t, err, "registering entry type FakeDTType: type must be a pointer")
	})
	t.Run("it reads decoders", func(t *testing.T) {
		decoder, has := r.Decoder("FakeDTType")
		require.True(t, has)
		require.NotNil(t, decoder)
		decoder, has = r.Decoder("OtherType")
		require.False(t, has)
		require.Nil(t, decoder)
	})
	t.Run("it reads processors", func(t *testing.T) {
		processor, has := r.Processor("FakeDTType")
		require.True(t, has)
		require.NotNil(t, processor)
		processor, has = r.Processor("OtherType")
		require.False(t, has)
		require.Nil(t, processor)
	})

}
