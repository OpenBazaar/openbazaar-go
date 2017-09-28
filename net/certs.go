package net

import (
	"fmt"
	"github.com/btcsuite/btcutil"
	"io/ioutil"
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
	cert, key, err := btcutil.NewTLSCertPair(cfg.Organization, validUntil, cfg.ExtraHosts)
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
