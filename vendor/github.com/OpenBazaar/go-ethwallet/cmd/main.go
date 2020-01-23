package main

import (
	"context"
	"flag"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/joho/godotenv/autoload"
	log "github.com/sirupsen/logrus"

	"github.com/OpenBazaar/go-ethwallet/wallet"
)

var (
	password string
	keyDir   string
	keyFile  string
)

const ethToWei = 1 << 17

// InfuraRopstenBase is the base URL for Infura Ropsten network
const InfuraRopstenBase string = "https://ropsten.infura.io/"

func init() {
	flag.StringVar(&password, "p", "", "password for keystore")
	flag.StringVar(&keyDir, "d", "", "key dir to generate key")
	flag.StringVar(&keyFile, "f", "", "key file path")
}

func main() {
	fmt.Println(os.Getenv("INFURA_KEY"))

	ropstenURL := InfuraRopstenBase + os.Getenv("INFURA_KEY")

	flag.Parse()

	fmt.Println("Password is : ", password)
	fmt.Println("keydir is: ", keyDir)
	fmt.Println("keyfile is : ", keyFile)

	client, err := ethclient.Dial("https://mainnet.infura.io")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("we have a connection")
	_ = client

	address := common.HexToAddress("0x71c7656ec7ab88b098defb751b7401b5f6d8976f")
	fmt.Println(address.Hex())
	// 0x71C7656EC7ab88b098defB751B7401B5f6d8976F
	fmt.Println(address.Hash().Hex())
	// 0x00000000000000000000000071c7656ec7ab88b098defb751b7401b5f6d8976f
	fmt.Println(address.Bytes())

	account := common.HexToAddress("0x71c7656ec7ab88b098defb751b7401b5f6d8976f")
	balance, err := client.BalanceAt(context.Background(), account, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(balance)

	// Get the balance at a particular instance of time expressed as block number
	blockNumber := big.NewInt(5532993)
	balance, err = client.BalanceAt(context.Background(), account, blockNumber)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(balance)

	//wallet.GenWallet()

	//wallet.GenDefaultKeyStore(password)
	var myAccount *wallet.Account
	myAccount, err = wallet.NewAccountFromKeyfile(keyFile, password)
	if err != nil {
		log.Fatal("key file validation failed:%s", err.Error())
	}
	fmt.Println(myAccount.Address().String())

	// create the source wallet obj for Infura Ropsten
	myWallet := wallet.NewEthereumWalletWithKeyfile(ropstenURL, keyFile, password)
	fmt.Println(myWallet.GetBalance())

	// create dest account
	//wallet.GenDefaultKeyStore(password)
	var destAccount *wallet.Account
	destKeyFile := "./UTC--2018-06-16T20-09-33.726552102Z--cecb952de5b23950b15bfd49302d1bdd25f9ee67"
	destAccount, err = wallet.NewAccountFromKeyfile(destKeyFile, password)
	if err != nil {
		log.Fatal("key file validation failed:%s", err.Error())
	}
	fmt.Println(destAccount.Address().String())

	// create the destination wallet obj for Infura Ropsten
	destWallet := wallet.NewEthereumWalletWithKeyfile(ropstenURL, destKeyFile, password)
	fmt.Println(destWallet.GetBalance())

	// lets transfer
	//err = myWallet.Transfer(destAccount.Address().String(), big.NewInt(3344556677))
	//if err != nil {
	//	fmt.Println("what happened here : ", err)
	//}
	fmt.Println("after transfer : ")
	fmt.Println("Dest balance ")
	fmt.Println(destWallet.GetBalance())
	fmt.Println(destWallet.Balance())
	fmt.Println("Source balance ")
	fmt.Println(myWallet.GetBalance())
	fmt.Println(myWallet.Balance())

	fmt.Println(myWallet.CreateAddress())
}
