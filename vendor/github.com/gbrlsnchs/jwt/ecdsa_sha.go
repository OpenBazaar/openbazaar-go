package jwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"

	"github.com/gbrlsnchs/jwt/v3/internal"
)

var (
	// ErrECDSANilPrivKey is the error for trying to sign a JWT with a nil private key.
	ErrECDSANilPrivKey = internal.NewError("jwt: ECDSA private key is nil")
	// ErrECDSANilPubKey is the error for trying to verify a JWT with a nil public key.
	ErrECDSANilPubKey = internal.NewError("jwt: ECDSA public key is nil")
	// ErrECDSAVerification is the error for an invalid ECDSA signature.
	ErrECDSAVerification = internal.NewError("jwt: ECDSA verification failed")

	_ Algorithm = new(ECDSASHA)
)

// ECDSAPrivateKey is an option to set a private key to the ECDSA-SHA algorithm.
func ECDSAPrivateKey(priv *ecdsa.PrivateKey) func(*ECDSASHA) {
	return func(es *ECDSASHA) {
		es.priv = priv
	}
}

// ECDSAPublicKey is an option to set a public key to the ECDSA-SHA algorithm.
func ECDSAPublicKey(pub *ecdsa.PublicKey) func(*ECDSASHA) {
	return func(es *ECDSASHA) {
		es.pub = pub
	}
}

func byteSize(bitSize int) int {
	byteSize := bitSize / 8
	if bitSize%8 > 0 {
		return byteSize + 1
	}
	return byteSize
}

// ECDSASHA is an algorithm that uses ECDSA to sign SHA hashes.
type ECDSASHA struct {
	name string
	priv *ecdsa.PrivateKey
	pub  *ecdsa.PublicKey
	sha  crypto.Hash
	size int

	pool *hashPool
}

func newECDSASHA(name string, opts []func(*ECDSASHA), sha crypto.Hash) *ECDSASHA {
	es := ECDSASHA{
		name: name,
		sha:  sha,
		pool: newHashPool(sha.New),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&es)
		}
	}
	if es.pub == nil {
		if es.priv == nil {
			panic(ErrECDSANilPrivKey)
		}
		es.pub = &es.priv.PublicKey
	}
	es.size = byteSize(es.pub.Params().BitSize) * 2
	return &es
}

// NewES256 creates a new algorithm using ECDSA and SHA-256.
func NewES256(opts ...func(*ECDSASHA)) *ECDSASHA {
	return newECDSASHA("ES256", opts, crypto.SHA256)
}

// NewES384 creates a new algorithm using ECDSA and SHA-384.
func NewES384(opts ...func(*ECDSASHA)) *ECDSASHA {
	return newECDSASHA("ES384", opts, crypto.SHA384)
}

// NewES512 creates a new algorithm using ECDSA and SHA-512.
func NewES512(opts ...func(*ECDSASHA)) *ECDSASHA {
	return newECDSASHA("ES512", opts, crypto.SHA512)
}

// Name returns the algorithm's name.
func (es *ECDSASHA) Name() string {
	return es.name
}

// Sign signs headerPayload using the ECDSA-SHA algorithm.
func (es *ECDSASHA) Sign(headerPayload []byte) ([]byte, error) {
	if es.priv == nil {
		return nil, ErrECDSANilPrivKey
	}
	return es.sign(headerPayload)
}

// Size returns the signature's byte size.
func (es *ECDSASHA) Size() int {
	return es.size
}

// Verify verifies a signature based on headerPayload using ECDSA-SHA.
func (es *ECDSASHA) Verify(headerPayload, sig []byte) (err error) {
	if es.pub == nil {
		return ErrECDSANilPubKey
	}
	if sig, err = internal.DecodeToBytes(sig); err != nil {
		return err
	}
	byteSize := byteSize(es.pub.Params().BitSize)
	if len(sig) != byteSize*2 {
		return ErrECDSAVerification
	}

	r := big.NewInt(0).SetBytes(sig[:byteSize])
	s := big.NewInt(0).SetBytes(sig[byteSize:])
	sum, err := es.pool.sign(headerPayload)
	if err != nil {
		return err
	}
	if !ecdsa.Verify(es.pub, sum, r, s) {
		return ErrECDSAVerification
	}
	return nil
}

func (es *ECDSASHA) sign(headerPayload []byte) ([]byte, error) {
	sum, err := es.pool.sign(headerPayload)
	if err != nil {
		return nil, err
	}
	r, s, err := ecdsa.Sign(rand.Reader, es.priv, sum)
	if err != nil {
		return nil, err
	}
	byteSize := byteSize(es.priv.Params().BitSize)
	rbytes := r.Bytes()
	rsig := make([]byte, byteSize)
	copy(rsig[byteSize-len(rbytes):], rbytes)

	sbytes := s.Bytes()
	ssig := make([]byte, byteSize)
	copy(ssig[byteSize-len(sbytes):], sbytes)
	return append(rsig, ssig...), nil
}
