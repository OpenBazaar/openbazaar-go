package ipfs

import (
	"encoding/base64"
	"encoding/binary"
	"github.com/ipfs/go-ipfs/repo/config"
	"golang.org/x/crypto/scrypt"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	libp2p "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
)

func IdentityFromKey(privkey []byte) (config.Identity, error) {
	ident := config.Identity{}
	sk, err := libp2p.UnmarshalPrivateKey(privkey)
	if err != nil {
		return ident, err
	}
	skbytes, err := sk.Bytes()
	if err != nil {
		return ident, err
	}
	ident.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	id, err := peer.IDFromPublicKey(sk.GetPublic())
	if err != nil {
		return ident, err
	}
	ident.PeerID = id.Pretty()
	return ident, nil
}

func IdentityKeyFromSeed(seed []byte, bits int) ([]byte, error) {
	reader := &DeterministicReader{Seed: seed, Counter: 0}
	sk, _, err := libp2p.GenerateKeyPairWithReader(libp2p.RSA, bits, reader)
	if err != nil {
		return nil, err
	}
	encodedKey, err := libp2p.MarshalPrivateKey(sk)
	if err != nil {
		return nil, err
	}
	return encodedKey, nil
}

type DeterministicReader struct {
	Seed    []byte
	Counter uint64
}

func (d *DeterministicReader) Read(p []byte) (n int, err error) {
	l := len(p)
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, d.Counter)
	dk, err := scrypt.Key(d.Seed, counterBytes, 512, 8, 1, l)
	copy(p, dk)
	d.Counter++
	return l, nil
}
