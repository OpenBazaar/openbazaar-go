package zcash

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/OpenBazaar/multiwallet/cache"
	"github.com/OpenBazaar/multiwallet/datastore"
	"github.com/OpenBazaar/multiwallet/keys"
	"github.com/OpenBazaar/multiwallet/model/mock"
	"github.com/OpenBazaar/multiwallet/service"
	"github.com/OpenBazaar/multiwallet/util"
	zaddr "github.com/OpenBazaar/multiwallet/zcash/address"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
)

type FeeResponse struct {
	Priority int `json:"priority"`
	Normal   int `json:"normal"`
	Economic int `json:"economic"`
}

func newMockWallet() (*ZCashWallet, error) {
	mockDb := datastore.NewMockMultiwalletDatastore()

	db, err := mockDb.GetDatastoreForWallet(wallet.Zcash)
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
	km, err := keys.NewKeyManager(db.Keys(), params, master, wallet.Zcash, zcashAddress)
	if err != nil {
		return nil, err
	}

	fp := util.NewFeeProvider(2000, 300, 200, 100, nil)

	bw := &ZCashWallet{
		params: params,
		km:     km,
		db:     db,
		fp:     fp,
	}
	cli := mock.NewMockApiClient(bw.AddressToScript)
	ws, err := service.NewWalletService(db, km, cli, params, wallet.Zcash, cache.NewMockCacher())
	if err != nil {
		return nil, err
	}
	bw.client = cli
	bw.ws = ws
	return bw, nil
}

func TestWalletService_VerifyWatchScriptFilter(t *testing.T) {
	// Verify that AddWatchedAddress should never add a script which already represents a key from its own wallet
	w, err := newMockWallet()
	if err != nil {
		t.Fatal(err)
	}
	keys := w.km.GetKeys()

	addr, err := w.km.KeyToAddress(keys[0])
	if err != nil {
		t.Fatal(err)
	}
	err = w.AddWatchedAddresses(addr)
	if err != nil {
		t.Fatal(err)
	}

	watchScripts, err := w.db.WatchedScripts().GetAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(watchScripts) != 0 {
		t.Error("Put watched scripts fails on key manager owned key")
	}
}

func TestWalletService_VerifyWatchScriptPut(t *testing.T) {
	// Verify that AddWatchedAddress should add a script which does not represent a key from its own wallet
	w, err := newMockWallet()
	if err != nil {
		t.Fatal(err)
	}

	addr, err := w.DecodeAddress("t1aZvxRLCGVeMPFXvqfnBgHVEbi4c6g8MVa")
	if err != nil {
		t.Fatal(err)
	}

	err = w.AddWatchedAddresses(addr)
	if err != nil {
		t.Fatal(err)
	}

	watchScripts, err := w.db.WatchedScripts().GetAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(watchScripts) == 0 {
		t.Error("Put watched scripts fails on non-key manager owned key")
	}
}

func waitForTxnSync(t *testing.T, txnStore wallet.Txns) {
	// Look for a known txn, this sucks a bit. It would be better to check if the
	// number of stored txns matched the expected, but not all the mock
	// transactions are relevant, so the numbers don't add up.
	// Even better would be for the wallet to signal that the initial sync was
	// done.
	lastTxn := mock.MockTransactions[len(mock.MockTransactions)-2]
	txHash, err := chainhash.NewHashFromStr(lastTxn.Txid)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		if _, err := txnStore.Get(*txHash); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("timeout waiting for wallet to sync transactions")
}

func TestZCashWallet_buildTx(t *testing.T) {
	w, err := newMockWallet()
	w.ws.Start()
	time.Sleep(time.Second / 2)
	if err != nil {
		t.Error(err)
	}
	addr, err := w.DecodeAddress("t1hASvMj8e6TXWryuB3L5TKXJB7XfNioZP3")
	if err != nil {
		t.Error(err)
	}
	// Test build normal tx
	tx, err := w.buildTx(1500000, addr, wallet.NORMAL, nil)
	if err != nil {
		w.DumpTables(os.Stdout)
		t.Error(err)
		return
	}
	if !containsOutput(tx, addr) {
		t.Error("Built tx does not contain the requested output")
	}
	if !validInputs(tx, w.db) {
		t.Error("Built tx does not contain valid inputs")
	}
	if !validChangeAddress(tx, w.db, w.params) {
		t.Error("Built tx does not contain a valid change output")
	}

	// Insuffient funds
	_, err = w.buildTx(1000000000, addr, wallet.NORMAL, nil)
	if err != wallet.ErrorInsuffientFunds {
		t.Error("Failed to throw insuffient funds error")
	}

	// Dust
	_, err = w.buildTx(1, addr, wallet.NORMAL, nil)
	if err != wallet.ErrorDustAmount {
		t.Error("Failed to throw dust error")
	}
}

