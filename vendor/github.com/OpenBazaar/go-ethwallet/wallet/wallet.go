package wallet

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"
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
	"github.com/nanmu42/etherscan-api"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
	"gopkg.in/yaml.v2"

	"github.com/OpenBazaar/go-ethwallet/util"
	ut "github.com/OpenBazaar/openbazaar-go/util"
)

var _ = wi.Wallet(&EthereumWallet{})
var done, doneBalanceTicker chan bool

const (
	// InfuraAPIKey is the hard coded Infura API key
	InfuraAPIKey = "v3/91c82af0169c4115940c76d331410749"
	// EtherScanAPIKey is needed for all Eherscan requests
	EtherScanAPIKey = "KA15D8FCHGBFZ4CQ25Y4NZM24417AXWF7M"
	maxGasLimit     = 400000
)

var (
	emptyChainHash *chainhash.Hash

	// EthCurrencyDefinition is eth defaults
	EthCurrencyDefinition = wi.CurrencyDefinition{
		Code:         "ETH",
		Divisibility: 18,
	}
	log = logging.MustGetLogger("ethwallet")
)

func init() {
	mustInitEmptyChainHash()
}

func mustInitEmptyChainHash() {
	hash, err := chainhash.NewHashFromStr("")
	if err != nil {
		panic(fmt.Sprintf("creating emptyChainHash: %s", err.Error()))
	}
	emptyChainHash = hash
}

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

// PendingTxn used to record a pending eth txn
type PendingTxn struct {
	TxnID     common.Hash
	OrderID   string
	Amount    string
	Nonce     int32
	From      string
	To        string
	WithInput bool
}

// SerializePendingTxn - used to serialize eth pending txn
func SerializePendingTxn(pTxn PendingTxn) ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(pTxn)
	return b.Bytes(), err
}

// DeserializePendingTxn - used to deserialize eth pending txn
func DeserializePendingTxn(b []byte) (PendingTxn, error) {
	pTxn := PendingTxn{}
	buf := bytes.NewBuffer(b)
	d := gob.NewDecoder(buf)
	err := d.Decode(&pTxn)
	return pTxn, err
}

// GenScriptHash - used to generate script hash for eth as per
// escrow smart contract
func GenScriptHash(script EthRedeemScript) ([32]byte, string, error) {
	a := make([]byte, 4)
	binary.BigEndian.PutUint32(a, script.Timeout)
	arr := append(script.TxnID.Bytes(), append([]byte{script.Threshold},
		append(a[:], append(script.Buyer.Bytes(),
			append(script.Seller.Bytes(), append(script.Moderator.Bytes(),
				append(script.MultisigAddress.Bytes())...)...)...)...)...)...)
	var retHash [32]byte

	copy(retHash[:], crypto.Keccak256(arr)[:])
	ahashStr := hexutil.Encode(retHash[:])

	return retHash, ahashStr, nil
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
	params        *chaincfg.Params
	listeners     []func(wi.TransactionCallback)
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

	return &EthereumWallet{client, myAccount, &EthAddress{&addr}, &Service{}, reg, nil, nil, nil, nil, []func(wi.TransactionCallback){}}
}

// NewEthereumWallet will return a reference to the Eth Wallet
func NewEthereumWallet(cfg config.CoinConfig, params *chaincfg.Params, mnemonic string, proxy proxy.Dialer) (*EthereumWallet, error) {
	client, err := NewEthClient(cfg.ClientAPIs[0] + "/" + InfuraAPIKey)
	if err != nil {
		log.Errorf("error initializing wallet: %v", err)
		return nil, err
	}
	var myAccount *Account
	myAccount, err = NewAccountFromMnemonic(mnemonic, "", params)
	if err != nil {
		log.Errorf("mnemonic based pk generation failed: %s", err.Error())
		return nil, err
	}
	addr := myAccount.Address()

	ethConfig := EthConfiguration{}

	var regAddr interface{}
	var ok bool
	registryKey := "RegistryAddress"
	if strings.Contains(cfg.ClientAPIs[0], "rinkeby") {
		registryKey = "RinkebyRegistryAddress"
	} else if strings.Contains(cfg.ClientAPIs[0], "ropsten") {
		registryKey = "RopstenRegistryAddress"
	}
	if regAddr, ok = cfg.Options[registryKey]; !ok {
		log.Errorf("ethereum registry not found: %s", cfg.Options[registryKey])
		return nil, err
	}

	ethConfig.RegistryAddress = regAddr.(string)

	reg, err := NewRegistry(common.HexToAddress(ethConfig.RegistryAddress), client)
	if err != nil {
		log.Errorf("error initilaizing contract failed: %s", err.Error())
		return nil, err
	}
	er := NewEthereumPriceFetcher(proxy)

	return &EthereumWallet{client, myAccount, &EthAddress{&addr}, &Service{}, reg, nil, cfg.DB, er, params, []func(wi.TransactionCallback){}}, nil
}

