package net

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	libp2p "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	"io"

	extra "github.com/agl/ed25519/extra25519"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/box"
)

const (
	// The version of the encryption algorithm used. Currently only 1 is supported
	CiphertextVersion = 1

	// Length of the serialized version in bytes
	CiphertextVersionBytes = 4

	// Length of the secret key used to generate the AES and MAC keys in bytes
	SecretKeyBytes = 32

	// Length of the AES key in bytes
	AESKeyBytes = 32

	// Length of the MAC key in bytes
	MacKeyBytes = 32

	// Length of the RSA encrypted secret key ciphertext in bytes
	EncryptedSecretKeyBytes = 512

	// Length of the MAC in bytes
	MacBytes = 32

	// Length of nacl nonce
	NonceBytes = 24

	// Length of nacl ephemeral public key
	EphemeralPublicKeyBytes = 32
)

var (
	// The ciphertext cannot be shorter than CiphertextVersionBytes + EncryptedSecretKeyBytes + aes.BlockSize + MacKeyBytes
	ErrShortCiphertext = errors.New("ciphertext is too short")

	// The HMAC included in the ciphertext is invalid
	ErrInvalidHmac = errors.New("invalid Hmac")

	// Nacl box decryption failed
	BoxDecryptionError = errors.New("failed to decrypt curve25519")

	// Satic salt used in the hdkf
	Salt = []byte("OpenBazaar Encryption Algorithm")
)

func Encrypt(pubKey libp2p.PubKey, plaintext []byte) ([]byte, error) {
	rsaPubkey, ok := pubKey.(*libp2p.RsaPublicKey)
	if ok {
		return encryptRSA(rsaPubkey, plaintext)
	}
	ed25519Pubkey, ok := pubKey.(*libp2p.Ed25519PublicKey)
	if ok {
		return encryptCurve25519(ed25519Pubkey, plaintext)
	}
	return nil, errors.New("could not determine key type")
}

func encryptCurve25519(pubKey *libp2p.Ed25519PublicKey, plaintext []byte) ([]byte, error) {
	// Generated ephemeral key pair
	ephemPub, ephemPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	// Convert recipient's key into curve25519
	rawBytes, err := pubKey.Raw()
	if err != nil {
		return nil, err
	}
	var raw [32]byte
	copy(raw[:], rawBytes)
	pk, err := pubkeyToCurve25519(raw)
	if err != nil {
		return nil, err
	}

	// Encrypt with nacl
	var ciphertext []byte
	var nonce [24]byte
	n := make([]byte, 24)
	_, err = rand.Read(n)
	if err != nil {
		return nil, err
	}
	copy(nonce[:], n)
	ciphertext = box.Seal(ciphertext, plaintext, &nonce, pk, ephemPriv)

	// Prepend the ephemeral public key
	ciphertext = append(ephemPub[:], ciphertext...)

	// Prepend nonce
	ciphertext = append(nonce[:], ciphertext...)
	return ciphertext, nil
}

func encryptRSA(pubKey *libp2p.RsaPublicKey, plaintext []byte) ([]byte, error) {
	// Encrypt random secret key with RSA pubkey
	secretKey := make([]byte, SecretKeyBytes)
	rand.Read(secretKey)

	encKey, err := pubKey.Encrypt(secretKey)
	if err != nil {
		return nil, err
	}

	// Derive MAC and AES keys from the secret key using hkdf
	hash := sha256.New

	hkdfReader := hkdf.New(hash, secretKey, Salt, nil)

	aesKey := make([]byte, AESKeyBytes)
	_, err = io.ReadFull(hkdfReader, aesKey)
	if err != nil {
		return nil, err
	}
	macKey := make([]byte, MacKeyBytes)
	_, err = io.ReadFull(hkdfReader, macKey)
	if err != nil {
		return nil, err
	}

	// Encrypt message with the AES key
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	/* The IV needs to be unique, but not secure. Therefore it is common to
	   include it at the beginning of the ciphertext. */
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	// Create the HMAC
	mac := hmac.New(sha256.New, macKey)
	mac.Write(ciphertext)
	messageMac := mac.Sum(nil)

	// Prepend the ciphertext with the encrypted secret key
	ciphertext = append(encKey, ciphertext...)

	// Prepend version
	version := make([]byte, CiphertextVersionBytes)
	binary.BigEndian.PutUint32(version, uint32(CiphertextVersion))
	ciphertext = append(version, ciphertext...)

	// Append the MAC
	ciphertext = append(ciphertext, messageMac...)
	return ciphertext, nil
}