func TestZCashWallet_buildSpendAllTx(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	w.ws.Start()
	time.Sleep(time.Second / 2)

	waitForTxnSync(t, w.db.Txns())
	addr, err := w.DecodeAddress("t1hASvMj8e6TXWryuB3L5TKXJB7XfNioZP3")
	if err != nil {
		t.Error(err)
	}

	// Test build spendAll tx
	tx, err := w.buildSpendAllTx(addr, wallet.NORMAL)
	if err != nil {
		t.Error(err)
	}
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		t.Fatal(err)
	}
	spendableUtxos := 0
	for _, u := range utxos {
		if !u.WatchOnly {
			spendableUtxos++
		}
	}
	if len(tx.TxIn) != spendableUtxos {
		t.Error("Built tx does not spend all available utxos")
	}
	if !containsOutput(tx, addr) {
		t.Error("Built tx does not contain the requested output")
	}
	if !validInputs(tx, w.db) {
		t.Error("Built tx does not contain valid inputs")
	}
	if len(tx.TxOut) != 1 {
		t.Error("Built tx should only have one output")
	}
}

func containsOutput(tx *wire.MsgTx, addr btcutil.Address) bool {
	for _, o := range tx.TxOut {
		script, _ := zaddr.PayToAddrScript(addr)
		if bytes.Equal(script, o.PkScript) {
			return true
		}
	}
	return false
}

func validInputs(tx *wire.MsgTx, db wallet.Datastore) bool {
	utxos, _ := db.Utxos().GetAll()
	uMap := make(map[wire.OutPoint]bool)
	for _, u := range utxos {
		uMap[u.Op] = true
	}
	for _, in := range tx.TxIn {
		if !uMap[in.PreviousOutPoint] {
			return false
		}
	}
	return true
}

func validChangeAddress(tx *wire.MsgTx, db wallet.Datastore, params *chaincfg.Params) bool {
	for _, out := range tx.TxOut {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(out.PkScript, params)
		if err != nil {
			continue
		}
		if len(addrs) == 0 {
			continue
		}
		_, err = db.Keys().GetPathForKey(addrs[0].ScriptAddress())
		if err == nil {
			return true
		}
	}
	return false
}

func TestZCashWallet_GenerateMultisigScript(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	key1, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}
	pubkey1, err := key1.ECPubKey()
	if err != nil {
		t.Error(err)
	}
	key2, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}
	pubkey2, err := key2.ECPubKey()
	if err != nil {
		t.Error(err)
	}
	key3, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}
	pubkey3, err := key3.ECPubKey()
	if err != nil {
		t.Error(err)
	}
	keys := []hdkeychain.ExtendedKey{*key1, *key2, *key3}

	// test without timeout
	addr, redeemScript, err := w.generateMultisigScript(keys, 2, 0, nil)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "t3S5yuHPzajqHcaJ6WDTGAwTuK9VDvWYj7r" {
		t.Error("Returned invalid address")
	}

	rs := "52" + // OP_2
		"21" + // OP_PUSHDATA(33)
		hex.EncodeToString(pubkey1.SerializeCompressed()) + // pubkey1
		"21" + // OP_PUSHDATA(33)
		hex.EncodeToString(pubkey2.SerializeCompressed()) + // pubkey2
		"21" + // OP_PUSHDATA(33)
		hex.EncodeToString(pubkey3.SerializeCompressed()) + // pubkey3
		"53" + // OP_3
		"ae" // OP_CHECKMULTISIG
	rsBytes, err := hex.DecodeString(rs)
	if !bytes.Equal(rsBytes, redeemScript) {
		t.Error("Returned invalid redeem script")
	}
}

