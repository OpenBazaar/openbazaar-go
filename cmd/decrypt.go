package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"golang.org/x/crypto/ssh/terminal"
)

type DecryptDatabase struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
}

func (x *DecryptDatabase) Execute(args []string) error {
	reader := bufio.NewReader(os.Stdin)

	var repoPath string
	var dbPath string
	var filename string
	var testnet bool
	var err error
	if x.DataDir == "" {
		repoPath, err = repo.GetRepoPath(false)
		if err != nil {
			fmt.Println(err)
			return nil
		}
	} else {
		repoPath = x.DataDir
	}
	for {
		fmt.Print("Decrypt the mainnet or testnet db?: ")
		resp, _ := reader.ReadString('\n')
		if strings.Contains(strings.ToLower(resp), "mainnet") {
			filename = "mainnet.db"
			dbPath = path.Join(repoPath, "datastore", filename)
			repoLockFile := filepath.Join(repoPath, fsrepo.LockFile)
			if _, err := os.Stat(repoLockFile); !os.IsNotExist(err) {
				fmt.Println("Cannot decrypt while the daemon is running.")
				return nil
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Println("Database does not exist. You may need to run the daemon at least once to initialize it.")
				return nil
			}
			break
		} else if strings.Contains(strings.ToLower(resp), "testnet") {
			testnet = true
			filename = "testnet.db"
			dbPath = path.Join(repoPath, "datastore", filename)
			repoLockFile := filepath.Join(repoPath, fsrepo.LockFile)
			if _, err := os.Stat(repoLockFile); !os.IsNotExist(err) {
				fmt.Println("Cannot decrypt while the daemon is running.")
				return nil
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Println("Database does not exist. You may need to run the node at least once to initialize it.")
				return nil
			}
			break
		} else {
			fmt.Println("No comprende")
		}
	}
	fmt.Print("Enter your password: ")
	bytePassword, _ := terminal.ReadPassword(syscall.Stdin)
	fmt.Println("")
	pw := string(bytePassword)
	pw = strings.Replace(pw, "'", "''", -1)
	sqlliteDB, err := db.Create(repoPath, pw, testnet, wallet.Bitcoin)
	if err != nil || sqlliteDB.Config().IsEncrypted() {
		fmt.Println("Invalid password")
		return err
	}
	if err := os.MkdirAll(path.Join(repoPath, "tmp", "datastore"), os.ModePerm); err != nil {
		return err
	}
	tmpDB, err := db.Create(path.Join(repoPath, "tmp"), "", testnet, wallet.Bitcoin)
	if err != nil {
		fmt.Println(err)
		return err
	}
	tmpDB.InitTables("")
	if err := sqlliteDB.Copy(path.Join(repoPath, "tmp", "datastore", filename), ""); err != nil {
		fmt.Println(err)
		return err
	}
	err = os.Rename(path.Join(repoPath, "tmp", "datastore", filename), path.Join(repoPath, "datastore", filename))
	if err != nil {
		fmt.Println(err)
		return err
	}
	os.RemoveAll(path.Join(repoPath, "tmp"))
	fmt.Println("Success!")
	return nil
}
