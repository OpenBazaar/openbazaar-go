package factory

import (
	"encoding/base64"
	"fmt"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/jsonpb"
)

func NewPeerIDProtobuf() *pb.ID {
	return &pb.ID{
		PeerID:  "QmeJ3vRqsYVJXtFZr2MRo47KS9LStvW9g4LRK8uqGX2bt5",
		Handle:  "",
		Pubkeys: NewPubkeysProtobuf(),
		//BitcoinSig: []byte("MEQCIGqBDqGyLGs8tVewab+b8BIMCY73uGrxg7wPro3+JuVmAiBa2wk55FwGWWQoyLYW1mGhP62FLHyk6pfkAt59A1tU8Q=="),
	}
}

func NewPubkeysIdentityKeyBytes() []byte {
	keyBytes, err := base64.StdEncoding.DecodeString("CAESIII6nbBUBtCkK0blWtYRwm2lKS4kuAm36sElyoeC0n0u")
	if err != nil {
		panic(err)
	}
	return keyBytes
}

func NewPubkeysProtobuf() *pb.ID_Pubkeys {
	return &pb.ID_Pubkeys{
		Identity: NewPubkeysIdentityKeyBytes(),
		Bitcoin:  []byte("AwD4y8eIx7F0bnwNmssZGi+XFqydypxuFRtA4TPyWiqJ"),
	}
}

// MustNewPeerIDProtobuf returns a PeerID protobuf example that is known to be valid
func MustNewPeerIDProtobuf() *pb.ID {
	var idJSON = `{
		"peerID": "QmSsRdJtKLHueUA6vsjZVoZo6N6fjQrMjao29CcVW7pX4g",
		"pubkeys": {
			"identity": "CAESIFD3dGlUgpYv1RsEIwZPriU/NnKLjOXOFPolx35Be6ff",
			"bitcoin": "AgcLRxnvq37Yt3nCpez8Sj7Y7fzdpkJpULQh/B3vzl7C"
		},
		"bitcoinSig": "MEQCIEf8jOQquW3yCXo29NTdhMyh5pIcGTOgZSYJVL4QO5kyAiAk25Bl1q1SktRev4Oo+ZwAuSuNCM1YwtndqZp/0ET/ow=="
}`
	pbID := new(pb.ID)
	if err := jsonpb.UnmarshalString(idJSON, pbID); err != nil {
		panic(err)
	}
	return pbID
}

// NewValidPeerInfo returns a PeerInfo example that is known to be valid
func MustNewValidPeerInfo() *repo.PeerInfo {
	var p = repo.NewPeerInfoFromProtobuf(MustNewPeerIDProtobuf())
	if err := p.Valid(); err != nil {
		panic(fmt.Sprintf("invalid peer: %+v", err))
	}
	return p
}