func TestZCashWallet_newUnsignedTransaction(t *testing.T) {
	w, err := newMockWallet()
	w.ws.Start()
	time.Sleep(time.Second / 2)
	if err != nil {
		t.Error(err)
	}
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		t.Error(err)
	}
	addr, err := w.DecodeAddress("t3ZZqETXWTohq3xXHxD9yzfq4UxpcACLkVc")
	if err != nil {
		t.Error(err)
	}

	script, err := zaddr.PayToAddrScript(addr)
	if err != nil {
		t.Error(err)
	}
	out := wire.NewTxOut(10000, script)
	outputs := []*wire.TxOut{out}

	changeSource := func() ([]byte, error) {
		addr := w.CurrentAddress(wallet.INTERNAL)
		script, err := zaddr.PayToAddrScript(addr)
		if err != nil {
			return []byte{}, err
		}
		return script, nil
	}

	inputSource := func(target btcutil.Amount) (total btcutil.Amount, inputs []*wire.TxIn, inputValues []btcutil.Amount, scripts [][]byte, err error) {
		total += btcutil.Amount(utxos[0].Value)
		in := wire.NewTxIn(&utxos[0].Op, []byte{}, [][]byte{})
		in.Sequence = 0 // Opt-in RBF so we can bump fees
		inputs = append(inputs, in)
		return total, inputs, inputValues, scripts, nil
	}

	// Regular transaction
	authoredTx, err := newUnsignedTransaction(outputs, btcutil.Amount(1000), inputSource, changeSource)
	if err != nil {
		t.Error(err)
	}
	if len(authoredTx.Tx.TxOut) != 2 {
		t.Error("Returned incorrect number of outputs")
	}
	if len(authoredTx.Tx.TxIn) != 1 {
		t.Error("Returned incorrect number of inputs")
	}

	// Insufficient funds
	outputs[0].Value = 1000000000
	_, err = newUnsignedTransaction(outputs, btcutil.Amount(1000), inputSource, changeSource)
	if err == nil {
		t.Error("Failed to return insuffient funds error")
	}
}

func TestZCashWallet_CreateMultisigSignature(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	ins, outs, redeemScript, err := buildTxData(w)
	if err != nil {
		t.Error(err)
	}

	key1, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}

	sigs, err := w.CreateMultisigSignature(ins, outs, key1, redeemScript, 50)
	if err != nil {
		t.Error(err)
	}
	if len(sigs) != 2 {
		t.Error(err)
	}
	for _, sig := range sigs {
		if len(sig.Signature) == 0 {
			t.Error("Returned empty signature")
		}
	}
}

func buildTxData(w *ZCashWallet) ([]wallet.TransactionInput, []wallet.TransactionOutput, []byte, error) {
	redeemScript := "522103c157f2a7c178430972263232c9306110090c50b44d4e906ecd6d377eec89a53c210205b02b9dbe570f36d1c12e3100e55586b2b9dc61d6778c1d24a8eaca03625e7e21030c83b025cd6bdd8c06e93a2b953b821b4a8c29da211335048d7dc3389706d7e853ae"
	redeemScriptBytes, err := hex.DecodeString(redeemScript)
	if err != nil {
		return nil, nil, nil, err
	}
	h1, err := hex.DecodeString("1a20f4299b4fa1f209428dace31ebf4f23f13abd8ed669cebede118343a6ae05")
	if err != nil {
		return nil, nil, nil, err
	}
	in1 := wallet.TransactionInput{
		OutpointHash:  h1,
		OutpointIndex: 1,
	}
	h2, err := hex.DecodeString("458d88b4ae9eb4a347f2e7f5592f1da3b9ddf7d40f307f6e5d7bc107a9b3e90e")
	if err != nil {
		return nil, nil, nil, err
	}
	in2 := wallet.TransactionInput{
		OutpointHash:  h2,
		OutpointIndex: 0,
	}
	addr, err := w.DecodeAddress("t3ZZqETXWTohq3xXHxD9yzfq4UxpcACLkVc")
	if err != nil {
		return nil, nil, nil, err
	}

	out := wallet.TransactionOutput{
		Value:   20000,
		Address: addr,
	}
	return []wallet.TransactionInput{in1, in2}, []wallet.TransactionOutput{out}, redeemScriptBytes, nil
}

