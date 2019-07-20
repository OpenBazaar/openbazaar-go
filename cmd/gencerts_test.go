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

func buildCertDirectory() (string, func(), error) {
	ctx := schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	}
	certSchema := schema.MustNewCustomSchemaManager(ctx)
	if err := certSchema.BuildSchemaDirectories(); err != nil {
		return "", nil, err
	}
	return ctx.DataPath, certSchema.DestroySchemaDirectories, nil
}

func TestGenCertsGenericDefaults(t *testing.T) {
	dataDir, destroy, schemaErr := buildCertDirectory()
	if schemaErr != nil {
		t.Fatal(schemaErr)
	}
	defer destroy()

	config := cmd.GenerateCertificates{
		DataDir:  dataDir,
		Testnet:  true,
		Host:     "127.0.0.1",
		ValidFor: 1, // 1ns ... 1*1e9 == 1s
	}

	args := []string{""}

	if err := config.Execute(args); err != nil {
		t.Fatalf("unable to GenerateCertificates: %s", err)
	}

	sslPath := path.Join(dataDir, "ssl")
	fileInfoSsl, errSsl := os.Stat(sslPath)
	if errSsl != nil {
		t.Fatalf("unable to find sslPath: %s", errSsl)
	}
	if fileInfoSsl.Mode().Perm() != 0755 {
		t.Fatal("ssl directory does not have 0755 permissions")
	}
	if !fileInfoSsl.IsDir() {
		t.Fatalf("Expecting a directory: %s", dataDir)
	}

	certPemPath := path.Join(dataDir, "ssl", "cert.pem")
	fileInfoCert, errCert := os.Stat(certPemPath)
	if errCert != nil {
		t.Fatalf("unable to find certPemPath %s: %s", certPemPath, errCert)
	}
	if fileInfoCert.Mode().Perm() != 0644 {
		t.Fatal("cert.pem does not have 0644 permissions")
	}
	if !fileInfoCert.Mode().IsRegular() {
		t.Fatalf("Expecting a file: %s", certPemPath)
	}

	keyPemPath := path.Join(dataDir, "ssl", "key.pem")
	fileInfoKey, errKey := os.Stat(keyPemPath)
	if errKey != nil {
		t.Fatalf("unable to find keyPemPath: %s", errKey)
	}
	if fileInfoKey.Mode().Perm() != 0600 {
		t.Fatal("cert.pem does not have 0600 permissions")
	}
	if !fileInfoKey.Mode().IsRegular() {
		t.Fatalf("Expecting a file: %s", keyPemPath)
	}

	certPemBytes, err := ioutil.ReadFile(certPemPath)
	if err != nil {
		t.Fatalf("unable to read certPemPath: %s", err)
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
		t.Errorf("unexpected issuer found on certificate: %s != OpenBazaar", pub.Issuer.String())
	}
	if err := pub.VerifyHostname("127.0.0.1"); err != nil {
		t.Errorf("unable to VerifyHostname 127.0.0.1: %s", err)
	}

	if pub.NotAfter.Sub(pub.NotBefore).Seconds() != 0 {
		t.Errorf("Certificate NotAfter != NotBefore: %s %s", pub.NotAfter.String(), pub.NotBefore.String())
	}
}
