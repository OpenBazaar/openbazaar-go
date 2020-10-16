package service

import (
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/OpenBazaar/multiwallet/cache"
	"github.com/OpenBazaar/multiwallet/datastore"
	"github.com/OpenBazaar/multiwallet/keys"
	"github.com/OpenBazaar/multiwallet/model/mock"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ltcsuite/ltcd/chaincfg/chainhash"
)

func mockWalletService() (*WalletService, error) {
	datastore := datastore.NewMockMultiwalletDatastore()

	db, err := datastore.GetDatastoreForWallet(wallet.Bitcoin)
	if err != nil {
		return nil, err
	}
	params := &chaincfg.MainNetParams

	seed, err := hex.DecodeString("16c034c59522326867593487c03a8f9615fb248406dd0d4ffb3a6b976a248403")
	if err != nil {
		return nil, err
	}
	master, err := hdkeychain.NewMaster(seed, params)
	if err != nil {
		return nil, err
	}
	km, err := keys.NewKeyManager(db.Keys(), params, master, wallet.Bitcoin, bitcoinAddress)
	if err != nil {
		return nil, err
	}
	cli := mock.NewMockApiClient(func(addr btcutil.Address) ([]byte, error) {
		return txscript.PayToAddrScript(addr)
	})
	return NewWalletService(db, km, cli, params, wallet.Bitcoin, cache.NewMockCacher())
}

func bitcoinAddress(key *hdkeychain.ExtendedKey, params *chaincfg.Params) (btcutil.Address, error) {
	return key.Address(params)
}

func TestWalletService_ChainTip(t *testing.T) {
	ws, err := mockWalletService()
	if err != nil {
		t.Fatal(err)
	}
	ws.UpdateState()
	height, hash := ws.ChainTip()
	if height != 1289594 {
		t.Error("returned incorrect height")
	}
	if hash.String() != "000000000000004c68a477283a8db18c1d1c2155b03d9bc23d587ac5e1c4d1af" {
		t.Error("returned incorrect best hash")
	}
}

func TestWalletService_syncTxs(t *testing.T) {
	ws, err := mockWalletService()
	if err != nil {
		t.Fatal(err)
	}
	ws.syncTxs(ws.getStoredAddresses())

	txns, err := ws.db.Txns().GetAll(true)
	if err != nil {
		t.Error(err)
	}
	if len(txns) != 3 {
		t.Error("failed to update state correctly")
	}
	txMap := make(map[string]wallet.Txn)
	for _, tx := range txns {
		txMap[tx.Txid] = tx
	}

	tx, ok := txMap["54ebaa07c42216393b9d5816e40dd608593b92c42e2d6525f45bdd36bce8fe4d"]
	if !ok {
		t.Error("failed to return tx")
	}
	if tx.Value != 2717080 || tx.WatchOnly {
		t.Error("failed to return incorrect value for tx")
	}
	tx, ok = txMap["ff2b865c3b73439912eebf4cce9a15b12c7d7bcdd14ae1110a90541426c4e7c5"]
	if !ok {
		t.Error("failed to return tx")
	}
	if tx.Value != -1717080 || tx.WatchOnly {
		t.Error("failed to return incorrect value for tx")
	}
	tx, ok = txMap["1d4288fa682fa376fbae73dbd74ea04b9ea33011d63315ca9d2d50d081e671d5"]
	if !ok {
		t.Error("failed to return tx")
	}
	if tx.Value != 10000000 || tx.WatchOnly {
		t.Error("failed to return incorrect value for tx")
	}
}

