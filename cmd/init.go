package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/op/go-logging"
	"os"
	"strings"
	"time"
)

var log = logging.MustGetLogger("cmd")

type Init struct {
	Password           string `short:"p" long:"password" description:"the encryption password if the database is to be encrypted"`
	DataDir            string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Mnemonic           string `short:"m" long:"mnemonic" description:"specify a mnemonic seed to use to derive the keychain"`
	Testnet            bool   `short:"t" long:"testnet" description:"use the test network"`
	Force              bool   `short:"f" long:"force" description:"force overwrite existing repo (dangerous!)"`
	WalletCreationDate string `short:"w" long:"walletcreationdate" description:"specify the date the seed was created. if omitted the wallet will sync from the oldest checkpoint."`
}

func (x *Init) Execute(args []string) error {
	// Set repo path
	repoPath, err := repo.GetRepoPath(x.Testnet)
	if err != nil {
		return err
	}
	if x.DataDir != "" {
		repoPath = x.DataDir
	}
	if x.Password != "" {
		x.Password = strings.Replace(x.Password, "'", "''", -1)
	}
	creationDate := time.Now()
	if x.WalletCreationDate != "" {
		creationDate, err = time.Parse(time.RFC3339, x.WalletCreationDate)
		if err != nil {
			return errors.New("Wallet creation date timestamp must be in RFC3339 format")
		}
	}

	_, err = InitializeRepo(repoPath, x.Password, x.Mnemonic, x.Testnet, creationDate)
	if err == repo.ErrRepoExists && x.Force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Force overwriting the db will destroy your existing keys and history. Are you really, really sure you want to continue? (y/n): ")
		resp, _ := reader.ReadString('\n')
		if strings.ToLower(resp) == "y\n" || strings.ToLower(resp) == "yes\n" || strings.ToLower(resp)[:1] == "y" {
			os.RemoveAll(repoPath)
			_, err = InitializeRepo(repoPath, x.Password, x.Mnemonic, x.Testnet, creationDate)
			if err != nil {
				return err
			}
			fmt.Printf("OpenBazaar repo initialized at %s\n", repoPath)
			return nil
		} else {
			return nil
		}
	} else if err != nil {
		return err
	}
	fmt.Printf("OpenBazaar repo initialized at %s\n", repoPath)
	return nil
}
