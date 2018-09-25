package wallet

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"path"
	"runtime"
	"time"

	"github.com/OpenBazaar/multiwallet/config"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"golang.org/x/net/proxy"

	"github.com/OpenBazaar/go-ethwallet/util"
)

const (
	// InfuraAPIKey is the hard coded Infura API key
	InfuraAPIKey = "openbazaar"
)

// EthConfiguration - used for eth specific configuration
type EthConfiguration struct {
	RopstenPPAddress string `yaml:"ROPSTEN_PPv2_ADDRESS"`
	RegistryAddress  string `yaml:"ROPSTEN_REGISTRY"`
}

// EthRedeemScript - used to represent redeem script for eth wallet
// <uniqueId: 20><threshold:1><timeoutHours:4><buyer:20><seller:20>
// <moderator:20><multisigAddress:20><tokenAddress:20>
type EthRedeemScript struct {
	TxnID           common.Address
	Threshold       uint8
	Timeout         uint32
	Buyer           common.Address
	Seller          common.Address
	Moderator       common.Address
	MultisigAddress common.Address
	TokenAddress    common.Address
}

// SerializeEthScript - used to serialize eth redeem script
func SerializeEthScript(scrpt EthRedeemScript) ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(scrpt)
	return b.Bytes(), err
}

// DeserializeEthScript - used to deserialize eth redeem script
func DeserializeEthScript(b []byte) (EthRedeemScript, error) {
	scrpt := EthRedeemScript{}
	buf := bytes.NewBuffer(b)
	d := gob.NewDecoder(buf)
	err := d.Decode(&scrpt)
	return scrpt, err
}

// EthereumWallet is the wallet implementation for ethereum
type EthereumWallet struct {
	client        *EthClient
	account       *Account
	address       *EthAddress
	service       *Service
	registry      *Registry
	ppsct         *Escrow
	db            wi.Datastore
	exchangeRates wi.ExchangeRates
}

// NewEthereumWalletWithKeyfile will return a reference to the Eth Wallet
func NewEthereumWalletWithKeyfile(url, keyFile, passwd string) *EthereumWallet {
	client, err := NewEthClient(url)
	if err != nil {
		log.Fatalf("error initializing wallet: %v", err)
	}
	var myAccount *Account
	myAccount, err = NewAccountFromKeyfile(keyFile, passwd)
	if err != nil {
		log.Fatalf("key file validation failed: %s", err.Error())
	}
	addr := myAccount.Address()

	_, filename, _, _ := runtime.Caller(0)
	conf, err := ioutil.ReadFile(path.Join(path.Dir(filename), "../configuration.yaml"))

	if err != nil {
		log.Fatalf("ethereum config not found: %s", err.Error())
	}
	ethConfig := EthConfiguration{}
	err = yaml.Unmarshal(conf, &ethConfig)
	if err != nil {
		log.Fatalf("ethereum config not valid: %s", err.Error())
	}

	reg, err := NewRegistry(common.HexToAddress(ethConfig.RegistryAddress), client)
	if err != nil {
		log.Fatalf("error initilaizing contract failed: %s", err.Error())
	}

	//reg.GetVersionDetails()

	//smtct, err := NewWallet(common.HexToAddress(ethConfig.RopstenPPAddress), client)
	//if err != nil {
	//	log.Fatalf("error initilaizing contract failed: %s", err.Error())
	//}

	return &EthereumWallet{client, myAccount, &EthAddress{&addr}, &Service{}, reg, nil, nil, nil}
}