func TestZCashWallet_Multisign(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	ins, outs, redeemScript, err := buildTxData(w)
	if err != nil {
		t.Error(err)
	}

	key1, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}

	key2, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}

	sigs1, err := w.CreateMultisigSignature(ins, outs, key1, redeemScript, 50)
	if err != nil {
		t.Error(err)
	}
	if len(sigs1) != 2 {
		t.Error(err)
	}
	sigs2, err := w.CreateMultisigSignature(ins, outs, key2, redeemScript, 50)
	if err != nil {
		t.Error(err)
	}
	if len(sigs2) != 2 {
		t.Error(err)
	}
	_, err = w.Multisign(ins, outs, sigs1, sigs2, redeemScript, 50, false)
	if err != nil {
		t.Error(err)
	}
}

func TestZCashWallet_bumpFee(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Fatal(err)
	}

	w.ws.Start()
	time.Sleep(time.Second / 2)
	txns, err := w.db.Txns().GetAll(false)
	if err != nil {
		t.Fatal(err)
	}

	ch, err := chainhash.NewHashFromStr(txns[2].Txid)
	if err != nil {
		t.Fatal(err)
	}
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range utxos {
		if u.Op.Hash.IsEqual(ch) {
			u.AtHeight = 0
			if err := w.db.Utxos().Put(u); err != nil {
				t.Fatal(err)
			}
		}
	}

	w.db.Txns().UpdateHeight(*ch, 0, time.Now())

	// Test unconfirmed
	_, err = w.bumpFee(*ch)
	if err != nil {
		t.Error(err)
	}

	err = w.db.Txns().UpdateHeight(*ch, 1289597, time.Now())
	if err != nil {
		t.Error(err)
	}

	// Test confirmed
	_, err = w.bumpFee(*ch)
	if err == nil {
		t.Error("Should not be able to bump fee of confirmed txs")
	}
}

func TestZCashWallet_sweepAddress(t *testing.T) {
	w, err := newMockWallet()
	w.ws.Start()
	time.Sleep(time.Second / 2)
	if err != nil {
		t.Error(err)
	}
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		t.Error(err)
	}
	var in wallet.TransactionInput
	var key *hdkeychain.ExtendedKey
	for _, ut := range utxos {
		if ut.Value > 0 && !ut.WatchOnly {
			addr, err := w.ScriptToAddress(ut.ScriptPubkey)
			if err != nil {
				t.Error(err)
			}
			key, err = w.km.GetKeyForScript(addr.ScriptAddress())
			if err != nil {
				t.Error(err)
			}
			h, err := hex.DecodeString(ut.Op.Hash.String())
			if err != nil {
				t.Error(err)
			}
			in = wallet.TransactionInput{
				LinkedAddress: addr,
				Value:         ut.Value,
				OutpointIndex: ut.Op.Index,
				OutpointHash:  h,
			}
		}
	}
	// P2PKH addr
	_, err = w.sweepAddress([]wallet.TransactionInput{in}, nil, key, nil, wallet.NORMAL)
	if err != nil {
		t.Error(err)
		return
	}

	// 1 of 2 P2WSH
	for _, ut := range utxos {
		if ut.Value > 0 && ut.WatchOnly {
			addr, err := w.ScriptToAddress(ut.ScriptPubkey)
			if err != nil {
				t.Error(err)
			}
			h, err := hex.DecodeString(ut.Op.Hash.String())
			if err != nil {
				t.Error(err)
			}
			in = wallet.TransactionInput{
				LinkedAddress: addr,
				Value:         ut.Value,
				OutpointIndex: ut.Op.Index,
				OutpointHash:  h,
			}
		}
	}
	key1, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}

	key2, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}
	_, redeemScript, err := w.GenerateMultisigScript([]hdkeychain.ExtendedKey{*key1, *key2}, 1, 0, nil)
	if err != nil {
		t.Error(err)
	}
	_, err = w.sweepAddress([]wallet.TransactionInput{in}, nil, key1, &redeemScript, wallet.NORMAL)
	if err != nil {
		t.Error(err)
	}
}

