package wallet

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/OpenBazaar/multiwallet/config"
	ut "github.com/OpenBazaar/openbazaar-go/util"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/davecgh/go-spew/spew"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/nanmu42/etherscan-api"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"

	"github.com/OpenBazaar/go-ethwallet/util"
)

var doneERC20, doneBalanceTickerERC20 chan bool
var currencyDefinitionERC20 wi.CurrencyDefinition
var logERC = logging.MustGetLogger("ercwallet")

// ERC20Wallet is the wallet implementation for ethereum
type ERC20Wallet struct {
	client               *EthClient
	account              *Account
	address              *EthAddress
	service              *Service
	registry             *Registry
	ppsct                *Escrow
	db                   wi.Datastore
	exchangeRates        wi.ExchangeRates
	params               *chaincfg.Params
	symbol               string
	name                 string
	deployAddressMain    common.Address
	deployAddressRopsten common.Address
	deployAddressRinkeby common.Address
	currentdeployAddress common.Address
	token                *Token
	listeners            []func(wi.TransactionCallback)
}

// GenTokenScriptHash - used to generate script hash for erc20 token as per
// escrow smart contract
func GenTokenScriptHash(script EthRedeemScript) ([32]byte, string, error) {
	a := make([]byte, 4)
	binary.BigEndian.PutUint32(a, script.Timeout)
	arr := append(script.TxnID.Bytes(), append([]byte{script.Threshold},
		append(a[:], append(script.Buyer.Bytes(),
			append(script.Seller.Bytes(), append(script.Moderator.Bytes(),
				append(script.MultisigAddress.Bytes(),
					script.TokenAddress.Bytes()...)...)...)...)...)...)...)
	var retHash [32]byte

	copy(retHash[:], crypto.Keccak256(arr)[:])
	ahashStr := hexutil.Encode(retHash[:])

	return retHash, ahashStr, nil
}

// TokenDetail is used to capture ERC20 token details
type TokenDetail struct {
	name                 string
	symbol               string
	deployAddressMain    common.Address
	deployAddressRopsten common.Address
	deployAddressRinkeby common.Address
	currentdeployAddress common.Address
}

// NewERC20Wallet will return a reference to the ERC20 Wallet
func NewERC20Wallet(cfg config.CoinConfig, params *chaincfg.Params, mnemonic string, proxy proxy.Dialer) (*ERC20Wallet, error) {
	client, err := NewEthClient(cfg.ClientAPIs[0] + "/" + InfuraAPIKey)
	if err != nil {
		logERC.Errorf("error initializing wallet: %v", err)
		return nil, err
	}
	var myAccount *Account
	myAccount, err = NewAccountFromMnemonic(mnemonic, "", params)
	if err != nil {
		logERC.Errorf("mnemonic based pk generation failed: %s", err.Error())
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
		logERC.Errorf("ethereum registry not found: %s", cfg.Options[registryKey])
		return nil, err
	}

	ethConfig.RegistryAddress = regAddr.(string)

	reg, err := NewRegistry(common.HexToAddress(ethConfig.RegistryAddress), client)
	if err != nil {
		logERC.Errorf("error initilaizing contract failed: %s", err.Error())
		return nil, err
	}
	er := NewEthereumPriceFetcher(proxy)

	token := TokenDetail{}

	var name, symbol, deployAddrMain, deployAddrRopsten, deployAddrRinkeby interface{}
	if name, ok = cfg.Options["Name"]; !ok {
		logERC.Errorf("erc20 token name not found: %s", cfg.Options["Name"])
		return nil, err
	}

	token.name = name.(string)

	if symbol, ok = cfg.Options["Symbol"]; !ok {
		logERC.Errorf("erc20 token symbol not found: %s", cfg.Options["Symbol"])
		return nil, err
	}

	token.symbol = symbol.(string)

	if deployAddrMain, ok = cfg.Options["MainNetAddress"]; !ok {
		logERC.Errorf("erc20 token address not found: %s", cfg.Options["MainNetAddress"])
		return nil, err
	}

	token.deployAddressMain = common.HexToAddress(deployAddrMain.(string))

	if deployAddrRopsten, ok = cfg.Options["RopstenAddress"]; ok {
		token.deployAddressRopsten = common.HexToAddress(deployAddrRopsten.(string))
	}

	if deployAddrRinkeby, ok = cfg.Options["RinkebyAddress"]; ok {
		token.deployAddressRinkeby = common.HexToAddress(deployAddrRinkeby.(string))
	}

	token.currentdeployAddress = token.deployAddressMain
	if strings.Contains(cfg.ClientAPIs[0], "rinkeby") {
		token.currentdeployAddress = token.deployAddressRinkeby
	} else if strings.Contains(cfg.ClientAPIs[0], "ropsten") {
		token.currentdeployAddress = token.deployAddressRopsten
	}
	erc20Token, err := NewToken(token.currentdeployAddress, client)
	if err != nil {
		logERC.Errorf("error initilaizing erc20 token failed: %s", err.Error())
		return nil, err
	}

	currencyDefinitionERC20 = wi.CurrencyDefinition{
		Code:         token.symbol,
		Divisibility: 18,
	}

	return &ERC20Wallet{
		client:               client,
		account:              myAccount,
		address:              &EthAddress{&addr},
		service:              &Service{},
		registry:             reg,
		ppsct:                nil,
		db:                   cfg.DB,
		exchangeRates:        er,
		params:               params,
		symbol:               token.symbol,
		name:                 token.name,
		deployAddressMain:    token.deployAddressMain,
		deployAddressRopsten: token.deployAddressRopsten,
		deployAddressRinkeby: token.deployAddressRinkeby,
		currentdeployAddress: token.currentdeployAddress,
		token:                erc20Token,
		listeners:            []func(wi.TransactionCallback){},
	}, nil
}

