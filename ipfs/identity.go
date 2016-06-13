package ipfs

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	libp2p "gx/ipfs/QmUEUu1CM8bxBJxc3ZLojAi8evhTr4byQogWstABet79oY/go-libp2p-crypto"
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"

	"github.com/ipfs/go-ipfs/repo/config"
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
	Counter int
}

// TODO: this is a place holder until we settle on a key expansion algorithm
func (d *DeterministicReader) Read(p []byte) (n int, err error) {
	l := len(p)
	deterministcBytes := []byte{}
	for {
		bs := make([]byte, 8)
		binary.BigEndian.PutUint64(bs, uint64(d.Counter))
		b := append(d.Seed, bs...)
		out := sha256.Sum256(b)
		deterministcBytes = append(deterministcBytes, out[:]...)
		if len(deterministcBytes) >= l {
			break
		}
		d.Counter++
	}
	for a := 0; a < l; a++ {
		p[a] = deterministcBytes[a]
	}
	return l, nil
}
