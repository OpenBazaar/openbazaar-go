package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"golang.org/x/crypto/ssh/terminal"
)

type SetAPICreds struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"config file is for testnet node"`
}

func (x *SetAPICreds) Execute(args []string) error {
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
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	apiCfg, err := schema.GetAPIConfig(configFile)
	if err != nil {
		log.Error(err)
		return err
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter username: ")
	username, _ := reader.ReadString('\n')

	var pw string
	for {
		fmt.Print("Enter a veerrrry strong password: ")
		bytePassword, _ := terminal.ReadPassword(syscall.Stdin)
		fmt.Println("")
		resp := string(bytePassword)
		if len(resp) < 8 {
			fmt.Println("You call that a password? Try again.")
		} else if resp != "" {
			pw = resp
			break
		} else {
			fmt.Println("Seriously, enter a password.")
		}
	}
	for {
		fmt.Print("Confirm your password: ")
		bytePassword, _ := terminal.ReadPassword(syscall.Stdin)
		fmt.Println("")
		resp := string(bytePassword)
		if resp == pw {
			break
		} else {
			fmt.Println("Quit effin around. Try again.")
		}
	}
	pw = strings.Replace(pw, "'", "''", -1)
	if strings.Contains(username, "\r\n") {
		apiCfg.Username = strings.Replace(username, "\r\n", "", -1)
	} else if strings.Contains(username, "\n") {
		apiCfg.Username = strings.Replace(username, "\n", "", -1)
	}
	apiCfg.Authenticated = true
	h := sha256.Sum256([]byte(pw))
	apiCfg.Password = hex.EncodeToString(h[:])
	if len(apiCfg.AllowedIPs) == 0 {
		apiCfg.AllowedIPs = []string{}
	}

	err = r.SetConfigKey("JSON-API", apiCfg)
	if err != nil {
		return err
	}

	return nil
}
