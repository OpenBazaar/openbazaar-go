package bitcoin

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/OpenBazaar/multiwallet/cache"
	"github.com/OpenBazaar/multiwallet/client"
	"github.com/OpenBazaar/multiwallet/datastore"
	"github.com/OpenBazaar/multiwallet/keys"
	"github.com/OpenBazaar/multiwallet/service"
	"github.com/OpenBazaar/spvwallet"
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

func newMockWallet() (*BitcoinWallet, error) {
	mockDb := datastore.NewMockMultiwalletDatastore()

	db, err := mockDb.GetDatastoreForWallet(wallet.Bitcoin)
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
	km, err := keys.NewKeyManager(db.Keys(), params, master, wallet.Bitcoin, keyToAddress)
	if err != nil {
		return nil, err
	}

	fp := spvwallet.NewFeeProvider(2000, 300, 200, 100, "", nil)

	bw := &BitcoinWallet{
		params: params,
		km:     km,
		db:     db,
		fp:     fp,
	}
	cli := client.NewMockApiClient(bw.AddressToScript)
	ws, err := service.NewWalletService(db, km, cli, params, wallet.Bitcoin, cache.NewMockCacher())
	if err != nil {
		return nil, err
	}

	bw.client = cli
	bw.ws = ws
	return bw, nil
}

