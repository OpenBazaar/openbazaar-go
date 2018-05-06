package cmd

import (
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"os/exec"
	"runtime"
)

type GenerateCertificates struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"config file is for testnet node"`
}

func (x *GenerateCertificates) Execute(args []string) error {

	// Set repo path
	repoPath, err := repo.GetRepoPath(x.Testnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		log.Error(err)
		return err
	}

	var command []string
	switch runtime.GOOS {
	case "darwin":
		command = []string{"open"}
	case "windows":
		command = []string{"cmd"}
	default:
		command = []string{`sh`, `-c`}
	}
	cmd := exec.Command(command[0], append(command[1:], `openssl req -newkey rsa:2048 -sha256 -nodes -keyout key.pem -x509 -days 365 -out cert.pem -subj "/C=. /ST=. /L=. /O=. Company/CN=."`)...)
	cmd.Run()

	log.Notice("Done")

	return nil

}
