package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"math/big"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

//GenerateCertificates struct
type GenerateCertificates struct {
	DataDir  string        `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet  bool          `short:"t" long:"testnet" description:"config file is for testnet node"`
	Host     string        `short:"h" long:"host" description:"comma-separated hostnames and IPs to generate a certificate for"`
	ValidFor time.Duration `long:"duration" description:"duration that certificate is valid for"`
}

//Execute gencerts command
func (x *GenerateCertificates) Execute(args []string) error {

	flag.Parse()

	// Set repo path
	repoPath, err := repo.GetRepoPath(x.Testnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}

	//Check if host entered
	if len(x.Host) == 0 {
		log.Fatalf("Missing required --host parameter")
	}

	// Set default duration
	if x.ValidFor == 0 {
		x.ValidFor = 365 * 24 * time.Hour
	}

	var priv interface{}

	//Generate key
	priv, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("failed to generate private key: %s", err)
	}

	//Set creation date
	var notBefore = time.Now()
	notAfter := notBefore.Add(x.ValidFor)

	//Crate serial nmuber
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"OpenBazaar"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: true,
	}

	//Check if host ip or dns name and count their quantity
	hosts := strings.Split(x.Host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	//Create sertificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.(*rsa.PrivateKey).PublicKey, priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	// Create ssl directory
	err = os.MkdirAll(path.Join(repoPath, "ssl"), os.ModePerm)
	if err != nil {
		log.Fatalf("Failed to create ssl directory: %s", err)
	}

	//Create and write cert.pem
	certOut, err := os.Create(path.Join(repoPath, "ssl", "cert.pem"))
	if err != nil {
		log.Fatalf("failed to open cert.pem for writing: %s", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()
	log.Noticef("wrote cert.pem\n")

	//Create and write key.pem
	keyOut, err := os.OpenFile(path.Join(repoPath, "ssl", "key.pem"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Noticef("failed to open key.pem for writing:", err)
		return err
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv.(*rsa.PrivateKey))})
	keyOut.Close()
	log.Noticef("wrote key.pem\n")

	return nil
}
