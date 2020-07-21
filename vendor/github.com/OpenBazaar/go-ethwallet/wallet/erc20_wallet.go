package wallet

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/OpenBazaar/multiwallet/config"
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
	"golang.org/x/net/proxy"

	"github.com/OpenBazaar/go-ethwallet/util"
)

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
	symbol               string
	name                 string
	deployAddressMain    common.Address
	deployAddressRopsten common.Address
	deployAddressRinkeby common.Address
	token                *Token
	listeners            []func(wi.TransactionCallback)
}

// GenTokenScriptHash - used to generate script hash for erc20 token as per
// escrow smart contract
func GenTokenScriptHash(script EthRedeemScript) ([32]byte, string, error) {
	//ahash := sha3.NewKeccak256()
	a := make([]byte, 4)
	binary.BigEndian.PutUint32(a, script.Timeout)
	arr := append(script.TxnID.Bytes(), append([]byte{script.Threshold},
		append(a[:], append(script.Buyer.Bytes(),
			append(script.Seller.Bytes(), append(script.Moderator.Bytes(),
				append(script.MultisigAddress.Bytes(),
					script.TokenAddress.Bytes()...)...)...)...)...)...)...)
	//ahash.Write(arr)
	var retHash [32]byte

	//copy(retHash[:], ahash.Sum(nil)[:])
	copy(retHash[:], crypto.Keccak256(arr))
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
}

// NewERC20Wallet will return a reference to the ERC20 Wallet
func NewERC20Wallet(cfg config.CoinConfig, params *chaincfg.Params, mnemonic string, proxy proxy.Dialer) (*ERC20Wallet, error) {
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
	if regAddr, ok = cfg.Options["RegistryAddress"]; !ok {
		log.Errorf("ethereum registry not found: %s", cfg.Options["RegistryAddress"])
		return nil, err
	}

	ethConfig.RegistryAddress = regAddr.(string)

	reg, err := NewRegistry(common.HexToAddress(ethConfig.RegistryAddress), client)
	if err != nil {
		log.Errorf("error initilaizing contract failed: %s", err.Error())
		return nil, err
	}

	er := NewEthereumPriceFetcher(proxy)

	token := TokenDetail{}

	var name, symbol, deployAddrMain, deployAddrRopsten, deployAddrRinkeby interface{}
	if name, ok = cfg.Options["Name"]; !ok {
		log.Errorf("erc20 token name not found: %s", cfg.Options["Name"])
		return nil, err
	}

	token.name = name.(string)

	if symbol, ok = cfg.Options["Symbol"]; !ok {
		log.Errorf("erc20 token symbol not found: %s", cfg.Options["Symbol"])
		return nil, err
	}

	token.symbol = symbol.(string)

	if deployAddrMain, ok = cfg.Options["MainNetAddress"]; !ok {
		log.Errorf("erc20 token address not found: %s", cfg.Options["MainNetAddress"])
		return nil, err
	}

	token.deployAddressMain = common.HexToAddress(deployAddrMain.(string))

	if deployAddrRopsten, ok = cfg.Options["RopstenAddress"]; ok {
		token.deployAddressMain = common.HexToAddress(deployAddrRopsten.(string))
	}

	if deployAddrRinkeby, ok = cfg.Options["RinkebyAddress"]; ok {
		token.deployAddressRinkeby = common.HexToAddress(deployAddrRinkeby.(string))
	}

	erc20Token, err := NewToken(token.deployAddressMain, client)
	if err != nil {
		log.Errorf("error initilaizing erc20 token failed: %s", err.Error())
		return nil, err
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
		symbol:               token.symbol,
		name:                 token.name,
		deployAddressMain:    token.deployAddressMain,
		deployAddressRopsten: token.deployAddressRopsten,
		deployAddressRinkeby: token.deployAddressRinkeby,
		token:                erc20Token,
		listeners:            []func(wi.TransactionCallback){},
	}, nil
}

// Params - return nil to comply
func (wallet *ERC20Wallet) Params() *chaincfg.Params {
	return nil
}

// GetBalance returns the balance for the wallet
func (wallet *ERC20Wallet) GetBalance() (*big.Int, error) {
	return wallet.client.GetTokenBalance(wallet.account.Address(), wallet.deployAddressMain)
}

