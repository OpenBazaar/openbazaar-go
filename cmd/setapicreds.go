package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	"os"
	"strings"
	"syscall"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"golang.org/x/crypto/ssh/terminal"
)

type SetAPICreds struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Testnet bool   `short:"t" long:"testnet" description:"config file is for testnet node"`
}

func (x *SetAPICreds) Execute(args []string) error {
	// Set repo path
	repoPath, err := repo.GetRepoPath(x.Testnet, x.DataDir)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}
	cfgPath := path.Join(repoPath, "config")
	configFile, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	_, err = fsrepo.Open(repoPath)
	if _, ok := err.(fsrepo.NoRepoError); ok {
		return fmt.Errorf(
			"IPFS repo in the data directory '%s' has not been initialized."+
				"\nRun openbazaar with the 'start' command to initialize.",
			repoPath)
	}
	if err != nil {
		return err
	}

	configJson := make(map[string]interface{})
	err = json.Unmarshal(configFile, &configJson)
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
		// nolint:unconvert
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
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
		// nolint:unconvert
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
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

	configJson["JSON-API"] = apiCfg

	out, err := json.MarshalIndent(configJson, "", "    ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(cfgPath, out, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