func TestWalletService_syncUtxos(t *testing.T) {
	ws, err := mockWalletService()
	if err != nil {
		t.Fatal(err)
	}
	script, err := hex.DecodeString("a91457fc729da2a83dc8cd3c1835351c4a813c2ae8ba87")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.db.WatchedScripts().Put(script); err != nil {
		t.Fatal(err)
	}
	ws.syncUtxos(ws.getStoredAddresses())

	utxos, err := ws.db.Utxos().GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(utxos) != 3 {
		t.Error("failed to update state correctly")
	}

	utxoMap := make(map[string]wallet.Utxo)
	for _, u := range utxos {
		utxoMap[u.Op.Hash.String()+":"+strconv.Itoa(int(u.Op.Index))] = u
	}

	u, ok := utxoMap["ff2b865c3b73439912eebf4cce9a15b12c7d7bcdd14ae1110a90541426c4e7c5:1"]
	if !ok {
		t.Error("failed to return correct utxo")
	}
	if u.Value != 1000000 || u.WatchOnly {
		t.Error("returned incorrect value")
	}
	u, ok = utxoMap["1d4288fa682fa376fbae73dbd74ea04b9ea33011d63315ca9d2d50d081e671d5:1"]
	if !ok {
		t.Error("failed to return correct utxo")
	}
	if u.Value != 10000000 || u.WatchOnly {
		t.Error("returned incorrect value")
	}
	u, ok = utxoMap["830bf683ab8eec1a75d891689e2989f846508bc7d500cb026ef671c2d1dce20c:1"]
	if !ok {
		t.Error("failed to return correct utxo")
	}
	if u.Value != 751918 || !u.WatchOnly {
		t.Error("returned incorrect value")
	}
}

func TestWalletService_TestSyncWatchOnly(t *testing.T) {
	ws, err := mockWalletService()
	if err != nil {
		t.Fatal(err)
	}
	script, err := hex.DecodeString("a91457fc729da2a83dc8cd3c1835351c4a813c2ae8ba87")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.db.WatchedScripts().Put(script); err != nil {
		t.Fatal(err)
	}
	ws.syncTxs(ws.getStoredAddresses())
	ws.syncUtxos(ws.getStoredAddresses())

	txns, err := ws.db.Txns().GetAll(true)
	if err != nil {
		t.Error(err)
	}
	if len(txns) != 4 {
		t.Error("failed to update state correctly")
	}
	txMap := make(map[string]wallet.Txn)
	for _, tx := range txns {
		txMap[tx.Txid] = tx
	}

	tx, ok := txMap["830bf683ab8eec1a75d891689e2989f846508bc7d500cb026ef671c2d1dce20c"]
	if !ok {
		t.Fatal("Failed to return correct transaction")
	}
	if !tx.WatchOnly {
		t.Error("failed to return correct value for tx")
	}

	utxos, err := ws.db.Utxos().GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(utxos) != 3 {
		t.Error("failed to update state correctly")
	}
	utxoMap := make(map[string]wallet.Utxo)
	for _, u := range utxos {
		utxoMap[u.Op.String()] = u
	}

	utxo, ok := utxoMap["830bf683ab8eec1a75d891689e2989f846508bc7d500cb026ef671c2d1dce20c:1"]
	if !ok {
		t.Fatal("Failed to return correct utxo")
	}
	if !utxo.WatchOnly {
		t.Error("failed to return correct value for utxo")
	}
}

