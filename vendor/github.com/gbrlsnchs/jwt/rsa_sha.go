package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"

	"github.com/gbrlsnchs/jwt/v3/internal"
)

var (
	// ErrRSANilPrivKey is the error for trying to sign a JWT with a nil private key.
	ErrRSANilPrivKey = internal.NewError("jwt: RSA private key is nil")
	// ErrRSANilPubKey is the error for trying to verify a JWT with a nil public key.
	ErrRSANilPubKey = internal.NewError("jwt: RSA public key is nil")
	// ErrRSAVerification is the error for an invalid RSA signature.
	ErrRSAVerification = internal.NewError("jwt: RSA verification failed")

	_ Algorithm = new(RSASHA)
)

// RSAPrivateKey is an option to set a private key to the RSA-SHA algorithm.
func RSAPrivateKey(priv *rsa.PrivateKey) func(*RSASHA) {
	return func(rs *RSASHA) {
		rs.priv = priv
	}
}

// RSAPublicKey is an option to set a public key to the RSA-SHA algorithm.
func RSAPublicKey(pub *rsa.PublicKey) func(*RSASHA) {
	return func(rs *RSASHA) {
		rs.pub = pub
	}
}

// RSASHA is an algorithm that uses RSA to sign SHA hashes.
type RSASHA struct {
	name string
	priv *rsa.PrivateKey
	pub  *rsa.PublicKey
	sha  crypto.Hash
	size int
	pool *hashPool
	opts *rsa.PSSOptions
}

func newRSASHA(name string, opts []func(*RSASHA), sha crypto.Hash, pss bool) *RSASHA {
	rs := RSASHA{
		name: name, // cache name
		sha:  sha,
		pool: newHashPool(sha.New),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&rs)
		}
	}
	if rs.pub == nil {
		if rs.priv == nil {
			panic(ErrRSANilPrivKey)
		}
		rs.pub = &rs.priv.PublicKey
	}
	rs.size = rs.pub.Size() // cache size
	if pss {
		rs.opts = &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthAuto,
			Hash:       sha,
		}
	}
	return &rs
}

// NewRS256 creates a new algorithm using RSA and SHA-256.
func NewRS256(opts ...func(*RSASHA)) *RSASHA {
	return newRSASHA("RS256", opts, crypto.SHA256, false)
}

// NewRS384 creates a new algorithm using RSA and SHA-384.
func NewRS384(opts ...func(*RSASHA)) *RSASHA {
	return newRSASHA("RS384", opts, crypto.SHA384, false)
}

// NewRS512 creates a new algorithm using RSA and SHA-512.
func NewRS512(opts ...func(*RSASHA)) *RSASHA {
	return newRSASHA("RS512", opts, crypto.SHA512, false)
}

// NewPS256 creates a new algorithm using RSA-PSS and SHA-256.
func NewPS256(opts ...func(*RSASHA)) *RSASHA {
	return newRSASHA("PS256", opts, crypto.SHA256, true)
}

// NewPS384 creates a new algorithm using RSA-PSS and SHA-384.
func NewPS384(opts ...func(*RSASHA)) *RSASHA {
	return newRSASHA("PS384", opts, crypto.SHA384, true)
}

// NewPS512 creates a new algorithm using RSA-PSS and SHA-512.
func NewPS512(opts ...func(*RSASHA)) *RSASHA {
	return newRSASHA("PS512", opts, crypto.SHA512, true)
}

// Name returns the algorithm's name.
func (rs *RSASHA) Name() string {
	return rs.name
}

// Sign signs headerPayload using either RSA-SHA or RSA-PSS-SHA algorithms.
func (rs *RSASHA) Sign(headerPayload []byte) ([]byte, error) {
	if rs.priv == nil {
		return nil, ErrRSANilPrivKey
	}
	sum, err := rs.pool.sign(headerPayload)
	if err != nil {
		return nil, err
	}
	if rs.opts != nil {
		return rsa.SignPSS(rand.Reader, rs.priv, rs.sha, sum, rs.opts)
	}
	return rsa.SignPKCS1v15(rand.Reader, rs.priv, rs.sha, sum)
}

// Size returns the signature's byte size.
func (rs *RSASHA) Size() int {
	return rs.size
}

// Verify verifies a signature based on headerPayload using either RSA-SHA or RSA-PSS-SHA.
func (rs *RSASHA) Verify(headerPayload, sig []byte) (err error) {
	if rs.pub == nil {
		return ErrRSANilPubKey
	}
	if sig, err = internal.DecodeToBytes(sig); err != nil {
		return err
	}
	sum, err := rs.pool.sign(headerPayload)
	if err != nil {
		return err
	}
	if rs.opts != nil {
		err = rsa.VerifyPSS(rs.pub, rs.sha, sum, sig, rs.opts)
	} else {
		err = rsa.VerifyPKCS1v15(rs.pub, rs.sha, sum, sig)
	}
	if err != nil {
		return ErrRSAVerification
	}
	return nil
}
