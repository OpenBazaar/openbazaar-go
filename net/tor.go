package net

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"github.com/yawning/bulb"
	"github.com/yawning/bulb/utils/pkcs1"
	"os"
	"path"
	"path/filepath"
)

// Return the Tor control port if Tor is running or an error
func GetTorControlPort() (int, error) {
	conn, err := bulb.Dial("tcp4", "127.0.0.1:9151")
	if err == nil {
		conn.Close()
		return 9151, nil
	}
	conn, err = bulb.Dial("tcp4", "127.0.0.1:9051")
	if err == nil {
		conn.Close()
		return 9051, nil
	}
	return 0, errors.New("Tor control unavailable")
}

// Generate a new RSA key and onion address and save it to the repo
func CreateHiddenServiceKey(repoPath string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		log.Fatalf("Failed to generate RSA key")
	}
	id, err := pkcs1.OnionAddr(&priv.PublicKey)
	if err != nil {
		log.Fatalf("Failed to derive onion ID: %v", err)
	}

	f, err := os.Create(path.Join(repoPath, id+".onion_key"))
	if err != nil {
		return err
	}
	defer f.Close()

	privKeyBytes, err := pkcs1.EncodePrivateKeyDER(priv)
	if err != nil {
		return err
	}

	block := pem.Block{Type: "RSA PRIVATE KEY", Bytes: privKeyBytes}
	err = pem.Encode(f, &block)
	if err != nil {
		return err
	}
	return nil
}

// Generate a new key pair if one does not already exist
func MaybeCreateHiddenServiceKey(repoPath string) error {
	d, err := os.Open(repoPath)
	if err != nil {
		return err
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		return err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".onion_key" {
			return nil
		}
	}

	return CreateHiddenServiceKey(repoPath)
}