func TestWalletService_ProcessIncomingTransaction(t *testing.T) {
	ws, err := mockWalletService()
	if err != nil {
		t.Fatal(err)
	}

	// Process an incoming transaction
	ws.ProcessIncomingTransaction(mock.MockTransactions[0])
	txns, err := ws.db.Txns().GetAll(true)
	if err != nil {
		t.Error(err)
	}
	if len(txns) != 1 {
		t.Error("failed to update state correctly")
	}
	if txns[0].Txid != mock.MockTransactions[0].Txid {
		t.Error("saved incorrect transaction")
	}
	if txns[0].Value != 2717080 {
		t.Error("saved incorrect value")
	}
	if txns[0].WatchOnly {
		t.Error("saved incorrect watch only")
	}

	utxos, err := ws.db.Utxos().GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(utxos) != 1 {
		t.Error("failed to update state correctly")
	}
	if utxos[0].WatchOnly {
		t.Error("saved incorrect watch only")
	}
	if utxos[0].Op.Hash.String() != mock.MockTransactions[0].Txid {
		t.Error("saved incorrect transaction ID")
	}
	if utxos[0].Op.Index != 1 {
		t.Error("saved incorrect outpoint index")
	}
	if utxos[0].Value != 2717080 {
		t.Error("saved incorrect value")
	}

	// Process an outgoing transaction. Make sure it deletes the utxo
	ws.ProcessIncomingTransaction(mock.MockTransactions[1])
	txns, err = ws.db.Txns().GetAll(true)
	if err != nil {
		t.Error(err)
	}
	if len(txns) != 2 {
		t.Error("failed to update state correctly")
	}

	utxos, err = ws.db.Utxos().GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(utxos) != 1 {
		t.Error("failed to update state correctly")
	}
	if utxos[0].Op.Hash.String() != mock.MockTransactions[1].Txid {
		t.Error("failed to save correct utxo")
	}
	if utxos[0].Op.Index != 1 {
		t.Error("failed to save correct utxo")
	}
}

func TestWalletService_processIncomingBlock(t *testing.T) {
	ws, err := mockWalletService()
	if err != nil {
		t.Fatal(err)
	}
	ws.chainHeight = uint32(mock.MockBlocks[0].Height)
	ws.bestBlock = mock.MockBlocks[0].Hash

	// Check update height
	ws.processIncomingBlock(mock.MockBlocks[1])
	height, hash := ws.ChainTip()
	if height != uint32(mock.MockBlocks[1].Height) {
		t.Error("failed to update height")
	}
	if hash.String() != mock.MockBlocks[1].Hash {
		t.Error("failed to update hash")
	}

	// Check update height of unconfirmed txs and utxos
	tx := mock.MockTransactions[0]
	tx.Confirmations = 0
	ws.ProcessIncomingTransaction(tx)

	ws.processIncomingBlock(mock.MockBlocks[2])
	time.Sleep(time.Second / 2)

	txns, err := ws.db.Txns().GetAll(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) != 1 {
		t.Fatal("Returned incorrect number of txs")
	}
	if txns[0].Height != int32(mock.MockBlocks[2].Height-14) {
		t.Error("returned incorrect transaction height")
	}

	utxos, err := ws.db.Utxos().GetAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(utxos) != 1 {
		t.Fatal("Returned incorrect number of utxos")
	}
	if utxos[0].AtHeight != int32(mock.MockBlocks[2].Height-14) {
		t.Error("returned incorrect utxo height")
	}

	// Test updateState() is called during reorg
	block := mock.MockBlocks[1]
	block.Hash = "0000000000000000003c4b7f56e45567980f02012ea00d8e384267a2d825fcf9"
	ws.processIncomingBlock(block)

	time.Sleep(time.Second / 2)

	txns, err = ws.db.Txns().GetAll(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) != 3 {
		t.Fatal("Returned incorrect number of txs")
	}

	utxos, err = ws.db.Utxos().GetAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(utxos) != 3 {
		t.Fatal("Returned incorrect number of utxos")
	}
}