func TestZCashWallet_estimateSpendFee(t *testing.T) {
	w, err := newMockWallet()
	w.ws.Start()
	time.Sleep(time.Second / 2)
	if err != nil {
		t.Error(err)
	}
	fee, err := w.estimateSpendFee(1000, wallet.NORMAL)
	if err != nil {
		t.Error(err)
	}
	if fee <= 0 {
		t.Error("Returned incorrect fee")
	}
}

func buildTestTx() (*wire.MsgTx, []byte, error) {
	expected, err := hex.DecodeString(`0400008085202f8901a8c685478265f4c14dada651969c45a65e1aeb8cd6791f2f5bb6a1d9952104d9010000006b483045022100a61e5d557568c2ddc1d9b03a7173c6ce7c996c4daecab007ac8f34bee01e6b9702204d38fdc0bcf2728a69fde78462a10fb45a9baa27873e6a5fc45fb5c76764202a01210365ffea3efa3908918a8b8627724af852fc9b86d7375b103ab0543cf418bcaa7ffeffffff02005a6202000000001976a9148132712c3ff19f3a151234616777420a6d7ef22688ac8b959800000000001976a9145453e4698f02a38abdaa521cd1ff2dee6fac187188ac29b0040048b004000000000000000000000000`)
	if err != nil {
		return nil, nil, err
	}

	tx := wire.NewMsgTx(1)

	inHash, err := hex.DecodeString("a8c685478265f4c14dada651969c45a65e1aeb8cd6791f2f5bb6a1d9952104d9")
	if err != nil {
		return nil, nil, err
	}
	prevHash, err := chainhash.NewHash(inHash)
	if err != nil {
		return nil, nil, err
	}
	op := wire.NewOutPoint(prevHash, 1)

	scriptSig, err := hex.DecodeString("483045022100a61e5d557568c2ddc1d9b03a7173c6ce7c996c4daecab007ac8f34bee01e6b9702204d38fdc0bcf2728a69fde78462a10fb45a9baa27873e6a5fc45fb5c76764202a01210365ffea3efa3908918a8b8627724af852fc9b86d7375b103ab0543cf418bcaa7f")
	if err != nil {
		return nil, nil, err
	}
	txIn := wire.NewTxIn(op, scriptSig, nil)
	txIn.Sequence = 4294967294

	tx.TxIn = []*wire.TxIn{txIn}

	pkScirpt0, err := hex.DecodeString("76a9148132712c3ff19f3a151234616777420a6d7ef22688ac")
	if err != nil {
		return nil, nil, err
	}
	out0 := wire.NewTxOut(40000000, pkScirpt0)

	pkScirpt1, err := hex.DecodeString("76a9145453e4698f02a38abdaa521cd1ff2dee6fac187188ac")
	if err != nil {
		return nil, nil, err
	}
	out1 := wire.NewTxOut(9999755, pkScirpt1)
	tx.TxOut = []*wire.TxOut{out0, out1}

	tx.LockTime = 307241
	return tx, expected, nil
}

func TestSerializeVersion4Transaction(t *testing.T) {
	tx, expected, err := buildTestTx()
	if err != nil {
		t.Fatal(err)
	}

	serialized, err := serializeVersion4Transaction(tx, 307272)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(serialized, expected) {
		t.Fatal("Failed to serialize transaction correctly")
	}
}

func TestCalcSignatureHash(t *testing.T) {
	tx, _, err := buildTestTx()
	if err != nil {
		t.Fatal(err)
	}

	prevScript, err := hex.DecodeString("76a914507173527b4c3318a2aecd793bf1cfed705950cf88ac")
	if err != nil {
		t.Fatal(err)
	}
	sigHash, err := calcSignatureHash(prevScript, txscript.SigHashAll, tx, 0, 50000000, 307272)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := hex.DecodeString("8df91420215909927be677a978c36b528e1e7b4ba343acefdd259fe57f3f1f85")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sigHash, expected) {
		t.Fatal("Failed to calculate correct sig hash")
	}
}