// Params - return nil to comply
func (wallet *ERC20Wallet) Params() *chaincfg.Params {
	return wallet.params
}

// GetBalance returns the balance for the wallet
func (wallet *ERC20Wallet) GetBalance() (*big.Int, error) {
	return wallet.client.GetTokenBalance(wallet.account.Address(), wallet.currentdeployAddress)
}

// GetUnconfirmedBalance returns the unconfirmed balance for the wallet
func (wallet *ERC20Wallet) GetUnconfirmedBalance() (*big.Int, error) {
	return big.NewInt(0), nil
}

// Transfer will transfer the amount from this wallet to the spec address
func (wallet *ERC20Wallet) Transfer(to string, value *big.Int, spendAll bool, fee big.Int) (common.Hash, error) {
	toAddress := common.HexToAddress(to)
	return wallet.client.TransferToken(wallet.account, toAddress, wallet.currentdeployAddress, value, spendAll, fee)
}

// Start will start the wallet daemon
func (wallet *ERC20Wallet) Start() {
	doneERC20 = make(chan bool)
	doneBalanceTickerERC20 = make(chan bool)
	// start the ticker to check for pending txn rcpts
	go func(wallet *ERC20Wallet) {
		logERC.Info("txn check ticker .....")
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				logERC.Info("tick  tick .....")
				// get the pending txns
				txns, err := wallet.db.Txns().GetAll(true)
				logERC.Info("do we have txns .... ", txns, "    ", err)
				if err != nil {
					continue
				}
				for _, txn := range txns {
					logERC.Info("lets qqqqqqqqqqqq ", txn.Txid)
					hash := common.HexToHash(txn.Txid)
					go func(txnData []byte) {
						_, err := wallet.checkTxnRcpt(&hash, txnData)
						if err != nil {
							logERC.Errorf(err.Error())
						}
					}(txn.Bytes)
				}
			}
		}
	}(wallet)

	// start the ticker to check for balance
	go func(wallet *ERC20Wallet) {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		currentBalance, err := wallet.GetBalance()
		if err != nil {
			logERC.Infof("err fetching initial balance: %v", err)
		}
		currentTip, _ := wallet.ChainTip()

		for {
			select {
			case <-doneBalanceTickerERC20:
				return
			case <-ticker.C:
				// fetch the current balance
				fetchedBalance, err := wallet.GetBalance()
				if err != nil {
					logERC.Infof("err fetching balance at %v: %v", time.Now(), err)
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

func (wallet *ERC20Wallet) processBalanceChange(previousBalance, currentBalance *big.Int, currentHead uint32) {
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

func (wallet *ERC20Wallet) invokeTxnCB(txnID string, value *big.Int) {
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

// CurrencyCode returns symbol
func (wallet *ERC20Wallet) CurrencyCode() string {
	if wallet.params == nil {
		return wallet.symbol
	}
	if wallet.params.Name == chaincfg.MainNetParams.Name {
		return wallet.symbol
	}
	return "T" + wallet.symbol
}

// IsDust Check if this amount is considered dust - 10000 wei
func (wallet *ERC20Wallet) IsDust(amount big.Int) bool {
	return amount.Cmp(big.NewInt(10000)) <= 0
}

// MasterPrivateKey - Get the master private key
func (wallet *ERC20Wallet) MasterPrivateKey() *hd.ExtendedKey {
	return hd.NewExtendedKey([]byte{0x00, 0x00, 0x00, 0x00}, wallet.account.privateKey.D.Bytes(),
		wallet.account.address.Bytes(), wallet.account.address.Bytes(), 0, 0, true)
}

// MasterPublicKey - Get the master public key
func (wallet *ERC20Wallet) MasterPublicKey() *hd.ExtendedKey {
	publicKey := wallet.account.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		logERC.Fatal("error casting public key to ECDSA")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	return hd.NewExtendedKey([]byte{0x00, 0x00, 0x00, 0x00}, publicKeyBytes,
		wallet.account.address.Bytes(), wallet.account.address.Bytes(), 0, 0, false)
}

// ChildKey Generate a child key using the given chaincode. The key is used in multisig transactions.
// For most implementations this should just be child key 0.
func (wallet *ERC20Wallet) ChildKey(keyBytes []byte, chaincode []byte, isPrivateKey bool) (*hd.ExtendedKey, error) {
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
func (wallet *ERC20Wallet) CurrentAddress(purpose wi.KeyPurpose) btcutil.Address {
	return *wallet.address
}

// NewAddress - Returns a fresh address that has never been returned by this function
func (wallet *ERC20Wallet) NewAddress(purpose wi.KeyPurpose) btcutil.Address {
	return *wallet.address
}

func ethTokenScriptToAddr(addr string) (common.Address, error) {
	rScriptBytes, err := hex.DecodeString(addr)
	if err != nil {
		return common.Address{}, err
	}
	rScript, err := DeserializeEthScript(rScriptBytes)
	if err != nil {
		return common.Address{}, err
	}
	_, sHash, err := GenTokenScriptHash(rScript)
	if err != nil {
		return common.Address{}, err
	}
	return common.HexToAddress(sHash), nil
}

// DecodeAddress - Parse the address string and return an address interface
func (wallet *ERC20Wallet) DecodeAddress(addr string) (btcutil.Address, error) {
	var (
		ethAddr common.Address
		err     error
	)
	if len(addr) > 64 {
		ethAddr, err = ethTokenScriptToAddr(addr)
		if err != nil {
			logERC.Error(err.Error())
		}
	} else {
		ethAddr = common.HexToAddress(addr)
	}

	return EthAddress{&ethAddr}, err
}

// ScriptToAddress - ?
func (wallet *ERC20Wallet) ScriptToAddress(script []byte) (btcutil.Address, error) {
	return wallet.address, nil
}

// HasKey - Returns if the wallet has the key for the given address
func (wallet *ERC20Wallet) HasKey(addr btcutil.Address) bool {
	if !util.IsValidAddress(addr.String()) {
		return false
	}
	return wallet.account.Address().String() == addr.String()
}

// Balance - Get the confirmed and unconfirmed balances
func (wallet *ERC20Wallet) Balance() (confirmed, unconfirmed wi.CurrencyValue) {
	var balance, ucbalance wi.CurrencyValue
	bal, err := wallet.GetBalance()
	if err == nil {
		balance = wi.CurrencyValue{
			Value:    *bal,
			Currency: currencyDefinitionERC20,
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

/*
func (wallet *ERC20Wallet) Balance() (confirmed, unconfirmed int64) {
	var balance, ucbalance int64
	bal, err := wallet.GetBalance()
	if err == nil {
		balance = bal.Int64()
	}
	ucbal, err := wallet.GetUnconfirmedBalance()
	if err == nil {
		ucbalance = ucbal.Int64()
	}
	ucb := int64(0)
	if ucbalance > balance {
		ucb = ucbalance - balance
	}
	return balance, ucb
}
*/

// TransactionsFromBlock - Returns a list of transactions for this wallet begining from the specified block
func (wallet *ERC20Wallet) TransactionsFromBlock(startBlock *int) ([]wi.Txn, error) {
	txns, err := wallet.client.eClient.NormalTxByAddress(util.EnsureCorrectPrefix(wallet.account.Address().String()), startBlock, nil,
		1, 0, false)
	if err != nil {
		logERC.Error("err fetching transactions : ", err)
		return []wi.Txn{}, nil
	}

	ret := []wi.Txn{}
	for _, t := range txns {
		status := wi.StatusConfirmed
		prefix := ""
		if t.IsError != 0 {
			status = wi.StatusError
		}
		if strings.ToLower(t.From) == strings.ToLower(wallet.address.String()) {
			prefix = "-"
		}
		tnew := wi.Txn{
			Txid:          util.EnsureCorrectPrefix(t.Hash),
			Value:         prefix + t.Value.Int().String(),
			Height:        int32(t.BlockNumber),
			Timestamp:     t.TimeStamp.Time(),
			WatchOnly:     false,
			Confirmations: int64(t.Confirmations),
			Status:        wi.StatusCode(status),
			Bytes:         []byte(t.Input),
		}
		ret = append(ret, tnew)
	}

	return ret, nil
}

// Transactions - Returns a list of transactions for this wallet
func (wallet *ERC20Wallet) Transactions() ([]wi.Txn, error) {
	return wallet.TransactionsFromBlock(nil)
}

/*
func (wallet *ERC20Wallet) Transactions() ([]wi.Txn, error) {
	txns, err := wallet.client.eClient.NormalTxByAddress(wallet.account.Address().String(), nil, nil,
		1, 0, true)
	if err != nil {
		return []wi.Txn{}, err
	}

	ret := []wi.Txn{}
	for _, t := range txns {
		status := wi.StatusConfirmed
		if t.IsError != 0 {
			status = wi.StatusError
		}
		if t.Confirmations > 0 && t.Confirmations < 300 {
			status = wi.StatusPending
		}
		tnew := wi.Txn{
			Txid:          t.Hash,
			Value:         t.Value.Int().String(),
			Height:        int32(t.BlockNumber),
			Timestamp:     t.TimeStamp.Time(),
			WatchOnly:     false,
			Confirmations: int64(t.Confirmations),
			Status:        wi.StatusCode(status),
			Bytes:         []byte(t.Input),
		}
		ret = append(ret, tnew)
	}

	return ret, nil
}
*/

// GetTransaction - Get info on a specific transaction
func (wallet *ERC20Wallet) GetTransaction(txid chainhash.Hash) (wi.Txn, error) {
	// tx, _, err := wallet.client.GetTransaction(common.HexToHash(txid.String()))
	// if err != nil {
	// 	return wi.Txn{}, err
	// }
	// return wi.Txn{
	// 	Txid:      tx.Hash().String(),
	// 	Value:     tx.Value().String(),
	// 	Height:    0,
	// 	Timestamp: time.Now(),
	// 	WatchOnly: false,
	// 	Bytes:     tx.Data(),
	// }, nil
	logERC.Info("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
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

	logERC.Info("AAAAAAAAAa111111111111")

	//value := tx.Value().String()
	fromAddr := msg.From()
	toAddr := msg.To()
	valueSub := big.NewInt(5000000)
	value := tx.Value()

	logERC.Info(hexutil.Encode(msg.Data()))

	if strings.HasPrefix(hexutil.Encode(msg.Data()), "0xa9059cbb") {
		value = big.NewInt(0).SetBytes(msg.Data()[36:])
	} else if strings.HasPrefix(hexutil.Encode(msg.Data()), "0x57bced76") {
		b := msg.Data()
		value = big.NewInt(0).SetBytes(b[len(b)-(3*32) : len(b)-(2*32)])
		valueSub = value
	}

	logERC.Info(value.String())

	if tx.To().String() == wallet.currentdeployAddress.String() {
		toAddr = wallet.address.address
		valueSub = value
	} else {
		v, err := wallet.registry.GetRecommendedVersion(nil, "escrow")
		if err == nil {
			if tx.To().String() == v.Implementation.String() {
				toAddr = wallet.address.address
			}
			if msg.Value().Cmp(valueSub) > 0 {
				valueSub = msg.Value()
			}
		}
	}

	return wi.Txn{
		Txid:        util.EnsureCorrectPrefix(tx.Hash().Hex()),
		Value:       value.String(),
		Height:      0,
		Timestamp:   time.Now(),
		WatchOnly:   false,
		Bytes:       tx.Data(),
		ToAddress:   util.EnsureCorrectPrefix(toAddr.String()),
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
func (wallet *ERC20Wallet) ChainTip() (uint32, chainhash.Hash) {
	num, hash, err := wallet.client.GetLatestBlock()
	h, _ := chainhash.NewHashFromStr("")
	if err != nil {
		return 0, *h
	}
	h, _ = chainhash.NewHashFromStr(hash.Hex()[2:])
	return num, *h
}

// GetFeePerByte - Get the current fee per byte
func (wallet *ERC20Wallet) GetFeePerByte(feeLevel wi.FeeLevel) big.Int {
	est, err := wallet.client.GetEthGasStationEstimate()
	ret := big.NewInt(0)
	if err != nil {
		logERC.Errorf("err fetching ethgas station data: %v", err)
		return *ret
	}
	switch feeLevel {
	case wi.NORMAL:
		ret, _ = big.NewFloat(est.Average * 100000000).Int(nil)
	case wi.ECONOMIC:
		ret, _ = big.NewFloat(est.SafeLow * 100000000).Int(nil)
	case wi.PRIOIRTY, wi.FEE_BUMP:
		ret, _ = big.NewFloat(est.Fast * 100000000).Int(nil)
	}
	return *ret
}

// Spend - Send ether to an external wallet
func (wallet *ERC20Wallet) Spend(amount big.Int, addr btcutil.Address, feeLevel wi.FeeLevel, referenceID string, spendAll bool) (*chainhash.Hash, error) {
	logERC.Info(",,, in erc20 spend ..... ", amount.String(), "  ", addr.String(), referenceID)
	var hash common.Hash
	var h *chainhash.Hash
	var err error
	actualRecipient := addr

	if referenceID == "" {
		// no referenceID means this is a direct transfer
		hash, err = wallet.Transfer(util.EnsureCorrectPrefix(addr.String()), &amount, spendAll, wallet.GetFeePerByte(feeLevel))
	} else {
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

		logERC.Info("if this a moderated or disputed order, then it is a script address .... ")
		for _, script := range scripts {
			if bytes.Equal(key, script[:common.AddressLength]) {
				isScript = true
				redeemScript = script[common.AddressLength:]
				break
			}
		}

		logERC.Info("is this a script : ", isScript)

		if isScript {
			ethScript, err := DeserializeEthScript(redeemScript)
			if err != nil {
				return nil, err
			}
			_, scrHash, err := GenTokenScriptHash(ethScript)
			if err != nil {
				logERC.Error(err.Error())
			}
			addrScrHash := common.HexToAddress(scrHash)
			logERC.Info("addrScrHash  : ", addrScrHash.String())
			actualRecipient = EthAddress{address: &addrScrHash} //EthAddress{address: &ethScript.Seller} //EthAddress{address: &addrScrHash}
			hash, _, err = wallet.callAddTokenTransaction(ethScript, &amount, feeLevel)
			if err != nil {
				logERC.Errorf("error call add txn: %v", err)
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

		logERC.Info("we have the hash here : ", hash.String())
		// txn is pending
		nonce, err := wallet.client.GetTxnNonce(util.EnsureCorrectPrefix(hash.Hex()))
		logERC.Info("after get nonce  ", nonce, "    ", err)
		if err == nil {
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
				err0 := wallet.db.Txns().Put(data, ut.NormalizeAddress(hash.Hex()), "0", 0, time.Now(), true)
				if err0 != nil {
					logERC.Error(err0.Error())
				}
			}
		}
	}

	logERC.Info("almost near  ... ", err)

	if err == nil {
		h, err = util.CreateChainHash(hash.Hex())
		if err == nil {
			wallet.invokeTxnCB(h.String(), &amount)
		}
	}
	logERC.Info("before return : ", h, "    ", err)
	return h, err
}

/*
func (wallet *ERC20Wallet) Spend(amount big.Int, addr btcutil.Address, feeLevel wi.FeeLevel, referenceID string) (*chainhash.Hash, error) {
	var hash common.Hash
	var h *chainhash.Hash
	var err error

	// check if the addr is a multisig addr
	scripts, err := wallet.db.WatchedScripts().GetAll()
	if err != nil {
		return nil, err
	}
	isScript := false
	key := []byte(addr.String())
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
		hash, err = wallet.callAddTokenTransaction(ethScript, &amount)
		if err != nil {
			logERC.Errorf("error call add token txn: %v", err)
		}
	} else {
		hash, err = wallet.Transfer(addr.String(), &amount)
	}

	if err != nil {
		return nil, err
	}
	start := time.Now()
	flag := false
	var rcpt *types.Receipt
	for !flag {
		rcpt, err = wallet.client.TransactionReceipt(context.Background(), hash)
		if rcpt != nil {
			flag = true
		}
		if time.Since(start).Seconds() > 120 {
			flag = true
		}
	}
	if rcpt != nil {
		// good. so the txn has been processed but we have to account for failed
		// but valid txn like some contract condition causing revert
		if rcpt.Status > 0 {
			// all good to update order state
			go wallet.callListeners(wallet.createTxnCallback(hash.String(), referenceID, addr, amount, time.Now()))
		} else {
			// there was some error processing this txn
			return nil, errors.New("problem processing this transaction")
		}
	}

	if err == nil {
		h, err = chainhash.NewHashFromStr(hash.Hex()[2:])
	}
	return h, err
}
*/

/*
func (wallet *ERC20Wallet) createTxnCallback(txID, orderID string, toAddress btcutil.Address, value big.Int, bTime time.Time) wi.TransactionCallback {
	output := wi.TransactionOutput{
		Address: toAddress,
		Value:   value,
		Index:   1,
		OrderID: orderID,
	}

	input := wi.TransactionInput{
		OutpointHash:  []byte(txID),
		OutpointIndex: 1,
		LinkedAddress: wallet.address,
		Value:         value,
		OrderID:       orderID,
	}

	return wi.TransactionCallback{
		Txid:      txID[2:],
		Outputs:   []wi.TransactionOutput{output},
		Inputs:    []wi.TransactionInput{input},
		Height:    1,
		Timestamp: time.Now(),
		Value:     value,
		WatchOnly: false,
		BlockTime: bTime,
	}
}
*/

func (wallet *ERC20Wallet) createTxnCallback(txID, orderID string, toAddress btcutil.Address, value big.Int, bTime time.Time, withInput bool) wi.TransactionCallback {
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
		Height:    1,
		Timestamp: time.Now(),
		Value:     value,
		WatchOnly: false,
		BlockTime: bTime,
	}
}

func (wallet *ERC20Wallet) callListeners(txnCB wi.TransactionCallback) {
	for _, l := range wallet.listeners {
		go l(txnCB)
	}
}

// AssociateTransactionWithOrder is used to transition order state
func (wallet *ERC20Wallet) AssociateTransactionWithOrder(txnCB wi.TransactionCallback) {
	for _, l := range wallet.listeners {
		go l(txnCB)
	}
}

// checkTxnRcpt check the txn rcpt status
func (wallet *ERC20Wallet) checkTxnRcpt(hash *common.Hash, data []byte) (*common.Hash, error) {
	logERC.Info("in chk txn rcpt .... ", hash.String())
	var rcpt *types.Receipt
	pTxn, err := DeserializePendingTxn(data)
	logERC.Info("after deserializing data   : ", err)
	if err != nil {
		return nil, err
	}

	rcpt, err = wallet.client.TransactionReceipt(context.Background(), *hash)
	logERC.Info("after TransactionReceipt  ", rcpt, "    ", err)
	if err != nil {
		logERC.Infof("fetching txn rcpt: %v", err)
	}

	if rcpt != nil {
		logERC.Info("rcpt is not nil .... status : ", rcpt.Status)
		// good. so the txn has been processed but we have to account for failed
		// but valid txn like some contract condition causing revert
		if rcpt.Status > 0 {
			// all good to update order state
			chash, err := util.CreateChainHash((*hash).Hex())
			logERC.Info("after create chain hash  : ", chash.String(), "   ", err)
			if err != nil {
				return nil, err
			}
			err = wallet.db.Txns().Delete(chash)
			if err != nil {
				logERC.Errorf("err deleting the pending txn : %v", err)
			}
			logERC.Info("amount  : ", pTxn.Amount)
			n := new(big.Int)
			n, _ = n.SetString(pTxn.Amount, 10)
			toAddr := common.HexToAddress(pTxn.To)
			withInput := true
			if pTxn.Amount != "0" {
				toAddr = common.HexToAddress(util.EnsureCorrectPrefix(pTxn.To))
				withInput = pTxn.WithInput
			}
			go wallet.AssociateTransactionWithOrder(
				wallet.createTxnCallback(util.EnsureCorrectPrefix(hash.Hex()), pTxn.OrderID, EthAddress{&toAddr},
					*n, time.Now(), withInput))
		}
	}
	return hash, nil

}

// BumpFee - Bump the fee for the given transaction
func (wallet *ERC20Wallet) BumpFee(txid chainhash.Hash) (*chainhash.Hash, error) {
	return chainhash.NewHashFromStr(txid.String())
}

// EstimateFee - Calculates the estimated size of the transaction and returns the total fee for the given feePerByte
func (wallet *ERC20Wallet) EstimateFee(ins []wi.TransactionInput, outs []wi.TransactionOutput, feePerByte big.Int) big.Int {
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

// EstimateSpendFee - Build a spend transaction for the amount and return the transaction fee
func (wallet *ERC20Wallet) EstimateSpendFee(amount big.Int, feeLevel wi.FeeLevel) (big.Int, error) {
	if !wallet.balanceCheck(feeLevel, amount) {
		return *big.NewInt(0), wi.ErrInsufficientFunds
	}
	gas, err := wallet.client.EstimateGasSpend(wallet.account.Address(), &amount)
	return *gas, err
}

func (wallet *ERC20Wallet) balanceCheck(feeLevel wi.FeeLevel, amount big.Int) bool {
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
		logERC.Error("err fetching erc20 wallet balance")
		currentBalance = big.NewInt(0)
	}
	if requiredBalance.Cmp(currentBalance) > 0 {
		// the wallet does not have the required balance
		return false
	}
	return true
}

// SweepAddress - Build and broadcast a transaction that sweeps all coins from an address. If it is a p2sh multisig, the redeemScript must be included
func (wallet *ERC20Wallet) SweepAddress(utxos []wi.TransactionInput, address *btcutil.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wi.FeeLevel) (*chainhash.Hash, error) {
	return chainhash.NewHashFromStr("")
}

// ExchangeRates - return the exchangerates
func (wallet *ERC20Wallet) ExchangeRates() wi.ExchangeRates {
	return wallet.exchangeRates
}

func (wallet *ERC20Wallet) callAddTokenTransaction(script EthRedeemScript, value *big.Int, feeLevel wi.FeeLevel) (common.Hash, uint64, error) {

	h := common.BigToHash(big.NewInt(0))

	// call registry to get the deployed address for the escrow ct
	fromAddress := wallet.account.Address()
	nonce, err := wallet.client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		logERC.Fatal(err)
	}
	gasPrice, err := wallet.client.SuggestGasPrice(context.Background())
	if err != nil {
		logERC.Fatal(err)
	}
	auth := bind.NewKeyedTransactor(wallet.account.privateKey)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0) // in wei
	auth.GasLimit = 4000000    // in units
	auth.GasPrice = gasPrice

	// lets check if the caller has enough balance to make the
	// multisign call
	if !wallet.balanceCheck(feeLevel, *big.NewInt(0)) {
		// the wallet does not have the required balance
		return h, nonce, wi.ErrInsufficientFunds
	}

	shash, _, err := GenTokenScriptHash(script)
	if err != nil {
		return h, 0, err
	}

	smtct, err := NewEscrow(script.MultisigAddress, wallet.client)
	if err != nil {
		logERC.Fatalf("error initilaizing contract failed: %s", err.Error())
	}

	var tx *types.Transaction

	tx, err = wallet.token.Approve(auth, script.MultisigAddress, value)

	if err != nil {
		return common.BigToHash(big.NewInt(0)), 0, err
	}

	//time.Sleep(2 * time.Minute)
	header, err := wallet.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		logERC.Errorf("error fetching latest blk: %v", err)
	}
	tclient, err := ethclient.Dial(wallet.client.wsurl)
	if err != nil {
		logERC.Errorf("error establishing ws conn: %v", err)
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{script.TokenAddress},
		FromBlock: header.Number,
		Topics:    [][]common.Hash{{common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925")}},
	}
	logs := make(chan types.Log)
	sub1, err := tclient.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		return common.BigToHash(big.NewInt(0)), 0, err
	}
	defer sub1.Unsubscribe()
	flag := false
	for !flag {
		select {
		case err := <-sub1.Err():
			logERC.Fatal(err)
		case vLog := <-logs:
			//logERC.Info(vLog) // pointer to event log
			//spew.Dump(vLog)
			//logERC.Info(vLog.Topics[0])
			logERC.Info(vLog.Address.String())
			if tx.Hash() == vLog.TxHash {
				logERC.Info("we have found the approval ...")
				//time.Sleep(2 * time.Minute)
				spew.Dump(vLog)
				nonce, _ = wallet.client.PendingNonceAt(context.Background(), fromAddress)
				auth.Nonce = big.NewInt(int64(nonce))
				auth.Value = big.NewInt(0) // in wei
				auth.GasLimit = 4000000    // in units
				auth.GasPrice = gasPrice

				tx, err = smtct.AddTokenTransaction(auth, script.Buyer, script.Seller,
					script.Moderator, script.Threshold, script.Timeout, shash,
					value, script.TxnID, wallet.currentdeployAddress)

				if err == nil {
					h = tx.Hash()
				}
				flag = true
				break
			}
		}
	}

	/*
		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = value      // in wei
		auth.GasLimit = 4000000 // in units
		auth.GasPrice = gasPrice

		tx, err = smtct.AddTokenTransaction(auth, script.Buyer, script.Seller,
			script.Moderator, script.Threshold, script.Timeout, shash,
			value, script.TxnID, wallet.currentDeployAddress)

		if err == nil {
			h = tx.Hash()
		}
	*/

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
func (wallet *ERC20Wallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (btcutil.Address, []byte, error) {
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
		logERC.Fatal(err)
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
	builder.TokenAddress = wallet.currentdeployAddress

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
		logERC.Errorf("err saving the redeemscript: %v", err)
	}

	return retAddr, redeemScript, nil
}

// CreateMultisigSignature - Create a signature for a multisig transaction
func (wallet *ERC20Wallet) CreateMultisigSignature(ins []wi.TransactionInput, outs []wi.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte big.Int) ([]wi.Signature, error) {

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

	shash, _, err := GenTokenScriptHash(rScript)
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
	logERC.Debugf("txnHash        : %s", hexutil.Encode(txnHash))
	logERC.Debugf("phash          : %s", hexutil.Encode(payloadHash[:]))
	copy(txHash[:], txnHash)

	sig, err := crypto.Sign(txHash[:], wallet.account.privateKey)
	if err != nil {
		logERC.Errorf("error signing in createmultisig : %v", err)
	}
	sigs = append(sigs, wi.Signature{InputIndex: 1, Signature: sig})

	return sigs, err
}

// Multisign - Combine signatures and optionally broadcast
func (wallet *ERC20Wallet) Multisign(ins []wi.TransactionInput, outs []wi.TransactionOutput, sigs1 []wi.Signature, sigs2 []wi.Signature, redeemScript []byte, feePerByte big.Int, broadcast bool) ([]byte, error) {

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

	shash, _, err := GenTokenScriptHash(rScript)
	if err != nil {
		return nil, err
	}

	smtct, err := NewEscrow(rScript.MultisigAddress, wallet.client)
	if err != nil {
		logERC.Fatalf("error initializing contract failed: %s", err.Error())
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
		logERC.Fatal(err)
	}
	gasPrice, err := wallet.client.SuggestGasPrice(context.Background())
	if err != nil {
		logERC.Fatal(err)
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
		logERC.Error("err fetching eth wallet balance")
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
	_, scrHash, err := GenTokenScriptHash(rScript)
	if err != nil {
		logERC.Error(err.Error())
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
			logERC.Error(err0.Error())
		}
	}

	return tx.Hash().Bytes(), nil
}

// AddWatchedAddresses - Add a script to the wallet and get notifications back when coins are received or spent from it
func (wallet *ERC20Wallet) AddWatchedAddresses(addrs ...btcutil.Address) error {
	// the reason eth wallet cannot use this as of now is because only the address
	// is insufficient, the redeemScript is also required
	return nil
}

/*
// GenerateMultisigScript - Generate a multisig script from public keys. If a timeout is included the returned script should be a timelocked escrow which releases using the timeoutKey.
func (wallet *ERC20Wallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (btcutil.Address, []byte, error) {
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
		ecKeys = append(ecKeys, common.BytesToAddress(ecKey.SerializeUncompressed()))
	}

	ver, err := wallet.registry.GetRecommendedVersion(nil, "escrow")
	if err != nil {
		logERC.Fatal(err)
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
	builder.TokenAddress = wallet.currentDeployAddress

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

	//hash := sha3.NewKeccak256()
	//hash.Write(redeemScript)
	addr := common.HexToAddress(hexutil.Encode(crypto.Keccak256(redeemScript)))
	retAddr := EthAddress{&addr}

	scriptKey := append(addr.Bytes(), redeemScript...)
	wallet.db.WatchedScripts().Put(scriptKey)

	return retAddr, redeemScript, nil
}
*/

/*
// CreateMultisigSignature - Create a signature for a multisig transaction
func (wallet *ERC20Wallet) CreateMultisigSignature(ins []wi.TransactionInput, outs []wi.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]wi.Signature, error) {

	var sigs []wi.Signature

	payables := make(map[string]*big.Int)
	for _, out := range outs {
		if out.Value.Cmp(big.NewInt(0)) <= 0 {
			continue
		}
		val := &out.Value
		if p, ok := payables[out.Address.String()]; ok {
			sum := big.NewInt(0)
			sum.Add(val, p)
			payables[out.Address.String()] = sum
		} else {
			payables[out.Address.String()] = val
		}
	}

	destArr := []byte{}
	amountArr := []byte{}

	for k, v := range payables {
		addr := common.HexToAddress(k)
		sample := [32]byte{}
		sampleDest := [32]byte{}
		copy(sampleDest[12:], addr.Bytes())
		a := make([]byte, 8)
		binary.BigEndian.PutUint64(a, v.Uint64())

		copy(sample[24:], a)
		destArr = append(destArr, sampleDest[:]...)
		amountArr = append(amountArr, sample[:]...)
	}

	rScript, err := DeserializeEthScript(redeemScript)
	if err != nil {
		return nil, err
	}

	shash, _, err := GenTokenScriptHash(rScript)
	if err != nil {
		return nil, err
	}

	var txHash [32]byte
	var payloadHash [32]byte


				// // Follows ERC191 signature scheme: https://github.com/ethereum/EIPs/issues/191
		        // bytes32 txHash = keccak256(
		        //     abi.encodePacked(
		        //         "\x19Ethereum Signed Message:\n32",
		        //         keccak256(
		        //             abi.encodePacked(
		        //                 byte(0x19),
		        //                 byte(0),
		        //                 this,
		        //                 destinations,
		        //                 amounts,
		        //                 scriptHash
		        //             )
		        //         )
		        //     )
		        // );



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
	copy(txHash[:], txnHash)

	sig, err := crypto.Sign(txHash[:], wallet.account.privateKey)
	if err != nil {
		logERC.Errorf("error signing in createmultisig : %v", err)
	}
	sigs = append(sigs, wi.Signature{InputIndex: 1, Signature: sig})

	return sigs, err
}
*/

/*
// Multisign - Combine signatures and optionally broadcast
func (wallet *ERC20Wallet) Multisign(ins []wi.TransactionInput, outs []wi.TransactionOutput, sigs1 []wi.Signature, sigs2 []wi.Signature, redeemScript []byte, feePerByte uint64, broadcast bool) ([]byte, error) {

	//var buf bytes.Buffer

	payables := make(map[string]*big.Int)
	for _, out := range outs {
		if out.Value.Cmp(big.NewInt(0)) <= 0 {
			continue
		}
		val := &out.Value
		if p, ok := payables[out.Address.String()]; ok {
			sum := big.NewInt(0)
			sum.Add(val, p)
			payables[out.Address.String()] = sum
		} else {
			payables[out.Address.String()] = val
		}
	}

	rSlice := [][32]byte{} //, 2)
	sSlice := [][32]byte{} //, 2)
	vSlice := []uint8{}    //, 2)

	var r [32]byte
	var s [32]byte
	var v uint8

	if len(sigs1[0].Signature) > 0 {
		r, s, v = util.SigRSV(sigs1[0].Signature)
		rSlice = append(rSlice, r)
		sSlice = append(sSlice, s)
		vSlice = append(vSlice, v)
	}

	//r = [32]byte{}
	//s = [32]byte{}
	//v = uint8(0)

	if len(sigs2[0].Signature) > 0 {
		r, s, v = util.SigRSV(sigs2[0].Signature)
		rSlice = append(rSlice, r)
		sSlice = append(sSlice, s)
		vSlice = append(vSlice, v)
	}

	rScript, err := DeserializeEthScript(redeemScript)
	if err != nil {
		return nil, err
	}

	shash, _, err := GenTokenScriptHash(rScript)
	if err != nil {
		return nil, err
	}

	smtct, err := NewEscrow(rScript.MultisigAddress, wallet.client)
	if err != nil {
		logERC.Fatalf("error initilaizing contract failed: %s", err.Error())
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
		logERC.Fatal(err)
	}
	gasPrice, err := wallet.client.SuggestGasPrice(context.Background())
	if err != nil {
		logERC.Fatal(err)
	}
	auth := bind.NewKeyedTransactor(wallet.account.privateKey)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0) // in wei
	auth.GasLimit = 4000000    // in units
	auth.GasPrice = gasPrice

	var tx *types.Transaction

	tx, err = smtct.Execute(auth, vSlice, rSlice, sSlice, shash, destinations, amounts)

	//logERC.Info(tx)
	//logERC.Info(err)

	if err != nil {
		return nil, err
	}

	ret, err := tx.MarshalJSON()

	return ret, err
}
*/

// AddTransactionListener - add a txn listener
func (wallet *ERC20Wallet) AddTransactionListener(callback func(wi.TransactionCallback)) {
	// add incoming txn listener using service
	wallet.listeners = append(wallet.listeners, callback)
}

// ReSyncBlockchain - Use this to re-download merkle blocks in case of missed transactions
func (wallet *ERC20Wallet) ReSyncBlockchain(fromTime time.Time) {
	// use service here
}

// GetConfirmations - Return the number of confirmations and the height for a transaction
func (wallet *ERC20Wallet) GetConfirmations(txid chainhash.Hash) (confirms, atHeight uint32, err error) {
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
func (wallet *ERC20Wallet) Close() {
	// stop the wallet daemon
	doneERC20 <- true
	doneBalanceTickerERC20 <- true
}

// CreateAddress - used to generate a new address
func (wallet *ERC20Wallet) CreateAddress() (common.Address, error) {
	fromAddress := wallet.account.Address()
	nonce, err := wallet.client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		logERC.Info(err)
	}
	addr := crypto.CreateAddress(fromAddress, nonce)
	//logERC.Info("Addr : ", addr.String())
	return addr, err
}