// NewEthereumWallet will return a reference to the Eth Wallet
func NewEthereumWallet(cfg config.CoinConfig, mnemonic string, proxy proxy.Dialer) (*EthereumWallet, error) {
	client, err := NewEthClient(cfg.ClientAPI.String() + "/" + InfuraAPIKey)
	if err != nil {
		log.Errorf("error initializing wallet: %v", err)
		return nil, err
	}
	var myAccount *Account
	/*
		seed := bip39.NewSeed(mnemonic, "")

		privateKeyECDSA, err := crypto.ToECDSA(seed)
		if err != nil {
			return nil
		}

		key := &keystore.Key{
			Address:    crypto.PubkeyToAddress(privateKeyECDSA.PublicKey),
			PrivateKey: privateKeyECDSA,
		}

		myAccount = &Account{key}
	*/
	myAccount, err = NewAccountFromMnemonic(mnemonic, "")
	if err != nil {
		log.Errorf("mnemonic based pk generation failed: %s", err.Error())
		return nil, err
	}
	addr := myAccount.Address()

	ethConfig := EthConfiguration{}

	var regAddr interface{}
	var ok bool
	if regAddr, ok = cfg.Options["RegistryAddress"]; !ok {
		log.Errorf("ethereum registry not found: %s", err.Error())
		return nil, err
	}

	ethConfig.RegistryAddress = regAddr.(string)

	/*
		_, filename, _, _ := runtime.Caller(0)
		conf, err := ioutil.ReadFile(path.Join(path.Dir(filename), "../configuration.yaml"))

		if err != nil {
			log.Errorf("ethereum config not found: %s", err.Error())
			return nil, err
		}

		err = yaml.Unmarshal(conf, &ethConfig)
		if err != nil {
			log.Errorf("ethereum config not valid: %s", err.Error())
			return nil, err
		}
	*/

	reg, err := NewRegistry(common.HexToAddress(ethConfig.RegistryAddress), client)
	if err != nil {
		log.Errorf("error initilaizing contract failed: %s", err.Error())
		return nil, err
	}

	//reg.GetVersionDetails()

	//smtct, err := NewWallet(common.HexToAddress(ethConfig.RopstenPPAddress), client)
	//if err != nil {
	//	log.Fatalf("error initilaizing contract failed: %s", err.Error())
	//}

	er := NewEthereumPriceFetcher(proxy)

	return &EthereumWallet{client, myAccount, &EthAddress{&addr}, &Service{}, reg, nil, cfg.DB, er}, nil
}

// Params - return nil to comply
func (wallet *EthereumWallet) Params() *chaincfg.Params {
	return nil
}

// GetBalance returns the balance for the wallet
func (wallet *EthereumWallet) GetBalance() (*big.Int, error) {
	return wallet.client.GetBalance(wallet.account.Address())
}

// GetUnconfirmedBalance returns the unconfirmed balance for the wallet
func (wallet *EthereumWallet) GetUnconfirmedBalance() (*big.Int, error) {
	return wallet.client.GetUnconfirmedBalance(wallet.account.Address())
}

// Transfer will transfer the amount from this wallet to the spec address
func (wallet *EthereumWallet) Transfer(to string, value *big.Int) (common.Hash, error) {
	toAddress := common.HexToAddress(to)
	return wallet.client.Transfer(wallet.account, toAddress, value)
}

// Start will start the wallet daemon
func (wallet *EthereumWallet) Start() {
	// daemonize the wallet
}

// CurrencyCode returns ETH
func (wallet *EthereumWallet) CurrencyCode() string {
	return "ETH"
}

// IsDust Check if this amount is considered dust - 10000 wei
func (wallet *EthereumWallet) IsDust(amount int64) bool {
	return amount < 10000
}

// MasterPrivateKey - Get the master private key
func (wallet *EthereumWallet) MasterPrivateKey() *hd.ExtendedKey {
	return hd.NewExtendedKey([]byte{0x00, 0x00, 0x00, 0x00}, wallet.account.privateKey.D.Bytes(),
		wallet.account.address.Bytes(), wallet.account.address.Bytes(), 0, 0, true)
}

// MasterPublicKey - Get the master public key
func (wallet *EthereumWallet) MasterPublicKey() *hd.ExtendedKey {
	publicKey := wallet.account.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	return hd.NewExtendedKey([]byte{0x00, 0x00, 0x00, 0x00}, publicKeyBytes,
		wallet.account.address.Bytes(), wallet.account.address.Bytes(), 0, 0, false)
}

// ChildKey Generate a child key using the given chaincode. The key is used in multisig transactions.
// For most implementations this should just be child key 0.
func (wallet *EthereumWallet) ChildKey(keyBytes []byte, chaincode []byte, isPrivateKey bool) (*hd.ExtendedKey, error) {
	if isPrivateKey {
		return wallet.MasterPrivateKey(), nil
	}
	return wallet.MasterPublicKey(), nil
}

// CurrentAddress - Get the current address for the given purpose
func (wallet *EthereumWallet) CurrentAddress(purpose wi.KeyPurpose) btcutil.Address {
	return *wallet.address
}

// NewAddress - Returns a fresh address that has never been returned by this function
func (wallet *EthereumWallet) NewAddress(purpose wi.KeyPurpose) btcutil.Address {
	return *wallet.address
}