// Params - return nil to comply
func (wallet *EthereumWallet) Params() *chaincfg.Params {
	return wallet.params
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
func (wallet *EthereumWallet) Transfer(to string, value *big.Int, spendAll bool, fee big.Int) (common.Hash, error) {
	toAddress := common.HexToAddress(to)
	return wallet.client.Transfer(wallet.account, toAddress, value, spendAll, fee)
}

// Start will start the wallet daemon
func (wallet *EthereumWallet) Start() {
	done = make(chan bool)
	doneBalanceTicker = make(chan bool)
	// start the ticker to check for pending txn rcpts
	go func(wallet *EthereumWallet) {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				// get the pending txns
				txns, err := wallet.db.Txns().GetAll(true)
				if err != nil {
					continue
				}
				for _, txn := range txns {
					hash := common.HexToHash(txn.Txid)
					go func(txnData []byte) {
						_, err := wallet.checkTxnRcpt(&hash, txnData)
						if err != nil {
							log.Errorf(err.Error())
						}
					}(txn.Bytes)
				}
			}
		}
	}(wallet)

	// start the ticker to check for balance
	go func(wallet *EthereumWallet) {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		currentBalance, err := wallet.GetBalance()
		if err != nil {
			log.Infof("err fetching initial balance: %v", err)
		}
		currentTip, _ := wallet.ChainTip()

		for {
			select {
			case <-doneBalanceTicker:
				return
			case <-ticker.C:
				// fetch the current balance
				fetchedBalance, err := wallet.GetBalance()
				if err != nil {
					log.Infof("err fetching balance at %v: %v", time.Now(), err)
					continue
				}
				if fetchedBalance.Cmp(currentBalance) != 0 {
					// process balance change
					go wallet.processBalanceChange(currentBalance, fetchedBalance, currentTip)
					currentTip, _ = wallet.ChainTip()
					currentBalance = fetchedBalance
				}
			}
		}
	}(wallet)
}

func (wallet *EthereumWallet) processBalanceChange(previousBalance, currentBalance *big.Int, currentHead uint32) {
	count := 0
	cTip := int(currentHead)
	value := new(big.Int).Sub(currentBalance, previousBalance)
	for count < 30 {
		txns, err := wallet.TransactionsFromBlock(&cTip)
		if err == nil && len(txns) > 0 {
			count = 30
			txncb := wi.TransactionCallback{
				Txid:      util.EnsureCorrectPrefix(txns[0].Txid),
				Outputs:   []wi.TransactionOutput{},
				Inputs:    []wi.TransactionInput{},
				Height:    txns[0].Height,
				Timestamp: time.Now(),
				Value:     *value,
				WatchOnly: false,
			}
			for _, l := range wallet.listeners {
				go l(txncb)
			}
			continue
		}

		time.Sleep(2 * time.Second)
		count++
	}
}

func (wallet *EthereumWallet) invokeTxnCB(txnID string, value *big.Int) {
	txncb := wi.TransactionCallback{
		Txid:      util.EnsureCorrectPrefix(txnID),
		Outputs:   []wi.TransactionOutput{},
		Inputs:    []wi.TransactionInput{},
		Height:    0,
		Timestamp: time.Now(),
		Value:     *value,
		WatchOnly: false,
	}
	for _, l := range wallet.listeners {
		go l(txncb)
	}
}

// CurrencyCode returns ETH
func (wallet *EthereumWallet) CurrencyCode() string {
	if wallet.params == nil {
		return "ETH"
	}
	if wallet.params.Name == chaincfg.MainNetParams.Name {
		return "ETH"
	} else {
		return "TETH"
	}
	//return "ETH"
}

// IsDust Check if this amount is considered dust - 10000 wei
func (wallet *EthereumWallet) IsDust(amount big.Int) bool {
	return amount.Cmp(big.NewInt(10000)) <= 0
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

	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	version := []byte{0x04, 0x88, 0xad, 0xe4} // starts with xprv
	if !isPrivateKey {
		version = []byte{0x04, 0x88, 0xb2, 0x1e}
	}
	/*
		hdKey := hd.NewExtendedKey(
			version,
			keyBytes,
			chaincode,
			parentFP,
			0,
			0,
			isPrivateKey)
		return hdKey.Child(0)
	*/

	return hd.NewExtendedKey(version, keyBytes, chaincode, parentFP, 0, 0, isPrivateKey), nil
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
	var (
		ethAddr common.Address
		err     error
	)
	if len(addr) > 64 {
		ethAddr, err = ethScriptToAddr(addr)
		if err != nil {
			log.Error(err.Error())
		}
	} else {
		ethAddr = common.HexToAddress(addr)
	}

	return EthAddress{&ethAddr}, err
}