func waitForTxnSync(t *testing.T, txnStore wallet.Txns) {
	// Look for a known txn, this sucks a bit. It would be better to check if the
	// number of stored txns matched the expected, but not all the mock
	// transactions are relevant, so the numbers don't add up.
	// Even better would be for the wallet to signal that the initial sync was
	// done.
	lastTxn := client.MockTransactions[len(client.MockTransactions)-2]
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

func TestBitcoinWallet_buildTx(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	w.ws.Start()
	time.Sleep(time.Second / 2)

	waitForTxnSync(t, w.db.Txns())
	addr, err := w.DecodeAddress("1AhsMpyyyVyPZ9KDUgwsX3zTDJWWSsRo4f")
	if err != nil {
		t.Error(err)
	}

	// Test build normal tx
	tx, err := w.buildTx(1500000, addr, wallet.NORMAL, nil)
	if err != nil {
		t.Error(err)
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

func containsOutput(tx *wire.MsgTx, addr btcutil.Address) bool {
	for _, o := range tx.TxOut {
		script, _ := txscript.PayToAddrScript(addr)
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

func TestBitcoinWallet_GenerateMultisigScript(t *testing.T) {
	w, err := newMockWallet()
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
	key3, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}
	keys := []hdkeychain.ExtendedKey{*key1, *key2, *key3}

	// test without timeout
	addr, redeemScript, err := w.generateMultisigScript(keys, 2, 0, nil)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "bc1q7ckk79my7g0jltxtae34yk7e6nzth40dy6j6a67c96mhh6ue0hyqtmf66p" {
		t.Error("Returned invalid address")
	}

	rs := "52" + // OP_2
		"21" + // OP_PUSHDATA(33)
		"03c157f2a7c178430972263232c9306110090c50b44d4e906ecd6d377eec89a53c" + // pubkey1
		"21" + // OP_PUSHDATA(33)
		"0205b02b9dbe570f36d1c12e3100e55586b2b9dc61d6778c1d24a8eaca03625e7e" + // pubkey2
		"21" + // OP_PUSHDATA(33)
		"030c83b025cd6bdd8c06e93a2b953b821b4a8c29da211335048d7dc3389706d7e8" + // pubkey3
		"53" + // OP_3
		"ae" // OP_CHECKMULTISIG
	rsBytes, err := hex.DecodeString(rs)
	if !bytes.Equal(rsBytes, redeemScript) {
		t.Error("Returned invalid redeem script")
	}

	// test with timeout
	key4, err := w.km.GetFreshKey(wallet.INTERNAL)
	if err != nil {
		t.Error(err)
	}
	addr, redeemScript, err = w.generateMultisigScript(keys, 2, time.Hour*10, key4)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "bc1qlx7djex36u6ttf7kvqk0uzhvyu0ug3t695r4xjqz0s7pl4kkyzmqwxp2mc" {
		t.Error("Returned invalid address")
	}

	rs = "63" + // OP_IF
		"52" + // OP_2
		"21" + // OP_PUSHDATA(33)
		"03c157f2a7c178430972263232c9306110090c50b44d4e906ecd6d377eec89a53c" + // pubkey1
		"21" + // OP_PUSHDATA(33)
		"0205b02b9dbe570f36d1c12e3100e55586b2b9dc61d6778c1d24a8eaca03625e7e" + // pubkey2
		"21" + // OP_PUSHDATA(33)
		"030c83b025cd6bdd8c06e93a2b953b821b4a8c29da211335048d7dc3389706d7e8" + // pubkey3
		"53" + // OP_3
		"ae" + // OP_CHECKMULTISIG
		"67" + // OP_ELSE
		"01" + // OP_PUSHDATA(1)
		"3c" + // 60 blocks
		"b2" + // OP_CHECKSEQUENCEVERIFY
		"75" + // OP_DROP
		"21" + // OP_PUSHDATA(33)
		"02c2902e25457d7780471890b957fbbc3d80af94e3bba9a6b89fd28f618bf4147e" + // timeout pubkey
		"ac" + // OP_CHECKSIG
		"68" // OP_ENDIF
	rsBytes, err = hex.DecodeString(rs)
	if !bytes.Equal(rsBytes, redeemScript) {
		t.Error("Returned invalid redeem script")
	}
}

func TestBitcoinWallet_newUnsignedTransaction(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	w.ws.Start()
	waitForTxnSync(t, w.db.Txns())
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		t.Error(err)
	}
	addr, err := w.DecodeAddress("1AhsMpyyyVyPZ9KDUgwsX3zTDJWWSsRo4f")
	if err != nil {
		t.Error(err)
	}

	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		t.Error(err)
	}
	out := wire.NewTxOut(10000, script)
	outputs := []*wire.TxOut{out}

	changeSource := func() ([]byte, error) {
		addr := w.CurrentAddress(wallet.INTERNAL)
		script, err := txscript.PayToAddrScript(addr)
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

func TestBitcoinWallet_CreateMultisigSignature(t *testing.T) {
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

func buildTxData(w *BitcoinWallet) ([]wallet.TransactionInput, []wallet.TransactionOutput, []byte, error) {
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
	addr, err := w.DecodeAddress("1AhsMpyyyVyPZ9KDUgwsX3zTDJWWSsRo4f")
	if err != nil {
		return nil, nil, nil, err
	}

	out := wallet.TransactionOutput{
		Value:   20000,
		Address: addr,
	}
	return []wallet.TransactionInput{in1, in2}, []wallet.TransactionOutput{out}, redeemScriptBytes, nil
}

func TestBitcoinWallet_Multisign(t *testing.T) {
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
	txBytes, err := w.Multisign(ins, outs, sigs1, sigs2, redeemScript, 50, false)
	if err != nil {
		t.Error(err)
	}

	tx := wire.NewMsgTx(0)
	tx.BtcDecode(bytes.NewReader(txBytes), wire.ProtocolVersion, wire.WitnessEncoding)
	if len(tx.TxIn) != 2 {
		t.Error("Transactions has incorrect number of inputs")
	}
	if len(tx.TxOut) != 1 {
		t.Error("Transactions has incorrect number of outputs")
	}
	for _, in := range tx.TxIn {
		if len(in.Witness) == 0 {
			t.Error("Input witness has zero length")
		}
	}
}

func TestBitcoinWallet_bumpFee(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	w.ws.Start()
	waitForTxnSync(t, w.db.Txns())
	ch, err := chainhash.NewHashFromStr("ff2b865c3b73439912eebf4cce9a15b12c7d7bcdd14ae1110a90541426c4e7c5")
	if err != nil {
		t.Error(err)
	}
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		t.Error(err)
	}
	for _, u := range utxos {
		if u.Op.Hash.IsEqual(ch) {
			u.AtHeight = 0
			w.db.Utxos().Put(u)
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

func TestBitcoinWallet_sweepAddress(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	w.ws.Start()
	waitForTxnSync(t, w.db.Txns())
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

func TestBitcoinWallet_estimateSpendFee(t *testing.T) {
	w, err := newMockWallet()
	if err != nil {
		t.Error(err)
	}
	w.ws.Start()
	waitForTxnSync(t, w.db.Txns())
	fee, err := w.estimateSpendFee(1000, wallet.NORMAL)
	if err != nil {
		t.Error(err)
	}
	if fee <= 0 {
		t.Error("Returned incorrect fee")
	}
}
