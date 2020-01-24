package factory

import (
	"encoding/base64"
	"fmt"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/btcsuite/btcd/btcec"
	"github.com/golang/protobuf/jsonpb"
)

func NewPeerIDProtobuf() *pb.ID {
	privKey := MustNewBitcoinPrivKey()
	bitcoinSig, err := privKey.Sign([]byte("QmeJ3vRqsYVJXtFZr2MRo47KS9LStvW9g4LRK8uqGX2bt5"))
	if err != nil {
		panic(fmt.Sprintf("signing peerid: %s", err.Error()))
	}
	return &pb.ID{
		PeerID:     "QmeJ3vRqsYVJXtFZr2MRo47KS9LStvW9g4LRK8uqGX2bt5",
		Handle:     "",
		Pubkeys:    MustNewPubkeysProtobuf(privKey),
		BitcoinSig: bitcoinSig.Serialize(),
	}
}

func MustNewPubkeysIdentityKeyBytes() []byte {
	keyBytes, err := base64.StdEncoding.DecodeString("CAESIII6nbBUBtCkK0blWtYRwm2lKS4kuAm36sElyoeC0n0u")
	if err != nil {
		panic(err)
	}
	return keyBytes
}

func MustNewBitcoinPrivKey() *btcec.PrivateKey {
	priv, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		panic(err)
	}
	return priv
}

func MustNewPubkeysProtobuf(bitcoinKey *btcec.PrivateKey) *pb.ID_Pubkeys {
	if bitcoinKey == nil {
		panic("nil bitcoin pubkey cannot produce pubkey protobuf")
	}
	return &pb.ID_Pubkeys{
		Identity: MustNewPubkeysIdentityKeyBytes(),
		Bitcoin:  bitcoinKey.PubKey().SerializeCompressed(),
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
