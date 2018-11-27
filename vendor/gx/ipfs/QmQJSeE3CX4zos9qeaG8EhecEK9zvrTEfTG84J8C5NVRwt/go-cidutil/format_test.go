package cidutil

import (
	"fmt"
	"testing"

	c "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mb "gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"
)

func TestFmt(t *testing.T) {
	cids := map[string]string{
		"cidv0": "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn",
		"cidv1": "zdj7WfLr9DhLrb1hsoSi4fSdjjxuZmeqgEtBPWxMLtPbDNbFD",
	}
	tests := []struct {
		cidId   string
		newBase mb.Encoding
		fmtStr  string
		result  string
	}{
		{"cidv0", -1, "%P", "cidv0-protobuf-sha2-256-32"},
		{"cidv0", -1, "%b-%v-%c-%h-%L", "base58btc-cidv0-protobuf-sha2-256-32"},
		{"cidv0", -1, "%s", "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"},
		{"cidv0", -1, "%S", "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"},
		{"cidv0", -1, "ver#%V/#%C/#%H/%L", "ver#0/#112/#18/32"},
		{"cidv0", -1, "%m", "zQmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"},
		{"cidv0", -1, "%M", "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"},
		{"cidv0", -1, "%d", "z72gdmFAgRzYHkJzKiL8MgMMRW3BTSCGyDHroPxJbxMJn"},
		{"cidv0", -1, "%D", "72gdmFAgRzYHkJzKiL8MgMMRW3BTSCGyDHroPxJbxMJn"},
		{"cidv0", 'B', "%S", "CIQFTFEEHEDF6KLBT32BFAGLXEZL4UWFNWM4LFTLMXQBCERZ6CMLX3Y"},
		{"cidv0", 'B', "%B%S", "BCIQFTFEEHEDF6KLBT32BFAGLXEZL4UWFNWM4LFTLMXQBCERZ6CMLX3Y"},
		{"cidv1", -1, "%P", "cidv1-protobuf-sha2-256-32"},
		{"cidv1", -1, "%b-%v-%c-%h-%L", "base58btc-cidv1-protobuf-sha2-256-32"},
		{"cidv1", -1, "%s", "zdj7WfLr9DhLrb1hsoSi4fSdjjxuZmeqgEtBPWxMLtPbDNbFD"},
		{"cidv1", -1, "%S", "dj7WfLr9DhLrb1hsoSi4fSdjjxuZmeqgEtBPWxMLtPbDNbFD"},
		{"cidv1", -1, "ver#%V/#%C/#%H/%L", "ver#1/#112/#18/32"},
		{"cidv1", -1, "%m", "zQmYFbmndVP7QqAVWyKhpmMuQHMaD88pkK57RgYVimmoh5H"},
		{"cidv1", -1, "%M", "QmYFbmndVP7QqAVWyKhpmMuQHMaD88pkK57RgYVimmoh5H"},
		{"cidv1", -1, "%d", "zAux4gVVsLRMXtsZ9fd3tFEZN4jGYB6kP37fgoZNTc11H"},
		{"cidv1", -1, "%D", "Aux4gVVsLRMXtsZ9fd3tFEZN4jGYB6kP37fgoZNTc11H"},
		{"cidv1", 'B', "%s", "BAFYBEIETJGSRL3EQPQPCABV3G6IUBYTSIFVQ24XRRHD3JUETSKLTGQ7DJA"},
		{"cidv1", 'B', "%S", "AFYBEIETJGSRL3EQPQPCABV3G6IUBYTSIFVQ24XRRHD3JUETSKLTGQ7DJA"},
		{"cidv1", 'B', "%B%S", "BAFYBEIETJGSRL3EQPQPCABV3G6IUBYTSIFVQ24XRRHD3JUETSKLTGQ7DJA"},
	}
	for _, tc := range tests {
		name := fmt.Sprintf("%s/%s", tc.cidId, tc.fmtStr)
		if tc.newBase != -1 {
			name = fmt.Sprintf("%s/%c", name, tc.newBase)
		}
		cidStr := cids[tc.cidId]
		t.Run(name, func(t *testing.T) {
			testFmt(t, cidStr, tc.newBase, tc.fmtStr, tc.result)
		})
	}
}

func testFmt(t *testing.T, cidStr string, newBase mb.Encoding, fmtStr string, result string) {
	cid, err := c.Decode(cidStr)
	if err != nil {
		t.Fatal(err)
	}
	base := newBase
	if newBase == -1 {
		base, _ = c.ExtractEncoding(cidStr)
	}
	str, err := Format(fmtStr, base, cid)
	if err != nil {
		t.Fatal(err)
	}
	if str != result {
		t.Error(fmt.Sprintf("expected: %s; but got: %s", result, str))
	}
}
