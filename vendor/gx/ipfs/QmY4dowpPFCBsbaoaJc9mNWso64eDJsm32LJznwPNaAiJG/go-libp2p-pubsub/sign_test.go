package pubsub

import (
	"testing"

	pb "gx/ipfs/QmY4dowpPFCBsbaoaJc9mNWso64eDJsm32LJznwPNaAiJG/go-libp2p-pubsub/pb"

	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
)

func TestSigning(t *testing.T) {
	privk, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		t.Fatal(err)
	}
	testSignVerify(t, privk)

	privk, _, err = crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		t.Fatal(err)
	}
	testSignVerify(t, privk)
}

func testSignVerify(t *testing.T, privk crypto.PrivKey) {
	id, err := peer.IDFromPublicKey(privk.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	m := pb.Message{
		Data:     []byte("abc"),
		TopicIDs: []string{"foo"},
		From:     []byte(id),
		Seqno:    []byte("123"),
	}
	signMessage(id, privk, &m)
	err = verifyMessageSignature(&m)
	if err != nil {
		t.Fatal(err)
	}
}