// GetUnconfirmedBalance returns the unconfirmed balance for the wallet
func (wallet *ERC20Wallet) GetUnconfirmedBalance() (*big.Int, error) {
	return big.NewInt(0), nil
}

// Transfer will transfer the amount from this wallet to the spec address
func (wallet *ERC20Wallet) Transfer(to string, value *big.Int) (common.Hash, error) {
	toAddress := common.HexToAddress(to)
	//wallet.token.Transfer()
	return wallet.client.TransferToken(wallet.account, toAddress, wallet.deployAddressMain, value)
}

// Start will start the wallet daemon
func (wallet *ERC20Wallet) Start() {
	// daemonize the wallet
}

// CurrencyCode returns symbol
func (wallet *ERC20Wallet) CurrencyCode() string {
	return wallet.symbol
}

// IsDust Check if this amount is considered dust - 10000 wei
func (wallet *ERC20Wallet) IsDust(amount int64) bool {
	return amount < 10000
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
		log.Fatal("error casting public key to ECDSA")
	}

	publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	return hd.NewExtendedKey([]byte{0x00, 0x00, 0x00, 0x00}, publicKeyBytes,
		wallet.account.address.Bytes(), wallet.account.address.Bytes(), 0, 0, false)
}

// ChildKey Generate a child key using the given chaincode. The key is used in multisig transactions.
// For most implementations this should just be child key 0.
func (wallet *ERC20Wallet) ChildKey(keyBytes []byte, chaincode []byte, isPrivateKey bool) (*hd.ExtendedKey, error) {
	if isPrivateKey {
		return wallet.MasterPrivateKey(), nil
	}
	return wallet.MasterPublicKey(), nil
}

// CurrentAddress - Get the current address for the given purpose
func (wallet *ERC20Wallet) CurrentAddress(purpose wi.KeyPurpose) btcutil.Address {
	return *wallet.address
}

// NewAddress - Returns a fresh address that has never been returned by this function
func (wallet *ERC20Wallet) NewAddress(purpose wi.KeyPurpose) btcutil.Address {
	return *wallet.address
}

