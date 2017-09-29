package net

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type config struct {
	Directory    string   `short:"d" long:"directory" description:"Directory to write certificate pair"`
	Years        int      `short:"y" long:"years" description:"How many years a certificate is valid for"`
	Organization string   `short:"o" long:"org" description:"Organization in certificate"`
	ExtraHosts   []string `short:"H" long:"host" description:"Additional hosts/IPs to create certificate for"`
	Force        bool     `short:"f" long:"force" description:"Force overwriting of any old certs and keys"`
}

func GenerateCerts(repoPath string, force bool) error {
	cfg := config{
		Years:        50,
		Organization: "OpenBazaar",
		Directory:    repoPath,
		Force:        force,
	}

	cfg.Directory = cleanAndExpandPath(cfg.Directory)
	certFile := filepath.Join(cfg.Directory, "ob.cert")
	keyFile := filepath.Join(cfg.Directory, "ob.key")

	if !cfg.Force {
		if fileExists(certFile) || fileExists(keyFile) {
			return fmt.Errorf("%v: certificate and/or key files exist; use -f to force", cfg.Directory)
		}
	}

	validUntil := time.Now().Add(time.Duration(cfg.Years) * 365 * 24 * time.Hour)
	cert, key, err := NewTLSCertPair(cfg.Organization, validUntil, cfg.ExtraHosts)
	if err != nil {
		return fmt.Errorf("Cannot generate certificate pair: %v\n", err)
	}

	// Write cert and key files.
	if err = ioutil.WriteFile(certFile, cert, 0666); err != nil {
		return fmt.Errorf("Cannot write cert: %v\n", err)
	}
	if err = ioutil.WriteFile(keyFile, key, 0600); err != nil {
		os.Remove(certFile)
		return fmt.Errorf("Cannot write key: %v\n", err)
	}
	return nil
}

// cleanAndExpandPath expands environement variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(path)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// NewTLSCertPair returns a new PEM-encoded x.509 certificate pair
// based on a 521-bit ECDSA private key.  The machine's local interface
// addresses and all variants of IPv4 and IPv6 localhost are included as
// valid IP addresses.
func NewTLSCertPair(organization string, validUntil time.Time, extraHosts []string) (cert, key []byte, err error) {
	now := time.Now()
	if validUntil.Before(now) {
		return nil, nil, errors.New("validUntil would create an already-expired certificate")
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// end of ASN.1 time
	endOfTime := time.Date(2049, 12, 31, 23, 59, 59, 0, time.UTC)
	if validUntil.After(endOfTime) {
		validUntil = endOfTime
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %s", err)
	}

	host, err := os.Hostname()
	if err != nil {
		return nil, nil, err
	}

	ipAddresses := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	dnsNames := []string{host}
	if host != "localhost" {
		dnsNames = append(dnsNames, "localhost")
	}

	addIP := func(ipAddr net.IP) {
		for _, ip := range ipAddresses {
			if bytes.Equal(ip, ipAddr) {
				return
			}
		}
		ipAddresses = append(ipAddresses, ipAddr)
	}
	addHost := func(host string) {
		for _, dnsName := range dnsNames {
			if host == dnsName {
				return
			}
		}
		dnsNames = append(dnsNames, host)
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, nil, err
	}
	for _, a := range addrs {
		ipAddr, _, err := net.ParseCIDR(a.String())
		if err == nil {
			addIP(ipAddr)
		}
	}

	for _, hostStr := range extraHosts {
		host, _, err := net.SplitHostPort(hostStr)
		if err != nil {
			host = hostStr
		}
		if ip := net.ParseIP(host); ip != nil {
			addIP(ip)
		} else {
			addHost(host)
		}
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{organization},
			CommonName:   host,
		},
		NotBefore: now.Add(-time.Hour * 24),
		NotAfter:  validUntil,

		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature |
			x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:        true, // so can sign self.
		BasicConstraintsValid: true,

		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template,
		&template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	certBuf := &bytes.Buffer{}
	err = pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode certificate: %v", err)
	}

	keybytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %v", err)
	}

	keyBuf := &bytes.Buffer{}
	err = pem.Encode(keyBuf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keybytes})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode private key: %v", err)
	}

	return certBuf.Bytes(), keyBuf.Bytes(), nil
}
