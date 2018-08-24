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

type EncryptDatabase struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
}

func (x *EncryptDatabase) Execute(args []string) error {
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
		fmt.Print("Encrypt the mainnet or testnet db?: ")
		resp, _ := reader.ReadString('\n')
		if strings.Contains(strings.ToLower(resp), "mainnet") {
			filename = "mainnet.db"
			dbPath = path.Join(repoPath, "datastore", filename)
			repoLockFile := filepath.Join(repoPath, fsrepo.LockFile)
			if _, err := os.Stat(repoLockFile); !os.IsNotExist(err) {
				fmt.Println("Cannot encrypt while the daemon is running.")
				return nil
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Println("Database does not exist. You may need to run the node at least once to initialize it.")
				return nil
			}
			break
		} else if strings.Contains(strings.ToLower(resp), "testnet") {
			testnet = true
			filename = "testnet.db"
			dbPath = path.Join(repoPath, "datastore", filename)
			repoLockFile := filepath.Join(repoPath, fsrepo.LockFile)
			if _, err := os.Stat(repoLockFile); !os.IsNotExist(err) {
				fmt.Println("Cannot encrypt while the daemon is running.")
				return nil
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Println("Database does not exist. You may need to run the daemon at least once to initialize it.")
				return nil
			}
			break
		} else {
			fmt.Println("No comprende")
		}
	}
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
	tmpPath := path.Join(repoPath, "tmp")
	sqlliteDB, err := db.Create(repoPath, "", testnet, wallet.Bitcoin)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if sqlliteDB.Config().IsEncrypted() {
		fmt.Println("The database is alredy encrypted")
		return nil
	}
	if err := os.MkdirAll(path.Join(repoPath, "tmp", "datastore"), os.ModePerm); err != nil {
		return err
	}
	tmpDB, err := db.Create(tmpPath, pw, testnet, wallet.Bitcoin)
	if err != nil {
		fmt.Println(err)
		return err
	}

	tmpDB.InitTables(pw)
	if err := sqlliteDB.Copy(path.Join(tmpPath, "datastore", filename), pw); err != nil {
		fmt.Println(err)
		return err
	}
	err = os.Rename(path.Join(tmpPath, "datastore", filename), path.Join(repoPath, "datastore", filename))
	if err != nil {
		fmt.Println(err)
		return err
	}
	os.RemoveAll(path.Join(tmpPath))
	fmt.Println("Success! You must now run openbazaard start with a password.")
	return nil
}