// DecodeAddress - Parse the address string and return an address interface
func (wallet *ERC20Wallet) DecodeAddress(addr string) (btcutil.Address, error) {
	ethAddr := common.HexToAddress(addr)
	return EthAddress{&ethAddr}, nil
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

// Transactions - Returns a list of transactions for this wallet
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

// GetTransaction - Get info on a specific transaction
func (wallet *ERC20Wallet) GetTransaction(txid chainhash.Hash) (wi.Txn, error) {
	tx, _, err := wallet.client.GetTransaction(common.HexToHash(txid.String()))
	if err != nil {
		return wi.Txn{}, err
	}
	return wi.Txn{
		Txid:      tx.Hash().String(),
		Value:     tx.Value().String(),
		Height:    0,
		Timestamp: time.Now(),
		WatchOnly: false,
		Bytes:     tx.Data(),
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
func (wallet *ERC20Wallet) GetFeePerByte(feeLevel wi.FeeLevel) uint64 {
	return 0
}

// Spend - Send ether to an external wallet
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
			log.Errorf("error call add token txn: %v", err)
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

func (wallet *ERC20Wallet) callListeners(txnCB wi.TransactionCallback) {
	for _, l := range wallet.listeners {
		go l(txnCB)
	}
}

// BumpFee - Bump the fee for the given transaction
func (wallet *ERC20Wallet) BumpFee(txid chainhash.Hash) (string, error) {
	return txid.String(), nil
}

// EstimateFee - Calculates the estimated size of the transaction and returns the total fee for the given feePerByte
func (wallet *ERC20Wallet) EstimateFee(ins []wi.TransactionInput, outs []wi.TransactionOutput, feePerByte uint64) uint64 {
	sum := big.NewInt(0)
	for _, out := range outs {
		gas, err := wallet.client.EstimateTxnGas(wallet.account.Address(),
			common.HexToAddress(out.Address.String()), &out.Value)
		if err != nil {
			return sum.Uint64()
		}
		sum.Add(sum, gas)
	}
	return sum.Uint64()
}

// EstimateSpendFee - Build a spend transaction for the amount and return the transaction fee
func (wallet *ERC20Wallet) EstimateSpendFee(amount int64, feeLevel wi.FeeLevel) (uint64, error) {
	gas, err := wallet.client.EstimateGasSpend(wallet.account.Address(), big.NewInt(amount))
	return gas.Uint64(), err
}

// SweepAddress - Build and broadcast a transaction that sweeps all coins from an address. If it is a p2sh multisig, the redeemScript must be included
func (wallet *ERC20Wallet) SweepAddress(utxos []wi.TransactionInput, address *btcutil.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wi.FeeLevel) (string, error) {
	return "", nil
}

// ExchangeRates - return the exchangerates
func (wallet *ERC20Wallet) ExchangeRates() wi.ExchangeRates {
	return wallet.exchangeRates
}

func (wallet *ERC20Wallet) callAddTokenTransaction(script EthRedeemScript, value *big.Int) (common.Hash, error) {

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
	auth := bind.NewKeyedTransactor(wallet.account.privateKey)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0) // in wei
	auth.GasLimit = 4000000    // in units
	auth.GasPrice = gasPrice

	shash, _, err := GenTokenScriptHash(script)
	if err != nil {
		return h, err
	}

	smtct, err := NewEscrow(script.MultisigAddress, wallet.client)
	if err != nil {
		log.Fatalf("error initilaizing contract failed: %s", err.Error())
	}

	var tx *types.Transaction

	tx, err = wallet.token.Approve(auth, script.MultisigAddress, value)

	if err != nil {
		return common.BigToHash(big.NewInt(0)), err
	}

	//time.Sleep(2 * time.Minute)
	header, err := wallet.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Errorf("error fetching latest blk: %v", err)
	}
	tclient, err := ethclient.Dial("wss://rinkeby.infura.io/ws")
	if err != nil {
		log.Errorf("error establishing ws conn: %v", err)
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{script.TokenAddress},
		FromBlock: header.Number,
		Topics:    [][]common.Hash{{common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925")}},
	}
	logs := make(chan types.Log)
	sub1, err := tclient.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		return common.BigToHash(big.NewInt(0)), err
	}
	defer sub1.Unsubscribe()
	flag := false
	for !flag {
		select {
		case err := <-sub1.Err():
			log.Fatal(err)
		case vLog := <-logs:
			//fmt.Println(vLog) // pointer to event log
			//spew.Dump(vLog)
			//fmt.Println(vLog.Topics[0])
			fmt.Println(vLog.Address.String())
			if tx.Hash() == vLog.TxHash {
				fmt.Println("we have found the approval ...")
				//time.Sleep(2 * time.Minute)
				spew.Dump(vLog)
				nonce, _ = wallet.client.PendingNonceAt(context.Background(), fromAddress)
				auth.Nonce = big.NewInt(int64(nonce))
				auth.Value = big.NewInt(0) // in wei
				auth.GasLimit = 4000000    // in units
				auth.GasPrice = gasPrice

				tx, err = smtct.AddTokenTransaction(auth, script.Buyer, script.Seller,
					script.Moderator, script.Threshold, script.Timeout, shash,
					value, script.TxnID, wallet.deployAddressMain)

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
			value, script.TxnID, wallet.deployAddressMain)

		if err == nil {
			h = tx.Hash()
		}
	*/

	return h, err

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
		ecKeys = append(ecKeys, common.BytesToAddress(ecKey.SerializeUncompressed()))
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
	builder.TokenAddress = wallet.deployAddressMain

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
	copy(txHash[:], txnHash)

	sig, err := crypto.Sign(txHash[:], wallet.account.privateKey)
	if err != nil {
		log.Errorf("error signing in createmultisig : %v", err)
	}
	sigs = append(sigs, wi.Signature{InputIndex: 1, Signature: sig})

	return sigs, err
}

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

	tx, err = smtct.Execute(auth, vSlice, rSlice, sSlice, shash, destinations, amounts)

	//fmt.Println(tx)
	//fmt.Println(err)

	if err != nil {
		return nil, err
	}

	ret, err := tx.MarshalJSON()

	return ret, err
}

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
	return 0, 0, nil
}

// Close will stop the wallet daemon
func (wallet *ERC20Wallet) Close() {
	// stop the wallet daemon
}

// CreateAddress - used to generate a new address
func (wallet *ERC20Wallet) CreateAddress() (common.Address, error) {
	fromAddress := wallet.account.Address()
	nonce, err := wallet.client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		fmt.Println(err)
	}
	addr := crypto.CreateAddress(fromAddress, nonce)
	//fmt.Println("Addr : ", addr.String())
	return addr, err
}
