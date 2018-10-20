package cmd_test

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/cmd"
)

func TestGenCertsGenericDefaults(t *testing.T) {

	if dataDir, err := ioutil.TempDir("", "gencerts_test"); err != nil {
		t.Fatal(err)
	} else {

		config := cmd.GenerateCertificates{
			DataDir:  dataDir,
			Testnet:  true,
			Host:     "127.0.0.1",
			ValidFor: 10,
		}

		args := []string{"args?"}

		if err := config.Execute(args); err != nil {
			t.Fatal(err)
		}

		sslPath := path.Join(dataDir, "ssl")
		if fileInfo, err := os.Stat(sslPath); err != nil {
			t.Fatal(err)
		} else {
			if fileInfo.Mode().Perm() != 0755 {
				t.Fatal("ssl directory does not have 0755 permissions")
			}
			if !fileInfo.IsDir() {
				t.Fatalf("Expecting a directory: %s", dataDir)
			}
		}

		certPemPath := path.Join(dataDir, "ssl", "cert.pem")
		if fileInfo, err := os.Stat(certPemPath); err != nil {
			t.Fatal(err)
		} else {
			if fileInfo.Mode().Perm() != 0644 {
				t.Fatal("cert.pem does not have 0644 permissions")
			}
			if !fileInfo.Mode().IsRegular() {
				t.Fatalf("Expecting a file: %s", certPemPath)
			}
		}

		keyPemPath := path.Join(dataDir, "ssl", "key.pem")
		if fileInfo, err := os.Stat(keyPemPath); err != nil {
			t.Fatal(err)
		} else {
			if fileInfo.Mode().Perm() != 0600 {
				t.Fatal("cert.pem does not have 0644 permissions")
			}
			if !fileInfo.Mode().IsRegular() {
				t.Fatalf("Expecting a file: %s", keyPemPath)
			}
		}

		certPemBytes, err := ioutil.ReadFile(certPemPath)
		if err != nil {
			t.Fatal(err)
		}
		block, _ := pem.Decode(certPemBytes)
		if block == nil {
			t.Fatal("failed to parse PEM block containing the public key")
		}

		pub, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal("failed to parse DER encoded public key: " + err.Error())
		}
		if pub.Issuer.String() != "O=OpenBazaar" {
			t.Fatalf("%s != OpenBazaar", pub.Issuer.String())
		}
		// t.Log(pub.IssuingCertificateURL)
		// t.Log(pub.URIs)
	}

}