func ethScriptToAddr(addr string) (common.Address, error) {
	rScriptBytes, err := hex.DecodeString(addr)
	if err != nil {
		return common.Address{}, err
	}
	rScript, err := DeserializeEthScript(rScriptBytes)
	if err != nil {
		return common.Address{}, err
	}
	_, sHash, err := GenScriptHash(rScript)
	if err != nil {
		return common.Address{}, err
	}
	return common.HexToAddress(sHash), nil
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
func (wallet *EthereumWallet) Balance() (confirmed, unconfirmed wi.CurrencyValue) {
	var balance, ucbalance wi.CurrencyValue
	bal, err := wallet.GetBalance()
	if err == nil {
		balance = wi.CurrencyValue{
			Value:    *bal,
			Currency: EthCurrencyDefinition,
		}
	}
	ucbal, err := wallet.GetUnconfirmedBalance()
	ucb := big.NewInt(0)
	if err == nil {
		if ucbal.Cmp(bal) > 0 {
			ucb.Sub(ucbal, bal)
		}
	}
	ucbalance = wi.CurrencyValue{
		Value:    *ucb,
		Currency: EthCurrencyDefinition,
	}
	return balance, ucbalance
}

// TransactionsFromBlock - Returns a list of transactions for this wallet begining from the specified block
func (wallet *EthereumWallet) TransactionsFromBlock(startBlock *int) ([]wi.Txn, error) {
	ret := []wi.Txn{}

	unconf, _ := wallet.db.Txns().GetAll(false)

	txns, err := wallet.client.eClient.NormalTxByAddress(util.EnsureCorrectPrefix(wallet.account.Address().String()), startBlock, nil,
		1, 0, false)
	if err != nil && len(unconf) == 0 {
		log.Error("err fetching transactions : ", err)
		return []wi.Txn{}, nil
	}

	for _, t := range txns {
		status := wi.StatusConfirmed
		if t.Confirmations > 1 && t.Confirmations <= 7 {
			status = wi.StatusPending
		}
		prefix := ""
		if t.IsError != 0 {
			status = wi.StatusError
		}
		if strings.ToLower(t.From) == strings.ToLower(wallet.address.String()) {
			prefix = "-"
		}

		val := t.Value.Int().String()

		if val == "0" {	// Internal Transaction
			internalTxns, err := wallet.client.eClient.InternalTxByAddress(t.To, &t.BlockNumber, &t.BlockNumber, 1, 0, false)
			if err != nil && len(unconf) == 0 {
				log.Errorf("Transaction Errored: %v\n", err)
				continue
			}
			intVal, _ := new(big.Int).SetString("0", 10)
			for _, v := range internalTxns {
				fmt.Println(v.From, v.To, v.Value)
				if v.To == t.From {
					intVal = new(big.Int).Add(intVal, v.Value.Int())
				}
			}
			val = intVal.String()
		} else {
			val = prefix + val
		}

		tnew := wi.Txn{
			Txid:          util.EnsureCorrectPrefix(t.Hash),
			Value:         val,
			Height:        int32(t.BlockNumber),
			Timestamp:     t.TimeStamp.Time(),
			WatchOnly:     false,
			Confirmations: int64(t.Confirmations),
			Status:        wi.StatusCode(status),
			Bytes:         []byte(t.Input),
		}
		ret = append(ret, tnew)
	}

	for _, u := range unconf {
		u.Status = wi.StatusUnconfirmed
		ret = append(ret, u)
	}

	return ret, nil
}

// Transactions - Returns a list of transactions for this wallet
func (wallet *EthereumWallet) Transactions() ([]wi.Txn, error) {
	return wallet.TransactionsFromBlock(nil)
}

// GetTransaction - Get info on a specific transaction
func (wallet *EthereumWallet) GetTransaction(txid chainhash.Hash) (wi.Txn, error) {
	tx, _, err := wallet.client.GetTransaction(common.HexToHash(util.EnsureCorrectPrefix(txid.String())))
	if err != nil {
		return wi.Txn{}, err
	}

	chainID, err := wallet.client.NetworkID(context.Background())
	if err != nil {
		return wi.Txn{}, err
	}

	msg, err := tx.AsMessage(types.NewEIP155Signer(chainID)) // HomesteadSigner{})
	if err != nil {
		return wi.Txn{}, err
	}

	//value := tx.Value().String()
	fromAddr := msg.From()
	toAddr := msg.To()
	valueSub := big.NewInt(5000000)

	v, err := wallet.registry.GetRecommendedVersion(nil, "escrow")
	if err == nil {
		if tx.To().String() == v.Implementation.String() {
			toAddr = wallet.address.address
		}
		if msg.Value().Cmp(valueSub) > 0 {
			valueSub = msg.Value()
		}
	}

	return wi.Txn{
		Txid:        util.EnsureCorrectPrefix(tx.Hash().Hex()),
		Value:       tx.Value().String(),
		Height:      0,
		Timestamp:   time.Now(),
		WatchOnly:   false,
		Bytes:       tx.Data(),
		ToAddress:   util.EnsureCorrectPrefix(tx.To().String()),
		FromAddress: util.EnsureCorrectPrefix(msg.From().Hex()),
		Outputs: []wi.TransactionOutput{
			{
				Address: EthAddress{toAddr},
				Value:   *valueSub,
				Index:   0,
			},
			{
				Address: EthAddress{&fromAddr},
				Value:   *valueSub,
				Index:   1,
			},
			{
				Address: EthAddress{msg.To()},
				Value:   *valueSub,
				Index:   2,
			},
		},
	}, nil
}

// ChainTip - Get the height and best hash of the blockchain
func (wallet *EthereumWallet) ChainTip() (uint32, string) {
	num, hash, err := wallet.client.GetLatestBlock()
	if err != nil {
		return 0, ""
	}
	return num, hash.String()
}

// GetFeePerByte - Get the current fee per byte
func (wallet *EthereumWallet) GetFeePerByte(feeLevel wi.FeeLevel) big.Int {
	est, err := wallet.client.GetEthGasStationEstimate()
	ret := big.NewInt(0)
	if err != nil {
		log.Errorf("err fetching ethgas station data: %v", err)
		return *ret
	}
	switch feeLevel {
	case wi.NORMAL:
		ret, _ = big.NewFloat(est.Average * 100000000).Int(nil)
	case wi.ECONOMIC, wi.SUPER_ECONOMIC:
		ret, _ = big.NewFloat(est.SafeLow * 100000000).Int(nil)
	case wi.PRIOIRTY, wi.FEE_BUMP:
		ret, _ = big.NewFloat(est.Fast * 100000000).Int(nil)
	}
	return *ret
}

// Spend - Send ether to an external wallet
func (wallet *EthereumWallet) Spend(amount big.Int, addr btcutil.Address, feeLevel wi.FeeLevel, referenceID string, spendAll bool) (*chainhash.Hash, error) {
	var (
		hash common.Hash
		h *chainhash.Hash
		watchOnly bool
		nonce int32
		err error
	)
	actualRecipient := addr

	if referenceID == "" {
		// no referenceID means this is a direct transfer
		hash, err = wallet.Transfer(util.EnsureCorrectPrefix(addr.String()), &amount, spendAll, wallet.GetFeePerByte(feeLevel))
	} else {
		watchOnly = true
		// this is a spend which means it has to be linked to an order
		// specified using the referenceID

		// check if the addr is a multisig addr
		scripts, err := wallet.db.WatchedScripts().GetAll()
		if err != nil {
			return nil, err
		}
		isScript := false
		addrEth := common.HexToAddress(addr.String())
		key := addrEth.Bytes()
		redeemScript := []byte{}

		for _, script := range scripts {
			if bytes.Equal(key, script[:common.AddressLength]) {
				isScript = true
				redeemScript = script[common.AddressLength:]
				break
			}
		}

		if isScript {
			ethScript, err := DeserializeEthScript(redeemScript)
			if err != nil {
				return nil, err
			}
			_, scrHash, err := GenScriptHash(ethScript)
			if err != nil {
				log.Error(err.Error())
			}
			addrScrHash := common.HexToAddress(scrHash)
			actualRecipient = EthAddress{address: &addrScrHash}
			hash, _, err = wallet.callAddTransaction(ethScript, &amount, feeLevel)
			if err != nil {
				log.Errorf("error call add txn: %v", err)
				return nil, wi.ErrInsufficientFunds
			}
		} else {
			if !wallet.balanceCheck(feeLevel, amount) {
				return nil, wi.ErrInsufficientFunds
			}
			hash, err = wallet.Transfer(util.EnsureCorrectPrefix(addr.String()), &amount, spendAll, wallet.GetFeePerByte(feeLevel))
		}
		if err != nil {
			return nil, err
		}

		// txn is pending
		nonce, err = wallet.client.GetTxnNonce(util.EnsureCorrectPrefix(hash.Hex()))
		if err != nil {
			return nil, err
		}
	}
	if err == nil {
		h, err = util.CreateChainHash(hash.Hex())
		if err == nil {
			wallet.invokeTxnCB(h.String(), &amount)
		}
	}
	data, err := SerializePendingTxn(PendingTxn{
		TxnID:     hash,
		Amount:    amount.String(),
		OrderID:   referenceID,
		Nonce:     nonce,
		From:      wallet.address.EncodeAddress(),
		To:        actualRecipient.EncodeAddress(),
		WithInput: false,
	})
	if err == nil {
		err0 := wallet.db.Txns().Put(data, ut.NormalizeAddress(hash.Hex()), amount.String(), 0, time.Now(), watchOnly)
		if err0 != nil {
			log.Error(err0.Error())
		}
	}
	return h, nil
}

func (wallet *EthereumWallet) createTxnCallback(txID, orderID string, toAddress btcutil.Address, value big.Int, bTime time.Time, withInput bool, height int64) wi.TransactionCallback {
	output := wi.TransactionOutput{
		Address: toAddress,
		Value:   value,
		Index:   1,
		OrderID: orderID,
	}

	input := wi.TransactionInput{}

	if withInput {
		input = wi.TransactionInput{
			OutpointHash:  []byte(util.EnsureCorrectPrefix(txID)),
			OutpointIndex: 1,
			LinkedAddress: toAddress,
			Value:         value,
			OrderID:       orderID,
		}

	}
	return wi.TransactionCallback{
		Txid:      util.EnsureCorrectPrefix(txID),
		Outputs:   []wi.TransactionOutput{output},
		Inputs:    []wi.TransactionInput{input},
		Height:    int32(height),
		Timestamp: time.Now(),
		Value:     value,
		WatchOnly: false,
		BlockTime: bTime,
	}
}

func (wallet *EthereumWallet) AssociateTransactionWithOrder(txnCB wi.TransactionCallback) {
	for _, l := range wallet.listeners {
		go l(txnCB)
	}
}

// checkTxnRcpt check the txn rcpt status
func (wallet *EthereumWallet) checkTxnRcpt(hash *common.Hash, data []byte) (*common.Hash, error) {
	var rcpt *types.Receipt
	pTxn, err := DeserializePendingTxn(data)
	if err != nil {
		return nil, err
	}

	rcpt, err = wallet.client.TransactionReceipt(context.Background(), *hash)
	if err != nil {
		log.Infof("fetching txn rcpt: %v", err)
	}

	if rcpt != nil {
		// good. so the txn has been processed but we have to account for failed
		// but valid txn like some contract condition causing revert
		if rcpt.Status > 0 {
			// all good to update order state
			chash, err := util.CreateChainHash((*hash).Hex())
			if err != nil {
				return nil, err
			}
			err = wallet.db.Txns().Delete(chash)
			if err != nil {
				log.Errorf("err deleting the pending txn : %v", err)
			}
			n := new(big.Int)
			n, _ = n.SetString(pTxn.Amount, 10)
			toAddr := common.HexToAddress(pTxn.To)
			withInput := true
			if pTxn.Amount != "0" {
				toAddr = common.HexToAddress(util.EnsureCorrectPrefix(pTxn.To))
				withInput = pTxn.WithInput
			}
			height := rcpt.BlockNumber.Int64()
			go wallet.AssociateTransactionWithOrder(
				wallet.createTxnCallback(util.EnsureCorrectPrefix(hash.Hex()), pTxn.OrderID, EthAddress{&toAddr},
					*n, time.Now(), withInput, height))
		}
	}
	return hash, nil

}

// BumpFee - Bump the fee for the given transaction
func (wallet *EthereumWallet) BumpFee(txid chainhash.Hash) (*chainhash.Hash, error) {
	return util.CreateChainHash(txid.String())
}

// EstimateFee - Calculates the estimated size of the transaction and returns the total fee for the given feePerByte
func (wallet *EthereumWallet) EstimateFee(ins []wi.TransactionInput, outs []wi.TransactionOutput, feePerByte big.Int) big.Int {
	sum := big.NewInt(0)
	for _, out := range outs {
		gas, err := wallet.client.EstimateTxnGas(wallet.account.Address(),
			common.HexToAddress(out.Address.String()), &out.Value)
		if err != nil {
			return *sum
		}
		sum.Add(sum, gas)
	}
	return *sum
}

func (wallet *EthereumWallet) balanceCheck(feeLevel wi.FeeLevel, amount big.Int) bool {
	fee := wallet.GetFeePerByte(feeLevel)
	if fee.Int64() == 0 {
		return false
	}
	// lets check if the caller has enough balance to make the
	// multisign call
	requiredBalance := new(big.Int).Mul(&fee, big.NewInt(maxGasLimit))
	requiredBalance = new(big.Int).Add(requiredBalance, &amount)
	currentBalance, err := wallet.GetBalance()
	if err != nil {
		log.Error("err fetching eth wallet balance")
		currentBalance = big.NewInt(0)
	}
	if requiredBalance.Cmp(currentBalance) > 0 {
		// the wallet does not have the required balance
		return false
	}
	return true
}

// EstimateSpendFee - Build a spend transaction for the amount and return the transaction fee
func (wallet *EthereumWallet) EstimateSpendFee(amount big.Int, feeLevel wi.FeeLevel) (big.Int, error) {
	if !wallet.balanceCheck(feeLevel, amount) {
		return *big.NewInt(0), wi.ErrInsufficientFunds
	}
	gas, err := wallet.client.EstimateGasSpend(wallet.account.Address(), &amount)
	return *gas, err
}

// SweepAddress - Build and broadcast a transaction that sweeps all coins from an address. If it is a p2sh multisig, the redeemScript must be included
func (wallet *EthereumWallet) SweepAddress(utxos []wi.TransactionInput, address *btcutil.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wi.FeeLevel) (*chainhash.Hash, error) {

	outs := []wi.TransactionOutput{}
	for i, in := range utxos {
		out := wi.TransactionOutput{
			Address: wallet.address,
			Value:   in.Value,
			Index:   uint32(i),
			OrderID: in.OrderID,
		}
		outs = append(outs, out)
	}

	sigs, err := wallet.CreateMultisigSignature([]wi.TransactionInput{}, outs, key, *redeemScript, *big.NewInt(1))
	if err != nil {
		return nil, err
	}

	data, err := wallet.Multisign([]wi.TransactionInput{}, outs, sigs, []wi.Signature{}, *redeemScript, *big.NewInt(1), false)
	if err != nil {
		return nil, err
	}
	hash := common.BytesToHash(data)

	return util.CreateChainHash(hash.Hex())
}

// ExchangeRates - return the exchangerates
func (wallet *EthereumWallet) ExchangeRates() wi.ExchangeRates {
	return wallet.exchangeRates
}

func (wallet *EthereumWallet) callAddTransaction(script EthRedeemScript, value *big.Int, feeLevel wi.FeeLevel) (common.Hash, uint64, error) {

	h := common.BigToHash(big.NewInt(0))

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
	gasPriceETHGAS := wallet.GetFeePerByte(feeLevel)
	if gasPriceETHGAS.Int64() < gasPrice.Int64() {
		gasPriceETHGAS = *gasPrice
	}
	auth := bind.NewKeyedTransactor(wallet.account.privateKey)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = value          // in wei
	auth.GasLimit = maxGasLimit // in units
	auth.GasPrice = gasPrice

	// lets check if the caller has enough balance to make the
	// multisign call
	if !wallet.balanceCheck(feeLevel, *big.NewInt(0)) {
		// the wallet does not have the required balance
		return h, nonce, wi.ErrInsufficientFunds
	}

	shash, _, err := GenScriptHash(script)
	if err != nil {
		return h, nonce, err
	}

	smtct, err := NewEscrow(script.MultisigAddress, wallet.client)
	if err != nil {
		log.Fatalf("error initilaizing contract failed: %s", err.Error())
	}

	var tx *types.Transaction
	tx, err = smtct.AddTransaction(auth, script.Buyer, script.Seller,
		script.Moderator, script.Threshold, script.Timeout, shash, script.TxnID)
	if err == nil {
		h = tx.Hash()
	} else {
		return h, 0, err
	}

	txns = append(txns, wi.Txn{
		Txid:      tx.Hash().Hex(),
		Value:     value.String(),
		Height:    int32(nonce),
		Timestamp: time.Now(),
		WatchOnly: false,
		Bytes:     tx.Data()})

	return h, nonce, err

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

	var ecKeys []common.Address
	for _, key := range keys {
		ecKey, err := key.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		ePubkey := ecKey.ToECDSA()
		ecKeys = append(ecKeys, crypto.PubkeyToAddress(*ePubkey))
	}

	ver, err := wallet.registry.GetRecommendedVersion(nil, "escrow")
	if err != nil {
		log.Fatal(err)
	}

	if util.IsZeroAddress(ver.Implementation) {
		return nil, nil, errors.New("no escrow contract available")
	}

	builder := EthRedeemScript{}

	builder.TxnID = common.BytesToAddress(util.ExtractChaincode(&keys[0]))
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

	addr := common.HexToAddress(hexutil.Encode(crypto.Keccak256(redeemScript))) //hash.Sum(nil)[:]))
	retAddr := EthAddress{&addr}

	scriptKey := append(addr.Bytes(), redeemScript...)
	err = wallet.db.WatchedScripts().Put(scriptKey)
	if err != nil {
		log.Errorf("err saving the redeemscript: %v", err)
	}

	return retAddr, redeemScript, nil
}

// CreateMultisigSignature - Create a signature for a multisig transaction
func (wallet *EthereumWallet) CreateMultisigSignature(ins []wi.TransactionInput, outs []wi.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte big.Int) ([]wi.Signature, error) {

	payouts := []wi.TransactionOutput{}
	difference := new(big.Int)

	if len(ins) > 0 {
		totalVal := ins[0].Value
		outVal := new(big.Int)
		for _, out := range outs {
			outVal = new(big.Int).Add(outVal, &out.Value)
		}
		if totalVal.Cmp(outVal) != 0 {
			if totalVal.Cmp(outVal) < 0 {
				return nil, errors.New("payout greater than initial amount")
			}
			difference = new(big.Int).Sub(&totalVal, outVal)
		}
	}

	rScript, err := DeserializeEthScript(redeemScript)
	if err != nil {
		return nil, err
	}

	indx := []int{}
	mbvAddresses := make([]string, 3)

	for i, out := range outs {
		if out.Value.Cmp(new(big.Int)) > 0 {
			indx = append(indx, i)
		}
		if out.Address.String() == rScript.Moderator.Hex() {
			mbvAddresses[0] = out.Address.String()
		} else if out.Address.String() == rScript.Buyer.Hex() && (out.Value.Cmp(new(big.Int)) > 0) {
			mbvAddresses[1] = out.Address.String()
		} else {
			mbvAddresses[2] = out.Address.String()
		}
		p := wi.TransactionOutput{
			Address: out.Address,
			Value:   out.Value,
			Index:   out.Index,
			OrderID: out.OrderID,
		}
		payouts = append(payouts, p)
	}

	if len(indx) > 0 {
		diff := new(big.Int)
		delta := new(big.Int)
		diff.DivMod(difference, big.NewInt(int64(len(indx))), delta)
		for _, i := range indx {
			payouts[i].Value.Add(&payouts[i].Value, diff)
		}
		payouts[indx[0]].Value.Add(&payouts[indx[0]].Value, delta)
	}

	sort.Slice(payouts, func(i, j int) bool {
		return strings.Compare(payouts[i].Address.String(), payouts[j].Address.String()) == -1
	})

	var sigs []wi.Signature

	payables := make(map[string]big.Int)
	addresses := []string{}
	for _, out := range payouts {
		if out.Value.Cmp(big.NewInt(0)) <= 0 {
			continue
		}
		val := new(big.Int).SetBytes(out.Value.Bytes()) // &out.Value
		if p, ok := payables[out.Address.String()]; ok {
			sum := new(big.Int).Add(val, &p)
			payables[out.Address.String()] = *sum
		} else {
			payables[out.Address.String()] = *val
			addresses = append(addresses, out.Address.String())
		}
	}

	sort.Strings(addresses)
	destArr := []byte{}
	amountArr := []byte{}

	for _, k := range mbvAddresses {
		v := payables[k]
		if v.Cmp(big.NewInt(0)) != 1 {
			continue
		}
		addr := common.HexToAddress(k)
		sample := [32]byte{}
		sampleDest := [32]byte{}
		copy(sampleDest[12:], addr.Bytes())
		val := v.Bytes()
		l := len(val)

		copy(sample[32-l:], val)
		destArr = append(destArr, sampleDest[:]...)
		amountArr = append(amountArr, sample[:]...)
	}

	shash, _, err := GenScriptHash(rScript)
	if err != nil {
		return nil, err
	}

	var txHash [32]byte
	var payloadHash [32]byte

	/*
				// Follows ERC191 signature scheme: https://github.com/ethereum/EIPs/issues/191
		        bytes32 txHash = keccak256(
		            abi.encodePacked(
		                "\x19Ethereum Signed Message:\n32",
		                keccak256(
		                    abi.encodePacked(
		                        byte(0x19),
		                        byte(0),
		                        this,
		                        destinations,
		                        amounts,
		                        scriptHash
		                    )
		                )
		            )
		        );

	*/

	payload := []byte{byte(0x19), byte(0)}
	payload = append(payload, rScript.MultisigAddress.Bytes()...)
	payload = append(payload, destArr...)
	payload = append(payload, amountArr...)
	payload = append(payload, shash[:]...)

	pHash := crypto.Keccak256(payload)
	copy(payloadHash[:], pHash)

	txData := []byte{byte(0x19)}
	txData = append(txData, []byte("Ethereum Signed Message:\n32")...)
	txData = append(txData, payloadHash[:]...)
	txnHash := crypto.Keccak256(txData)
	log.Debugf("txnHash        : %s", hexutil.Encode(txnHash))
	log.Debugf("phash          : %s", hexutil.Encode(payloadHash[:]))
	copy(txHash[:], txnHash)

	sig, err := crypto.Sign(txHash[:], wallet.account.privateKey)
	if err != nil {
		log.Errorf("error signing in createmultisig : %v", err)
	}
	sigs = append(sigs, wi.Signature{InputIndex: 1, Signature: sig})

	return sigs, err
}

// Multisign - Combine signatures and optionally broadcast
func (wallet *EthereumWallet) Multisign(ins []wi.TransactionInput, outs []wi.TransactionOutput, sigs1 []wi.Signature, sigs2 []wi.Signature, redeemScript []byte, feePerByte big.Int, broadcast bool) ([]byte, error) {

	payouts := []wi.TransactionOutput{}
	difference := new(big.Int)

	if len(ins) > 0 {
		totalVal := &ins[0].Value
		outVal := new(big.Int)
		for _, out := range outs {
			outVal.Add(outVal, &out.Value)
		}
		if totalVal.Cmp(outVal) != 0 {
			if totalVal.Cmp(outVal) < 0 {
				return nil, errors.New("payout greater than initial amount")
			}
			difference.Sub(totalVal, outVal)
		}
	}

	rScript, err := DeserializeEthScript(redeemScript)
	if err != nil {
		return nil, err
	}

	indx := []int{}
	referenceID := ""
	mbvAddresses := make([]string, 3)

	for i, out := range outs {
		if out.Value.Cmp(new(big.Int)) > 0 {
			indx = append(indx, i)
		}
		if out.Address.String() == rScript.Moderator.Hex() {
			indx = append(indx, i)
			mbvAddresses[0] = out.Address.String()
		} else if out.Address.String() == rScript.Buyer.Hex() {
			mbvAddresses[1] = out.Address.String()
		} else {
			mbvAddresses[2] = out.Address.String()
		}
		p := wi.TransactionOutput{
			Address: out.Address,
			Value:   out.Value,
			Index:   out.Index,
			OrderID: out.OrderID,
		}
		referenceID = out.OrderID
		payouts = append(payouts, p)
	}

	if len(indx) > 0 {
		diff := new(big.Int)
		delta := new(big.Int)
		diff.DivMod(difference, big.NewInt(int64(len(indx))), delta)
		for _, i := range indx {
			payouts[i].Value.Add(&payouts[i].Value, diff)
		}
		payouts[indx[0]].Value.Add(&payouts[indx[0]].Value, delta)
	}

	sort.Slice(payouts, func(i, j int) bool {
		return strings.Compare(payouts[i].Address.String(), payouts[j].Address.String()) == -1
	})

	payables := make(map[string]big.Int)
	for _, out := range payouts {
		if out.Value.Cmp(big.NewInt(0)) <= 0 {
			continue
		}
		val := new(big.Int).SetBytes(out.Value.Bytes()) // &out.Value
		if p, ok := payables[out.Address.String()]; ok {
			sum := new(big.Int).Add(val, &p)
			payables[out.Address.String()] = *sum
		} else {
			payables[out.Address.String()] = *val
		}
	}

	rSlice := [][32]byte{}
	sSlice := [][32]byte{}
	vSlice := []uint8{}

	var r [32]byte
	var s [32]byte
	var v uint8

	if len(sigs1) > 0 && len(sigs1[0].Signature) > 0 {
		r, s, v = util.SigRSV(sigs1[0].Signature)
		rSlice = append(rSlice, r)
		sSlice = append(sSlice, s)
		vSlice = append(vSlice, v)
	}

	if len(sigs2) > 0 && len(sigs2[0].Signature) > 0 {
		r, s, v = util.SigRSV(sigs2[0].Signature)
		rSlice = append(rSlice, r)
		sSlice = append(sSlice, s)
		vSlice = append(vSlice, v)
	}

	shash, _, err := GenScriptHash(rScript)
	if err != nil {
		return nil, err
	}

	smtct, err := NewEscrow(rScript.MultisigAddress, wallet.client)
	if err != nil {
		log.Fatalf("error initializing contract failed: %s", err.Error())
	}

	destinations := []common.Address{}
	amounts := []*big.Int{}

	for _, k := range mbvAddresses {
		v := payables[k]
		if v.Cmp(big.NewInt(0)) == 1 {
			destinations = append(destinations, common.HexToAddress(k))
			amounts = append(amounts, new(big.Int).SetBytes(v.Bytes()))
		}
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
	auth.Value = big.NewInt(0)  // in wei
	auth.GasLimit = maxGasLimit // in units
	auth.GasPrice = gasPrice

	// lets check if the caller has enough balance to make the
	// multisign call
	requiredBalance := new(big.Int).Mul(gasPrice, big.NewInt(maxGasLimit))
	currentBalance, err := wallet.GetBalance()
	if err != nil {
		log.Error("err fetching eth wallet balance")
		currentBalance = big.NewInt(0)
	}

	if requiredBalance.Cmp(currentBalance) > 0 {
		// the wallet does not have the required balance
		return nil, wi.ErrInsufficientFunds
	}

	tx, txnErr := smtct.Execute(auth, vSlice, rSlice, sSlice, shash, destinations, amounts)

	if txnErr != nil {
		return nil, txnErr
	}

	txns = append(txns, wi.Txn{
		Txid:      tx.Hash().Hex(),
		Value:     "0",
		Height:    int32(nonce),
		Timestamp: time.Now(),
		WatchOnly: false,
		Bytes:     tx.Data()})

	// this is a pending txn
	_, scrHash, err := GenScriptHash(rScript)
	if err != nil {
		log.Error(err.Error())
	}
	data, err := SerializePendingTxn(PendingTxn{
		TxnID:   tx.Hash(),
		Amount:  "0",
		OrderID: referenceID,
		Nonce:   int32(nonce),
		From:    wallet.address.EncodeAddress(),
		To:      scrHash,
	})
	if err == nil {
		err0 := wallet.db.Txns().Put(data, ut.NormalizeAddress(tx.Hash().Hex()), "0", 0, time.Now(), true)
		if err0 != nil {
			log.Error(err0.Error())
		}
	}

	return tx.Hash().Bytes(), nil
}

// AddWatchedAddresses - Add a script to the wallet and get notifications back when coins are received or spent from it
func (wallet *EthereumWallet) AddWatchedAddresses(addrs ...btcutil.Address) error {
	// the reason eth wallet cannot use this as of now is because only the address
	// is insufficient, the redeemScript is also required
	return nil
}

// AddTransactionListener will call the function callback when new transactions are discovered
func (wallet *EthereumWallet) AddTransactionListener(callback func(wi.TransactionCallback)) {
	// add incoming txn listener using service
	wallet.listeners = append(wallet.listeners, callback)
}

// ReSyncBlockchain - Use this to re-download merkle blocks in case of missed transactions
func (wallet *EthereumWallet) ReSyncBlockchain(fromTime time.Time) {
	// use service here
}

// GetConfirmations - Return the number of confirmations and the height for a transaction
func (wallet *EthereumWallet) GetConfirmations(txid chainhash.Hash) (confirms, atHeight uint32, err error) {
	// TODO: etherscan api is being used
	// when mainnet is activated we may need a way to set the
	// url correctly - done 6 April 2019
	hash := common.HexToHash(util.EnsureCorrectPrefix(txid.String()))
	network := etherscan.Rinkby
	if strings.Contains(wallet.client.url, "mainnet") {
		network = etherscan.Mainnet
	}
	urlStr := fmt.Sprintf("https://%s.etherscan.io/api?module=proxy&action=eth_getTransactionByHash&txhash=%s&apikey=%s",
		network, hash.String(), EtherScanAPIKey)
	res, err := http.Get(urlStr)
	if err != nil {
		return 0, 0, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, 0, err
	}
	if len(body) == 0 {
		return 0, 0, errors.New("invalid txn hash")
	}
	var s map[string]interface{}
	err = json.Unmarshal(body, &s)
	if err != nil {
		return 0, 0, err
	}

	if s["result"] == nil {
		return 0, 0, errors.New("invalid txn hash")
	}

	if s["message"] != nil {
		return 0, 0, nil
	}

	result := s["result"].(map[string]interface{})

	var d, conf int64
	if result["blockNumber"] != nil {
		d, _ = strconv.ParseInt(result["blockNumber"].(string), 0, 64)
	} else {
		d = 0
	}

	n, err := wallet.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, 0, err
	}

	if d != 0 {
		conf = n.Number.Int64() - d + 1
	} else {
		conf = 0
	}

	return uint32(conf), uint32(d), nil
}

// Close will stop the wallet daemon
func (wallet *EthereumWallet) Close() {
	// stop the wallet daemon
	done <- true
	doneBalanceTicker <- true
}

// CreateAddress - used to generate a new address
func (wallet *EthereumWallet) CreateAddress() (common.Address, error) {
	fromAddress := wallet.account.Address()
	nonce, err := wallet.client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Error(err.Error())
	}
	addr := crypto.CreateAddress(fromAddress, nonce)
	return addr, err
}

// PrintKeys - used to print the keys for this wallet
func (wallet *EthereumWallet) PrintKeys() {
	privateKeyBytes := crypto.FromECDSA(wallet.account.privateKey)
	log.Debug(string(privateKeyBytes))
	publicKey := wallet.account.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	log.Debug(address)
	log.Debug(string(publicKeyBytes))
}

// GenWallet creates a wallet
func GenWallet() {
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
