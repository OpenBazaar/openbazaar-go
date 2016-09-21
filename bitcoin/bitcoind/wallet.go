package bitcoind

import (
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcrpcclient"
	btc "github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/op/go-logging"
	b39 "github.com/tyler-smith/go-bip39"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
	"time"
)

var log = logging.MustGetLogger("bitcoind")

var account string = "OpenBazaar"

type BitcoindWallet struct {
	params           *chaincfg.Params
	repoPath         string
	trustedPeer      string
	masterPrivateKey *hd.ExtendedKey
	masterPublicKey  *hd.ExtendedKey
	listeners        []func(spvwallet.TransactionCallback)
	rpcClient        *btcrpcclient.Client
	binary           string
}

var connCfg *btcrpcclient.ConnConfig = &btcrpcclient.ConnConfig{
	Host:                 "localhost:8332",
	HTTPPostMode:         true, // Bitcoin core only supports HTTP POST mode
	DisableTLS:           true, // Bitcoin core does not provide TLS by default
	DisableAutoReconnect: false,
	DisableConnectOnNew:  false,
}

func NewBitcoindWallet(mnemonic string, params *chaincfg.Params, repoPath string, trustedPeer string, binary string, username string, password string) *BitcoindWallet {
	seed := b39.NewSeed(mnemonic, "")
	mPrivKey, _ := hd.NewMaster(seed, params)
	mPubKey, _ := mPrivKey.Neuter()

	if params.Name == chaincfg.TestNet3Params.Name || params.Name == chaincfg.RegressionNetParams.Name {
		connCfg.Host = "localhost:18332"
	}

	connCfg.User = username
	connCfg.Pass = password

	// TODO: need to make a similar script for windows
	script := []byte("#!/bin/bash\ncurl -d $1 http://localhost:8330/")
	ioutil.WriteFile(path.Join(repoPath, "notify.sh"), script, 0777)

	if trustedPeer != "" {
		trustedPeer = strings.Split(trustedPeer, ":")[0]
	}

	w := BitcoindWallet{
		params:           params,
		repoPath:         repoPath,
		trustedPeer:      trustedPeer,
		masterPrivateKey: mPrivKey,
		masterPublicKey:  mPubKey,
		binary:           binary,
	}
	return &w
}

func (w *BitcoindWallet) Start() {
	w.shutdownIfActive()

	args := []string{"-walletnotify='" + path.Join(w.repoPath, "notify.sh") + " %s'", "-server"}
	if w.params.Name == chaincfg.TestNet3Params.Name {
		args = append(args, "-testnet")
	} else if w.params.Name == chaincfg.RegressionNetParams.Name {
		args = append(args, "-regtest")
	}
	if w.trustedPeer != "" {
		args = append(args, "-connect="+w.trustedPeer)
	}
	client, _ := btcrpcclient.New(connCfg, nil)
	w.rpcClient = client
	go startNotificationListener(client, w.listeners)

	cmd := exec.Command(w.binary, args...)
	cmd.Start()
	ticker := time.NewTicker(20 * time.Second)
	go func() {
		for range ticker.C {
			log.Fatal("Failed to connect to bitcoind")
		}
	}()
	for {
		_, err := client.GetBlockCount()
		if err == nil {
			break
		}
	}
	ticker.Stop()
	log.Info("Connected to bitcoind")
}

// If bitcoind is already running let's shut it down so we restart it with our options
func (w *BitcoindWallet) shutdownIfActive() {
	client, err := btcrpcclient.New(connCfg, nil)
	if err != nil {
		return
	}
	client.Shutdown()
	time.Sleep(5 * time.Second)
}

func (w *BitcoindWallet) CurrencyCode() string {
	return "btc"
}

func (w *BitcoindWallet) MasterPrivateKey() *hd.ExtendedKey {
	return w.masterPrivateKey
}

func (w *BitcoindWallet) MasterPublicKey() *hd.ExtendedKey {
	return w.masterPublicKey
}

func (w *BitcoindWallet) CurrentAddress(purpose spvwallet.KeyPurpose) btc.Address {
	addr, _ := w.rpcClient.GetAccountAddress(account)
	return addr
}

func (w *BitcoindWallet) Balance() (confirmed, unconfirmed int64) {
	u, _ := w.rpcClient.GetUnconfirmedBalance(account)
	c, _ := w.rpcClient.GetBalance(account)
	return int64(c.ToUnit(btc.AmountSatoshi)), int64(u.ToUnit(btc.AmountSatoshi))
}

func (w *BitcoindWallet) ChainTip() uint32 {
	info, err := w.rpcClient.GetInfo()
	if err != nil {
		return uint32(0)
	}
	return uint32(info.Blocks)
}

func (w *BitcoindWallet) Spend(amount int64, addr btc.Address, feeLevel spvwallet.FeeLevel) error {
	amt, err := btc.NewAmount(float64(amount))
	if err != nil {
		return err
	}
	_, err = w.rpcClient.SendFrom(account, addr, amt)
	return err
}

func (w *BitcoindWallet) Params() *chaincfg.Params {
	return w.params
}

func (w *BitcoindWallet) AddTransactionListener(callback func(spvwallet.TransactionCallback)) {
	w.listeners = append(w.listeners, callback)
}

func (w *BitcoindWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int) (addr btc.Address, redeemScript []byte, err error) {
	var addrPubKeys []*btc.AddressPubKey
	for _, key := range keys {
		ecKey, err := key.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		k, err := btc.NewAddressPubKey(ecKey.SerializeCompressed(), w.params)
		if err != nil {
			return nil, nil, err
		}
		addrPubKeys = append(addrPubKeys, k)
	}
	redeemScript, err = txscript.MultiSigScript(addrPubKeys, threshold)
	if err != nil {
		return nil, nil, err
	}
	addr, err = btc.NewAddressScriptHash(redeemScript, w.params)
	if err != nil {
		return nil, nil, err
	}
	return addr, redeemScript, nil
}

func (w *BitcoindWallet) AddWatchedScript(script []byte) error {
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(script, w.params)
	if err != nil {
		return err
	}
	return w.rpcClient.ImportAddress(addrs[0].EncodeAddress())
}

func (w *BitcoindWallet) ReSyncBlockchain(fromHeight int32) {
	w.rpcClient.Shutdown()
	time.Sleep(5 * time.Second)
	args := []string{"-walletnotify='" + path.Join(w.repoPath, "notify.sh") + " %s'", "-server", "-rescan"}
	if w.params.Name == chaincfg.TestNet3Params.Name {
		args = append(args, "-testnet")
	} else if w.params.Name == chaincfg.RegressionNetParams.Name {
		args = append(args, "-regtest")
	}
	if w.trustedPeer != "" {
		args = append(args, "-connect="+w.trustedPeer)
	}
	cmd := exec.Command(w.binary, args...)
	cmd.Start()

	client, err := btcrpcclient.New(connCfg, nil)
	if err != nil {
		log.Error("Could not connect to bitcoind during rescan")
	}
	w.rpcClient = client
}

func (w *BitcoindWallet) Close() {
	w.rpcClient.Shutdown()
}