// DecodeAddress - Parse the address string and return an address interface
func (wallet *EthereumWallet) DecodeAddress(addr string) (btcutil.Address, error) {
	ethAddr := common.HexToAddress(addr)
	if wallet.HasKey(EthAddress{&ethAddr}) {
		return *wallet.address, nil
	}
	return EthAddress{}, errors.New("invalid or unknown address")
}

// ScriptToAddress - ?
func (wallet *EthereumWallet) ScriptToAddress(script []byte) (btcutil.Address, error) {
	return wallet.address, nil
}

// HasKey - Returns if the wallet has the key for the given address
func (wallet *EthereumWallet) HasKey(addr btcutil.Address) bool {
	if !util.IsValidAddress(addr.String()) {
		return false
	}
	return wallet.account.Address().String() == addr.String()
}

// Balance - Get the confirmed and unconfirmed balances
func (wallet *EthereumWallet) Balance() (confirmed, unconfirmed int64) {
	var balance, ucbalance int64
	bal, err := wallet.GetBalance()
	if err == nil {
		balance = bal.Int64()
	}
	ucbal, err := wallet.GetUnconfirmedBalance()
	if err == nil {
		ucbalance = ucbal.Int64()
	}
	return balance, ucbalance
}

// Transactions - Returns a list of transactions for this wallet
func (wallet *EthereumWallet) Transactions() ([]wi.Txn, error) {
	return txns, nil
}

// GetTransaction - Get info on a specific transaction
func (wallet *EthereumWallet) GetTransaction(txid chainhash.Hash) (wi.Txn, error) {
	tx, _, err := wallet.client.GetTransaction(common.HexToHash(txid.String()))
	if err != nil {
		return wi.Txn{}, err
	}
	return wi.Txn{
		Txid:      tx.Hash().String(),
		Value:     tx.Value().Int64(),
		Height:    0,
		Timestamp: time.Now(),
		WatchOnly: false,
		Bytes:     tx.Data(),
	}, nil
}

// ChainTip - Get the height and best hash of the blockchain
func (wallet *EthereumWallet) ChainTip() (uint32, chainhash.Hash) {
	num, hash, err := wallet.client.GetLatestBlock()
	h, _ := chainhash.NewHashFromStr("")
	if err != nil {
		return 0, *h
	}
	h, _ = chainhash.NewHashFromStr(hash)
	return num, *h
}

// GetFeePerByte - Get the current fee per byte
func (wallet *EthereumWallet) GetFeePerByte(feeLevel wi.FeeLevel) uint64 {
	return 0
}

// Spend - Send ether to an external wallet
func (wallet *EthereumWallet) Spend(amount int64, addr btcutil.Address, feeLevel wi.FeeLevel) (*chainhash.Hash, error) {
	hash, err := wallet.Transfer(addr.String(), big.NewInt(amount))
	var h *chainhash.Hash
	if err == nil {
		h, err = chainhash.NewHashFromStr(hash.String())
	}
	return h, err
}

// BumpFee - Bump the fee for the given transaction
func (wallet *EthereumWallet) BumpFee(txid chainhash.Hash) (*chainhash.Hash, error) {
	return chainhash.NewHashFromStr(txid.String())
}

// EstimateFee - Calculates the estimated size of the transaction and returns the total fee for the given feePerByte
func (wallet *EthereumWallet) EstimateFee(ins []wi.TransactionInput, outs []wi.TransactionOutput, feePerByte uint64) uint64 {
	sum := big.NewInt(0)
	for _, out := range outs {
		gas, err := wallet.client.EstimateTxnGas(wallet.account.Address(),
			common.HexToAddress(out.Address.String()), big.NewInt(out.Value))
		if err != nil {
			return sum.Uint64()
		}
		sum.Add(sum, gas)
	}
	return sum.Uint64()
}

// EstimateSpendFee - Build a spend transaction for the amount and return the transaction fee
func (wallet *EthereumWallet) EstimateSpendFee(amount int64, feeLevel wi.FeeLevel) (uint64, error) {
	gas, err := wallet.client.EstimateGasSpend(wallet.account.Address(), big.NewInt(amount))
	return gas.Uint64(), err
}

// SweepAddress - Build and broadcast a transaction that sweeps all coins from an address. If it is a p2sh multisig, the redeemScript must be included
func (wallet *EthereumWallet) SweepAddress(utxos []wi.TransactionInput, address *btcutil.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wi.FeeLevel) (*chainhash.Hash, error) {
	return chainhash.NewHashFromStr("")
}

