package net

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"golang.org/x/crypto/hkdf"
	libp2p "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
	"io"
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
)

var (
	// The ciphertext cannot be shorter than CiphertextVersionBytes + EncryptedSecretKeyBytes + aes.BlockSize + MacKeyBytes
	ErrShortCiphertext = errors.New("Ciphertext is too short")

	// The HMAC included in the ciphertext is invalid
	ErrInvalidHmac = errors.New("Invalid Hmac")

	// Satic salt used in the hdkf
	Salt = []byte("OpenBazaar Encryption Algorithm")
)

func Encrypt(pubKey libp2p.PubKey, plaintext []byte) ([]byte, error) {

	// Encrypt random secret key with RSA pubkey
	secretKey := make([]byte, SecretKeyBytes)
	rand.Read(secretKey)

	pubKeyBytes, err := pubKey.Bytes()
	if err != nil {
		return nil, err
	}
	rsaPubKey, err := libp2p.UnmarshalRsaPublicKey(pubKeyBytes)
	if err != nil {
		return nil, err
	}

	encKey, err := rsaPubKey.Encrypt(secretKey)
	if err != nil {
		return nil, err
	}

	// Derive MAC and AES keys from the secret key using hkdf
	hash := sha256.New

	hkdf := hkdf.New(hash, secretKey, Salt, nil)

	aesKey := make([]byte, AESKeyBytes)
	_, err = io.ReadFull(hkdf, aesKey)
	if err != nil {
		return nil, err
	}
	macKey := make([]byte, MacKeyBytes)
	_, err = io.ReadFull(hkdf, macKey)
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
	version := getCipherTextVersion(ciphertext)
	if version == CiphertextVersion {
		return decryptV1(privKey, ciphertext)
	} else {
		return nil, errors.New("Unknown ciphertext version")
	}
}

func decryptV1(privKey libp2p.PrivKey, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < CiphertextVersionBytes+EncryptedSecretKeyBytes+aes.BlockSize+MacKeyBytes {
		return nil, ErrShortCiphertext
	}
	privKeyBytes, err := privKey.Bytes()
	if err != nil {
		return nil, err
	}
	rsaPrivKey, err := libp2p.UnmarshalRsaPrivateKey(privKeyBytes)
	if err != nil {
		return nil, err
	}

	// Decrypt the secret key using the RSA private key
	secretKey, err := rsaPrivKey.Decrypt(ciphertext[CiphertextVersionBytes : CiphertextVersionBytes+EncryptedSecretKeyBytes])
	if err != nil {
		return nil, err
	}

	// Derive the AES and MAC keys from the secret key using hdkf
	hash := sha256.New

	hkdf := hkdf.New(hash, secretKey, Salt, nil)

	aesKey := make([]byte, AESKeyBytes)
	_, err = io.ReadFull(hkdf, aesKey)
	if err != nil {
		return nil, err
	}
	macKey := make([]byte, MacKeyBytes)
	_, err = io.ReadFull(hkdf, macKey)
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

func getCipherTextVersion(ciphertext []byte) uint32 {
	return binary.BigEndian.Uint32(ciphertext[:CiphertextVersionBytes])
}
