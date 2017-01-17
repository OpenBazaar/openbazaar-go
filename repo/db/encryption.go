package db

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	lockfile "github.com/ipfs/go-ipfs/repo/fsrepo/lock"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh/terminal"
	"runtime"
)

// FIXME: the encrypt and decrypt functions here should probably be added to the DB interface
// FIXME: and the stdin stuff should be moved somewhere outside of this package.

// Create a temp encrypted database, read the unencrypted database into it then replace the unencrypted database
func Encrypt() error {
	reader := bufio.NewReader(os.Stdin)
	var repoPath string
	var dbPath string
	var filename string
	var testnet bool
	var err error
	for {
		fmt.Print("Encrypt the mainnet or testnet db?: ")
		resp, _ := reader.ReadString('\n')
		if strings.Contains(strings.ToLower(resp), "mainnet") {
			repoPath, err = getRepoPath(false)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			filename = "mainnet.db"
			dbPath = path.Join(repoPath, "datastore", filename)
			repoLockFile := filepath.Join(repoPath, lockfile.LockFile)
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
			repoPath, err = getRepoPath(true)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			testnet = true
			filename = "testnet.db"
			dbPath = path.Join(repoPath, "datastore", filename)
			repoLockFile := filepath.Join(repoPath, lockfile.LockFile)
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
	tmpPath := path.Join(repoPath, "tmp")
	sqlliteDB, err := Create(repoPath, "", testnet)
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
	tmpDB, err := Create(tmpPath, pw, testnet)
	if err != nil {
		fmt.Println(err)
		return err
	}

	initDatabaseTables(tmpDB.db, pw)
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
	fmt.Println("Success! You must now run openbazaard start with the --password flag.")
	return nil
}

// Create a temp database, read the encrypted database into it then replace the encrypted database
func Decrypt() error {
	reader := bufio.NewReader(os.Stdin)

	var repoPath string
	var dbPath string
	var filename string
	var testnet bool
	var err error
	for {
		fmt.Print("Decrypt the mainnet or testnet db?: ")
		resp, _ := reader.ReadString('\n')
		if strings.Contains(strings.ToLower(resp), "mainnet") {
			repoPath, err = getRepoPath(false)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			filename = "mainnet.db"
			dbPath = path.Join(repoPath, "datastore", filename)
			repoLockFile := filepath.Join(repoPath, lockfile.LockFile)
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
			repoPath, err = getRepoPath(true)
			if err != nil {
				fmt.Println(err)
				return nil
			}
			testnet = true
			filename = "testnet.db"
			dbPath = path.Join(repoPath, "datastore", filename)
			repoLockFile := filepath.Join(repoPath, lockfile.LockFile)
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
	bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	pw := string(bytePassword)
	pw = strings.Replace(pw, "'", "''", -1)
	sqlliteDB, err := Create(repoPath, pw, testnet)
	if err != nil || sqlliteDB.Config().IsEncrypted() {
		fmt.Println("Invalid password")
		return err
	}
	if err := os.MkdirAll(path.Join(repoPath, "tmp", "datastore"), os.ModePerm); err != nil {
		return err
	}
	tmpDB, err := Create(path.Join(repoPath, "tmp"), "", testnet)
	if err != nil {
		fmt.Println(err)
		return err
	}
	initDatabaseTables(tmpDB.db, "")
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

func getRepoPath(isTestnet bool) (string, error) {
	// Set default base path and directory name
	path := "~"
	directoryName := "OpenBazaar2.0"

	// Override OS-specific names
	switch runtime.GOOS {
	case "linux":
		directoryName = ".openbazaar2.0"
	case "darwin":
		path = "~/Library/Application Support"
	}

	// Append testnet flag if on testnet
	if isTestnet {
		directoryName += "-testnet"
	}

	// Join the path and directory name, then expand the home path
	fullPath, err := homedir.Expand(filepath.Join(path, directoryName))
	if err != nil {
		return "", err
	}

	// Return the shortest lexical representation of the path
	return filepath.Clean(fullPath), nil
}