func Decrypt(privKey libp2p.PrivKey, ciphertext []byte) ([]byte, error) {
	rsaPrivkey, ok := privKey.(*libp2p.RsaPrivateKey)
	if ok {
		return decryptRSA(rsaPrivkey, ciphertext)
	}
	ed25519Privkey, ok := privKey.(*libp2p.Ed25519PrivateKey)
	if ok {
		return decryptCurve25519(ed25519Privkey, ciphertext)
	}
	return nil, errors.New("could not determine key type")
}

func decryptCurve25519(privKey *libp2p.Ed25519PrivateKey, ciphertext []byte) ([]byte, error) {
	rawBytes, err := privKey.Raw()
	if err != nil {
		return nil, err
	}
	var raw [64]byte
	copy(raw[:], rawBytes)
	curve25519Privkey := privkeyToCurve25519(raw)
	var plaintext []byte

	n := ciphertext[:NonceBytes]
	ephemPubkeyBytes := ciphertext[NonceBytes : NonceBytes+EphemeralPublicKeyBytes]
	ct := ciphertext[NonceBytes+EphemeralPublicKeyBytes:]

	var ephemPubkey [32]byte
	copy(ephemPubkey[:], ephemPubkeyBytes)

	var nonce [24]byte
	copy(nonce[:], n)

	plaintext, success := box.Open(plaintext, ct, &nonce, &ephemPubkey, curve25519Privkey)
	if !success {
		return nil, BoxDecryptionError
	}
	return plaintext, nil
}

func decryptRSA(privKey *libp2p.RsaPrivateKey, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < CiphertextVersionBytes+EncryptedSecretKeyBytes+aes.BlockSize+MacKeyBytes {
		return nil, ErrShortCiphertext
	}

	// Decrypt the secret key using the RSA private key
	secretKey, err := privKey.Decrypt(ciphertext[CiphertextVersionBytes : CiphertextVersionBytes+EncryptedSecretKeyBytes])
	if err != nil {
		return nil, err
	}

	// Derive the AES and MAC keys from the secret key using hdkf
	hash := sha256.New

	hkdfReader := hkdf.New(hash, secretKey, Salt, nil)

	aesKey := make([]byte, AESKeyBytes)
	_, err = io.ReadFull(hkdfReader, aesKey)
	if err != nil {
		return nil, err
	}
	macKey := make([]byte, MacKeyBytes)
	_, err = io.ReadFull(hkdfReader, macKey)
	if err != nil {
		return nil, err
	}

	// Calculate the HMAC and verify it is correct
	mac := hmac.New(sha256.New, macKey)
	mac.Write(ciphertext[CiphertextVersionBytes+EncryptedSecretKeyBytes : len(ciphertext)-MacBytes])
	messageMac := mac.Sum(nil)
	if !hmac.Equal(messageMac, ciphertext[len(ciphertext)-MacBytes:]) {
		return nil, ErrInvalidHmac
	}

	// Decrypt the AES ciphertext
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	ciphertext = ciphertext[CiphertextVersionBytes+EncryptedSecretKeyBytes : len(ciphertext)-MacBytes]
	if len(ciphertext) < aes.BlockSize {
		return nil, err
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same
	stream.XORKeyStream(ciphertext, ciphertext)
	plaintext := ciphertext
	return plaintext, nil
}

func privkeyToCurve25519(sk [64]byte) *[32]byte {
	var skNew [32]byte
	extra.PrivateKeyToCurve25519(&skNew, &sk)
	return &skNew
}

func pubkeyToCurve25519(pk [32]byte) (*[32]byte, error) {
	var pkNew [32]byte
	success := extra.PublicKeyToCurve25519(&pkNew, &pk)
	if !success {
		return nil, fmt.Errorf("error converting ed25519 pubkey to curve25519 pubkey")
	}
	return &pkNew, nil
}
