package encoding_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-data-transfer/encoding"
	"github.com/filecoin-project/go-data-transfer/encoding/testdata"
)

func TestRoundTrip(t *testing.T) {
	testCases := map[string]struct {
		val encoding.Encodable
	}{
		"can encode/decode IPLD prime types": {
			val: testdata.Prime,
		},
		"can encode/decode cbor-gen types": {
			val: testdata.Cbg,
		},
		"can encode/decode old ipld format types": {
			val: testdata.Standard,
		},
	}
	for testCase, data := range testCases {
		t.Run(testCase, func(t *testing.T) {
			encoded, err := encoding.Encode(data.val)
			require.NoError(t, err)
			decoder, err := encoding.NewDecoder(data.val)
			require.NoError(t, err)
			decoded, err := decoder.DecodeFromCbor(encoded)
			require.NoError(t, err)
			require.Equal(t, data.val, decoded)
		})
	}
}