func TestWalletService_listenersFired(t *testing.T) {
	nCallbacks := 0
	var response wallet.TransactionCallback
	cb := func(callback wallet.TransactionCallback) {
		nCallbacks++
		response = callback
	}
	ws, err := mockWalletService()
	if err != nil {
		t.Fatal(err)
	}
	ws.AddTransactionListener(cb)
	tx := mock.MockTransactions[0]
	tx.Confirmations = 0
	ws.saveSingleTxToDB(tx, int32(mock.MockBlocks[0].Height), ws.getStoredAddresses())
	if nCallbacks != 1 {
		t.Errorf("expected 1 callback but had %d", nCallbacks)
	}
	ch, err := chainhash.NewHashFromStr(response.Txid)
	if err != nil {
		t.Errorf("failed getting hash from %s: %s", response.Txid, err)
	}
	if ch.String() != mock.MockTransactions[0].Txid {
		t.Errorf("expected hash to be %s, but was %s", mock.MockTransactions[0].Txid, ch.String())
	}
	if response.Value != 2717080 {
		t.Errorf("expected tx value to be 2717080, but was %d", response.Value)
	}
	if response.Height != 0 {
		t.Error("returned incorrect height")
	}
	if response.WatchOnly {
		t.Error("returned incorrect watch only")
	}

	// Test watch only
	script, err := hex.DecodeString("a91457fc729da2a83dc8cd3c1835351c4a813c2ae8ba87")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.db.WatchedScripts().Put(script); err != nil {
		t.Fatal(err)
	}
	ws.saveSingleTxToDB(mock.MockTransactions[3], int32(mock.MockBlocks[0].Height), ws.getStoredAddresses())
	if nCallbacks != 2 {
		t.Error("failed to fire transaction callback")
	}
	ch, err = chainhash.NewHashFromStr(response.Txid)
	if err != nil {
		t.Error(err)
	}
	if ch.String() != mock.MockTransactions[3].Txid {
		t.Error("returned incorrect txid")
	}
	if response.Height != 1289594-1 {
		t.Error("returned incorrect height")
	}
	if !response.WatchOnly {
		t.Error("returned incorrect watch only")
	}

	// Test fired when height is updated
	tx = mock.MockTransactions[0]
	tx.Confirmations = 1
	ws.saveSingleTxToDB(tx, int32(mock.MockBlocks[0].Height), ws.getStoredAddresses())
	if nCallbacks != 3 {
		t.Error("failed to fire transaction callback")
	}
	ch, err = chainhash.NewHashFromStr(response.Txid)
	if err != nil {
		t.Error(err)
	}
	if ch.String() != mock.MockTransactions[0].Txid {
		t.Error("returned incorrect txid")
	}
	if response.Value != 2717080 {
		t.Error("returned incorrect value")
	}
	if response.Height != int32(mock.MockBlocks[0].Height) {
		t.Error("returned incorrect height")
	}
	if response.WatchOnly {
		t.Error("returned incorrect watch only")
	}
}

func TestWalletService_getStoredAddresses(t *testing.T) {
	ws, err := mockWalletService()
	if err != nil {
		t.Fatal(err)
	}

	types := []wallet.CoinType{
		wallet.Bitcoin,
		wallet.BitcoinCash,
		wallet.Zcash,
		wallet.Litecoin,
	}

	script, err := hex.DecodeString("a91457fc729da2a83dc8cd3c1835351c4a813c2ae8ba87")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.db.WatchedScripts().Put(script); err != nil {
		t.Fatal(err)
	}

	for _, ty := range types {
		ws.coinType = ty
		addrs := ws.getStoredAddresses()
		if len(addrs) != 41 {
			t.Error("returned incorrect number of addresses")
		}
		switch ty {
		case wallet.Bitcoin:
			sa, ok := addrs["39iF8cDMhctrPVoPbi2Vb1NnErg6CEB7BZ"]
			if !sa.WatchOnly || !ok {
				t.Error("returned incorrect watch only address")
			}
		case wallet.BitcoinCash:
			sa, ok := addrs["pptlcu5a525rmjxd8svr2dguf2qnc2hghgln5xu4l7"]
			if !sa.WatchOnly || !ok {
				t.Error("returned incorrect watch only address")
			}
		case wallet.Zcash:
			sa, ok := addrs["t3Sar8wdVfwgSz8rHY8qcipUhVWsB2x2xxa"]
			if !sa.WatchOnly || !ok {
				t.Error("returned incorrect watch only address")
			}
		case wallet.Litecoin:
			sa, ok := addrs["39iF8cDMhctrPVoPbi2Vb1NnErg6CEB7BZ"]
			if !sa.WatchOnly || !ok {
				t.Error("returned incorrect watch only address")
			}
		}
	}
}