// GenerateMultisigScript - Generate a multisig script from public keys. If a timeout is included the returned script should be a timelocked escrow which releases using the timeoutKey.
func (wallet *EthereumWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (btcutil.Address, []byte, error) {
	if uint32(timeout.Hours()) > 0 && timeoutKey == nil {
		return nil, nil, errors.New("timeout key must be non nil when using an escrow timeout")
	}

	if len(keys) < threshold {
		return nil, nil, fmt.Errorf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", threshold, len(keys))
	}

	if len(keys) < 2 {
		return nil, nil, fmt.Errorf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", threshold, len(keys))
	}

	// call registry to get the deployed address for the escrow ct
	fromAddress := wallet.account.Address()
	nonce, err := wallet.client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}
	gasPrice, err := wallet.client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	auth := bind.NewKeyedTransactor(wallet.account.privateKey)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0) // in wei
	auth.GasLimit = 4000000    // in units
	auth.GasPrice = gasPrice

	ver, err := wallet.registry.GetRecommendedVersion(nil, "escrow")
	if err != nil {
		log.Fatal(err)
	}

	if util.IsZeroAddress(ver.Implementation) {
		return nil, nil, errors.New("no escrow contract available")
	}

	smtct, err := NewEscrow(ver.Implementation, wallet.client)
	if err != nil {
		log.Fatalf("error initilaizing contract failed: %s", err.Error())
	}

	var ecKeys []common.Address
	for _, key := range keys {
		ecKey, err := key.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		ecKeys = append(ecKeys, common.BytesToAddress(ecKey.SerializeUncompressed()))
	}

	builder := EthRedeemScript{}

	builder.TxnID = common.HexToAddress(xid.New().String() + xid.New().String())
	builder.Timeout = uint32(timeout.Hours())
	builder.Threshold = uint8(threshold)
	builder.Buyer = ecKeys[0]
	builder.Seller = ecKeys[1]
	builder.MultisigAddress = ver.Implementation

	if threshold > 1 {
		builder.Moderator = ecKeys[2]
	}
	switch threshold {
	case 1:
		{
			// Seller is offline
		}
	case 2:
		{
			// Moderated payment
		}
	default:
		{
			// handle this
		}
	}

	redeemScript, err := SerializeEthScript(builder)
	if err != nil {
		return nil, nil, err
	}

	hash := sha3.NewKeccak256()
	hash.Write(redeemScript)
	addr := common.HexToAddress(hexutil.Encode(hash.Sum(nil)[:]))
	retAddr := EthAddress{&addr}
	var shash [32]byte
	copy(shash[:], hash.Sum(nil)[:])

	var tx *types.Transaction

	if builder.Threshold == 1 {
		tx, err = smtct.AddTransaction(auth, builder.Buyer, builder.Seller,
			[]common.Address{}, builder.Threshold,
			builder.Timeout, shash)
	} else {
		tx, err = smtct.AddTransaction(auth, builder.Buyer, builder.Seller,
			[]common.Address{builder.Moderator}, builder.Threshold,
			builder.Timeout, shash)
	}

	fmt.Println(tx)
	fmt.Println(err)

	return retAddr, redeemScript, nil
}

// CreateMultisigSignature - Create a signature for a multisig transaction
func (wallet *EthereumWallet) CreateMultisigSignature(ins []wi.TransactionInput, outs []wi.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]wi.Signature, error) {

	var sigs []wi.Signature
	shash := crypto.Keccak256(redeemScript)

	sig, err := crypto.Sign(shash, wallet.account.privateKey)
	if err != nil {
		log.Errorf("error signing in createmultisig : %v", err)
	}
	sigs = append(sigs, wi.Signature{InputIndex: 1, Signature: sig})

	return sigs, err
}

