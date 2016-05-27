package db

import (
	"bufio"
	"os"
	"fmt"
	"strings"
	"path/filepath"
	"path"
	"github.com/mitchellh/go-homedir"
)

//FIXME: the encrypt and decrypt functions here should probably be added to the DB interface
//FIXME: and the stdin stuff should be moved somewhere outside of this package.

// Create a temp encrypted database, read the unencrypted db into it then replace the unencrypted db
func Encrypt() error {
	//TODO: make sure we're not running when they do this
	reader := bufio.NewReader(os.Stdin)
	doesNotExist := func() {
		fmt.Println("Database does not exist. You may need to run the node at least once to initialize it.")
	}
	var repoPath string
	var filename string
	var testnet bool
	for {
		fmt.Print("Encrypt the mainnet or testnet db?: ")
		resp, _ := reader.ReadString('\n')
		if strings.ToLower(resp) == "mainnet\n" {
			rPath := "~/.openbazaar2"
			filename = "mainnet.db"
			testnet = false
			expPath, _ := homedir.Expand(filepath.Clean(rPath))
			repoPath = expPath
			if _, err := os.Stat(expPath); os.IsNotExist(err) {
				doesNotExist()
				return nil
			}
			break
		} else if strings.ToLower(resp) == "testnet\n" {
			rPath := "~/.openbazaar2-testnet"
			filename = "testnet.db"
			testnet = true
			expPath, _ := homedir.Expand(filepath.Clean(rPath))
			repoPath = expPath
			if _, err := os.Stat(expPath); os.IsNotExist(err) {
				doesNotExist()
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
		resp, _ := reader.ReadString('\n')
		if resp != "\n" {
			pw = resp[:len(resp) - 1]
			break
		} else {
			fmt.Println("Seriously, enter a password.")
		}
	}
	for {
		fmt.Print("Confirm your password: ")
		resp, _ := reader.ReadString('\n')
		if resp[:len(resp) - 1] == pw {
			break
		} else {
			fmt.Println("Your passwords didn't match. Try again")
		}
	}
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
	mnemonic, _ := sqlliteDB.Config().GetMnemonic()
	identityKey, _ := sqlliteDB.Config().GetIdentityKey()
	tmpDB.Config().Init(mnemonic, identityKey, pw)
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

// Create a temp database, read the encrypted db into it then replace the encrypted db
func Decrypt() error {
	//TODO: make sure we're not running when they do this
	reader := bufio.NewReader(os.Stdin)
	doesNotExist := func() {
		fmt.Println("Database does not exist. You may need to run the node at least once to initialize it.")
	}
	var repoPath string
	var filename string
	var testnet bool
	for {
		fmt.Print("Decrypt the mainnet or testnet db?: ")
		resp, _ := reader.ReadString('\n')
		if strings.ToLower(resp) == "mainnet\n" {
			rPath := "~/.openbazaar2"
			filename = "mainnet.db"
			testnet = false
			expPath, _ := homedir.Expand(filepath.Clean(rPath))
			repoPath = expPath
			if _, err := os.Stat(expPath); os.IsNotExist(err) {
				doesNotExist()
				return nil
			}
			break
		} else if strings.ToLower(resp) == "testnet\n" {
			rPath := "~/.openbazaar2-testnet"
			filename = "testnet.db"
			testnet = true
			expPath, _ := homedir.Expand(filepath.Clean(rPath))
			repoPath = expPath
			if _, err := os.Stat(expPath); os.IsNotExist(err) {
				doesNotExist()
				return nil
			}
			break
		} else {
			fmt.Println("No comprende")
		}
	}
	fmt.Print("Enter your password: ")
	pw, _ := reader.ReadString('\n')
	pw = pw[:len(pw)-1]
	sqlliteDB, err := Create(repoPath, pw, testnet)
	if err != nil || sqlliteDB.Config().IsEncrypted(){
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
	mnemonic, _ := sqlliteDB.Config().GetMnemonic()
	identityKey, _ := sqlliteDB.Config().GetIdentityKey()
	tmpDB.Config().Init(mnemonic, identityKey, "")
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