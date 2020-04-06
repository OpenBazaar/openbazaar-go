package wallet

import (
	"crypto/ecdsa"
	"io/ioutil"
	"os"

	"github.com/btcsuite/btcd/chaincfg"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/tyler-smith/go-bip39"
)

// EthAddress implements the WalletAddress interface
type EthAddress struct {
	address *common.Address
}

// String representation of eth address
func (addr EthAddress) String() string {
	return addr.address.Hex() // [2:] //String()[2:]
}

// EncodeAddress returns hex representation of the address
func (addr EthAddress) EncodeAddress() string {
	return addr.address.Hex() // [2:]
}

// ScriptAddress returns byte representation of address
func (addr EthAddress) ScriptAddress() []byte {
	return addr.address.Bytes()
}

// IsForNet returns true because EthAddress has to become btc.Address
func (addr EthAddress) IsForNet(params *chaincfg.Params) bool {
	return true
}

// Account represents ethereum keystore
type Account struct {
	// key *keystore.Key
	privateKey *ecdsa.PrivateKey
	address    common.Address
}

// NewAccountFromKeyfile returns the account imported
func NewAccountFromKeyfile(keyFile, password string) (*Account, error) {
	key, err := importKey(keyFile, password)
	if err != nil {
		return nil, err
	}

	return &Account{
		privateKey: key.PrivateKey,
		address:    crypto.PubkeyToAddress(key.PrivateKey.PublicKey),
	}, nil
}

// NewAccountFromMnemonic returns generated account
func NewAccountFromMnemonic(mnemonic, password string, params *chaincfg.Params) (*Account, error) {
	seed := bip39.NewSeed(mnemonic, password)

	/*
		fmt.Println(len(seed))
		fmt.Println(seed)

		priv := new(ecdsa.PrivateKey)
		priv.PublicKey.Curve = btcec.S256()

		if 8*len(seed[:32]) != priv.Params().BitSize {
			fmt.Println("whoa....", 8*len(seed[:32]), priv.Params().BitSize)
			//return nil, fmt.Errorf("invalid length, need %d bits", priv.Params().BitSize)
		}

	*/

	// This is no longer used. 31 Dec 2018
	/*
		privateKeyECDSA, err := crypto.ToECDSA(seed[:32])
		if err != nil {
			return nil, err
		}
	*/

	mPrivKey, err := hd.NewMaster(seed, params)
	if err != nil {
		log.Errorf("err initializing btc priv key : %v", err)
		return nil, err
	}

	exPrivKey, err := mPrivKey.ECPrivKey()
	if err != nil {
		log.Errorf("err extracting btcec priv key : %v", err)
		return nil, err
	}

	privateKeyECDSA := exPrivKey.ToECDSA()

	/*
		fmt.Println(privateKeyECDSA)
		fmt.Println(privateKeyECDSA.Public())

		privateKeyBytes := crypto.FromECDSA(privateKeyECDSA)
		fmt.Println(hexutil.Encode(privateKeyBytes)[2:])

		publicKey := privateKeyECDSA.Public()
		publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
		if !ok {
			log.Fatal("error casting public key to ECDSA")
		}

		publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
		fmt.Println(hexutil.Encode(publicKeyBytes)[4:])

		address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
		fmt.Println("address : ", address)
	*/

	return &Account{privateKey: privateKeyECDSA, address: crypto.PubkeyToAddress(privateKeyECDSA.PublicKey)}, nil
}

func importKey(keyFile, password string) (*keystore.Key, error) {
	f, err := os.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	json, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return keystore.DecryptKey(json, password)
}

// Address returns the eth address
func (account *Account) Address() common.Address {
	return account.address
}

// SignTransaction will sign the txn
func (account *Account) SignTransaction(signer types.Signer, tx *types.Transaction) (*types.Transaction, error) {
	signature, err := crypto.Sign(signer.Hash(tx).Bytes(), account.privateKey)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(signer, signature)
}