// Multisign - Combine signatures and optionally broadcast
func (wallet *EthereumWallet) Multisign(ins []wi.TransactionInput, outs []wi.TransactionOutput, sigs1 []wi.Signature, sigs2 []wi.Signature, redeemScript []byte, feePerByte uint64, broadcast bool) ([]byte, error) {

	var buf bytes.Buffer

	payables := make(map[string]*big.Int)
	for _, out := range outs {
		val := big.NewInt(out.Value)
		if p, ok := payables[out.Address.String()]; ok {
			sum := big.NewInt(0)
			sum.Add(val, p)
			payables[out.Address.String()] = sum
		} else {
			payables[out.Address.String()] = val
		}
	}

	rSlice := make([][32]byte, 2)
	sSlice := make([][32]byte, 2)
	vSlice := make([]uint8, 2)

	r := [32]byte{}
	s := [32]byte{}
	v := uint8(0)

	if len(sigs1[0].Signature) > 0 {
		r, s, v = util.SigRSV(sigs1[0].Signature)
		rSlice = append(rSlice, r)
		sSlice = append(sSlice, s)
		vSlice = append(vSlice, v)
	}

	r = [32]byte{}
	s = [32]byte{}
	v = uint8(0)

	if len(sigs2[0].Signature) > 0 {
		r, s, v = util.SigRSV(sigs2[0].Signature)
		rSlice = append(rSlice, r)
		sSlice = append(sSlice, s)
		vSlice = append(vSlice, v)
	}

	hash := crypto.Keccak256(redeemScript)
	var shash [32]byte
	copy(shash[:], hash)

	rScript, err := DeserializeEthScript(redeemScript)
	if err != nil {
		return nil, err
	}

	smtct, err := NewEscrow(rScript.MultisigAddress, wallet.client)
	if err != nil {
		log.Fatalf("error initilaizing contract failed: %s", err.Error())
	}

	destinations := []common.Address{}
	amounts := []*big.Int{}

	for k, v := range payables {
		destinations = append(destinations, common.HexToAddress(k))
		amounts = append(amounts, v)
	}

	fromAddress := wallet.account.Address()
	nonce, err := wallet.client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}
	gasPrice, err := wallet.client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	auth := bind.NewKeyedTransactor(wallet.account.privateKey)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0) // in wei
	auth.GasLimit = 4000000    // in units
	auth.GasPrice = gasPrice

	var tx *types.Transaction

	tx, err = smtct.Execute(auth, vSlice, rSlice, sSlice, shash, rScript.TxnID, destinations, amounts)

	fmt.Println(tx)
	fmt.Println(err)

	return buf.Bytes(), nil
}

// AddWatchedAddress - Add a script to the wallet and get notifications back when coins are received or spent from it
func (wallet *EthereumWallet) AddWatchedAddress(address btcutil.Address) error {
	return nil
}

// AddTransactionListener - add a txn listener
func (wallet *EthereumWallet) AddTransactionListener(callback func(wi.TransactionCallback)) {
	// add incoming txn listener using service
}

// ReSyncBlockchain - Use this to re-download merkle blocks in case of missed transactions
func (wallet *EthereumWallet) ReSyncBlockchain(fromTime time.Time) {
	// use service here
}

// GetConfirmations - Return the number of confirmations and the height for a transaction
func (wallet *EthereumWallet) GetConfirmations(txid chainhash.Hash) (confirms, atHeight uint32, err error) {
	return 0, 0, nil
}

// Close will stop the wallet daemon
func (wallet *EthereumWallet) Close() {
	// stop the wallet daemon
}

// CreateAddress - used to generate a new address
func (wallet *EthereumWallet) CreateAddress() (common.Address, error) {
	fromAddress := wallet.account.Address()
	nonce, err := wallet.client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		fmt.Println(err)
	}
	addr := crypto.CreateAddress(fromAddress, nonce)
	//fmt.Println("Addr : ", addr.String())
	return addr, err
}

// GenWallet creates a wallet
func GenWallet() {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	privateKeyBytes := crypto.FromECDSA(privateKey)
	fmt.Println(hexutil.Encode(privateKeyBytes)[2:]) // fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	fmt.Println(hexutil.Encode(publicKeyBytes)[4:]) // 9a7df67f79246283fdc93af76d4f8cdd62c4886e8cd870944e817dd0b97934fdd7719d0810951e03418205868a5c1b40b192451367f28e0088dd75e15de40c05

	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	fmt.Println(address) // 0x96216849c49358B10257cb55b28eA603c874b05E

	hash := sha3.NewKeccak256()
	hash.Write(publicKeyBytes[1:])
	fmt.Println(hexutil.Encode(hash.Sum(nil)[12:])) // 0x96216849c49358b10257cb55b28ea603c874b05e

	fmt.Println(util.IsValidAddress(address))

}

// GenDefaultKeyStore will generate a default keystore
func GenDefaultKeyStore(passwd string) (*Account, error) {
	ks := keystore.NewKeyStore("./", keystore.StandardScryptN, keystore.StandardScryptP)

	account, err := ks.NewAccount(passwd)
	if err != nil {
		return nil, err
	}
	return NewAccountFromKeyfile(account.URL.Path, passwd)
}
