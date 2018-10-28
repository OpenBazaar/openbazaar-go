package cmd_test

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/cmd"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildCertDirectory() (*string, func(), error) {
	ctx := schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	}
	certSchema := schema.MustNewCustomSchemaManager(ctx)
	if err := certSchema.BuildSchemaDirectories(); err != nil {
		return nil, nil, err
	}
	return &ctx.DataPath, certSchema.DestroySchemaDirectories, nil
}

func TestGenCertsGenericDefaults(t *testing.T) {
	var dataPath, Destroy, schema_err = buildCertDirectory()
	dataDir := string([]byte(*dataPath))
	if schema_err != nil {
		Destroy()
		t.Fatal(schema_err)
	}

	config := cmd.GenerateCertificates{
		DataDir:  dataDir,
		Testnet:  true,
		Host:     "127.0.0.1",
		ValidFor: 1, // 1ns ... 1*1e9 == 1s
	}

	args := []string{"args?"}

	if err := config.Execute(args); err != nil {
		Destroy()
		t.Fatal(err)
	}

	sslPath := path.Join(dataDir, "ssl")
	if fileInfo, err := os.Stat(sslPath); err != nil {
		Destroy()
		t.Fatal(err)
	} else {
		if fileInfo.Mode().Perm() != 0755 {
			Destroy()
			t.Fatal("ssl directory does not have 0755 permissions")
		}
		if !fileInfo.IsDir() {
			Destroy()
			t.Fatalf("Expecting a directory: %s", dataDir)
		}
	}

	certPemPath := path.Join(dataDir, "ssl", "cert.pem")
	if fileInfo, err := os.Stat(certPemPath); err != nil {
		Destroy()
		t.Fatal(err)
	} else {
		if fileInfo.Mode().Perm() != 0644 {
			Destroy()
			t.Fatal("cert.pem does not have 0644 permissions")
		}
		if !fileInfo.Mode().IsRegular() {
			Destroy()
			t.Fatalf("Expecting a file: %s", certPemPath)
		}
	}

	keyPemPath := path.Join(dataDir, "ssl", "key.pem")
	if fileInfo, err := os.Stat(keyPemPath); err != nil {
		Destroy()
		t.Fatal(err)
	} else {
		if fileInfo.Mode().Perm() != 0600 {
			Destroy()
			t.Fatal("cert.pem does not have 0600 permissions")
		}
		if !fileInfo.Mode().IsRegular() {
			Destroy()
			t.Fatalf("Expecting a file: %s", keyPemPath)
		}
	}

	certPemBytes, err := ioutil.ReadFile(certPemPath)
	if err != nil {
		Destroy()
		t.Fatal(err)
	}
	block, _ := pem.Decode(certPemBytes)
	if block == nil {
		Destroy()
		t.Fatal("failed to parse PEM block containing the public key")
	}

	pub, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		Destroy()
		t.Fatal("failed to parse DER encoded public key: " + err.Error())
	}
	if pub.Issuer.String() != "O=OpenBazaar" {
		t.Errorf("%s != OpenBazaar", pub.Issuer.String())
	}
	// t.Log(pub.IssuingCertificateURL)
	if err := pub.VerifyHostname("127.0.0.1"); err != nil {
		t.Errorf("%s", err)
	}

	if pub.NotAfter.Sub(pub.NotBefore).Seconds() != 0 {
		t.Errorf("Certificate NotAfter != NotBefore: %s %s", pub.NotAfter.String(), pub.NotBefore.String())
	}
	Destroy()
}
