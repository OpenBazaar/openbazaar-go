// Copyright (C) 2015-2016 The Lightning Network Developers
// Copyright (c) 2016-2017 The OpenBazaar Developers

package spvwallet

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	btc "github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/coinset"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcutil/txsort"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wallet/txrules"
)

func (s *SPVWallet) Broadcast(tx *wire.MsgTx) error {
	// Our own tx; don't keep track of false positives
	_, err := s.txstore.Ingest(tx, 0, time.Now())
	if err != nil {
		return err
	}

	log.Debugf("Broadcasting tx %s to peers", tx.TxHash().String())
	s.wireService.MsgChan() <- updateFiltersMsg{}
	for _, peer := range s.peerManager.ConnectedPeers() {
		peer.QueueMessageWithEncoding(tx, nil, wire.WitnessEncoding)
	}
	return nil
}

type Coin struct {
	TxHash       *chainhash.Hash
	TxIndex      uint32
	TxValue      btc.Amount
	TxNumConfs   int64
	ScriptPubKey []byte
}

func (c *Coin) Hash() *chainhash.Hash { return c.TxHash }
func (c *Coin) Index() uint32         { return c.TxIndex }
func (c *Coin) Value() btc.Amount     { return c.TxValue }
func (c *Coin) PkScript() []byte      { return c.ScriptPubKey }
func (c *Coin) NumConfs() int64       { return c.TxNumConfs }
func (c *Coin) ValueAge() int64       { return int64(c.TxValue) * c.TxNumConfs }

func newCoin(txid []byte, index uint32, value string, numConfs int64, scriptPubKey []byte) (coinset.Coin, error) {
	iVal, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("coin value (%s) is invalid", value)
	}
	shaTxid, _ := chainhash.NewHash(txid)
	c := &Coin{
		TxHash:       shaTxid,
		TxIndex:      index,
		TxValue:      btc.Amount(iVal),
		TxNumConfs:   numConfs,
		ScriptPubKey: scriptPubKey,
	}
	return coinset.Coin(c), nil
}

func (w *SPVWallet) gatherCoins() map[coinset.Coin]*hd.ExtendedKey {
	height, _ := w.blockchain.db.Height()
	utxos, _ := w.txstore.Utxos().GetAll()
	m := make(map[coinset.Coin]*hd.ExtendedKey)
	for _, u := range utxos {
		if u.WatchOnly {
			continue
		}
		var confirmations int32
		if u.AtHeight > 0 {
			confirmations = int32(height) - u.AtHeight
		}
		c, err := newCoin(u.Op.Hash.CloneBytes(), u.Op.Index, u.Value, int64(confirmations), u.ScriptPubkey)
		if err != nil {
			continue
		}
		addr, err := w.ScriptToAddress(u.ScriptPubkey)
		if err != nil {
			continue
		}
		key, err := w.keyManager.GetKeyForScript(addr.ScriptAddress())
		if err != nil {
			continue
		}
		m[c] = key
	}
	return m
}

func (w *SPVWallet) Spend(amount big.Int, addr btc.Address, fee wallet.Fee, referenceID string, spendAll bool) (*chainhash.Hash, error) {
	var (
		tx  *wire.MsgTx
		err error
	)
	if spendAll {
		tx, err = w.buildSpendAllTx(addr, fee)
		if err != nil {
			return nil, err
		}
	} else {
		tx, err = w.buildTx(amount.Int64(), addr, fee, nil)
		if err != nil {
			return nil, err
		}
	}

	if err := w.Broadcast(tx); err != nil {
		return nil, err
	}
	ch := tx.TxHash()
	return &ch, nil
}

var BumpFeeAlreadyConfirmedError = errors.New("Transaction is confirmed, cannot bump fee")
var BumpFeeTransactionDeadError = errors.New("Cannot bump fee of dead transaction")
var BumpFeeNotFoundError = errors.New("Transaction either doesn't exist or has already been spent")

func (w *SPVWallet) BumpFee(txid chainhash.Hash) (*chainhash.Hash, error) {
	txn, err := w.txstore.Txns().Get(txid)
	if err != nil {
		return nil, err
	}
	if txn.Height > 0 {
		return nil, BumpFeeAlreadyConfirmedError
	}
	if txn.Height < 0 {
		return nil, BumpFeeTransactionDeadError
	}
	// Check stxos for RBF opportunity
	/*stxos, _ := w.txstore.Stxos().GetAll()
	for _, s := range stxos {
		if s.SpendTxid.IsEqual(&txid) {
			r := bytes.NewReader(txn.Bytes)
			msgTx := wire.NewMsgTx(1)
			msgTx.BtcDecode(r, 1)
			for i, output := range msgTx.TxOut {
				key, err := w.txstore.GetKeyForScript(output.PkScript)
				if key != nil && err == nil { // This is our change output
					// Calculate change - additional fee
					feePerByte := w.GetFeePerByte(PRIORITY)
					estimatedSize := EstimateSerializeSize(len(msgTx.TxIn), msgTx.TxOut, false)
					fee := estimatedSize * int(feePerByte)
					newValue := output.Value - int64(fee)

					// Check if still above dust value
					if newValue <= 0 || txrules.IsDustAmount(btc.Amount(newValue), len(output.PkScript), txrules.DefaultRelayFeePerKb) {
						msgTx.TxOut = append(msgTx.TxOut[:i], msgTx.TxOut[i+1:]...)
					} else {
						output.Value = newValue
					}

					// Bump sequence number
					optInRBF := false
					for _, input := range msgTx.TxIn {
						if input.Sequence < 4294967294 {
							input.Sequence++
							optInRBF = true
						}
					}
					if !optInRBF {
						break
					}

					//TODO: Re-sign transaction

					// Mark original tx as dead
					if err = w.txstore.markAsDead(txid); err != nil {
						return nil, err
					}

					// Broadcast new tx
					if err := w.Broadcast(msgTx); err != nil {
						return nil, err
					}
					newTxid := msgTx.TxHash()
					return &newTxid, nil
				}
			}
		}
	}*/
	// Check utxos for CPFP
	utxos, _ := w.txstore.Utxos().GetAll()
	for _, u := range utxos {
		if u.Op.Hash.IsEqual(&txid) && u.AtHeight == 0 {
			addr, err := w.ScriptToAddress(u.ScriptPubkey)
			if err != nil {
				return nil, err
			}
			key, err := w.keyManager.GetKeyForScript(addr.ScriptAddress())
			if err != nil {
				return nil, err
			}
			h, err := hex.DecodeString(u.Op.Hash.String())
			if err != nil {
				return nil, err
			}
			val0, _ := new(big.Int).SetString(u.Value, 10)
			in := wallet.TransactionInput{
				LinkedAddress: addr,
				OutpointIndex: u.Op.Index,
				OutpointHash:  h,
				Value:         *val0,
			}
			transactionID, err := w.SweepAddress([]wallet.TransactionInput{in}, nil, key, nil, wallet.FEE_BUMP)
			if err != nil {
				return nil, err
			}
			return transactionID, nil
		}
	}
	return nil, BumpFeeNotFoundError
}

func (w *SPVWallet) EstimateFee(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, feePerByte big.Int) big.Int {
	tx := new(wire.MsgTx)
	for _, out := range outs {
		scriptPubKey, _ := txscript.PayToAddrScript(out.Address)
		output := wire.NewTxOut(out.Value.Int64(), scriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false, P2PKH)
	fee := estimatedSize * int(feePerByte.Uint64())
	return *big.NewInt(int64(fee))
}

// Build a spend transaction for the amount and return the transaction fee
func (w *SPVWallet) EstimateSpendFee(amount big.Int, feeLevel wallet.FeeLevel) (big.Int, error) {
	// Since this is an estimate we can use a dummy output address. Let's use a long one so we don't under estimate.
	addr, err := btc.DecodeAddress("bc1qxtq7ha2l5qg70atpwp3fus84fx3w0v2w4r2my7gt89ll3w0vnlgspu349h", w.params)
	if err != nil {
		return *big.NewInt(0), err
	}
	fee := wallet.Fee{
		FeeLevel: feeLevel,
	}
	tx, err := w.buildTx(amount.Int64(), addr, fee, nil)
	if err != nil {
		return *big.NewInt(0), err
	}
	var outval int64
	for _, output := range tx.TxOut {
		outval += output.Value
	}
	var inval int64
	utxos, err := w.txstore.Utxos().GetAll()
	if err != nil {
		return *big.NewInt(0), err
	}
	for _, input := range tx.TxIn {
		for _, utxo := range utxos {
			if utxo.Op.Hash.IsEqual(&input.PreviousOutPoint.Hash) && utxo.Op.Index == input.PreviousOutPoint.Index {
				val0, _ := new(big.Int).SetString(utxo.Value, 10)
				inval += val0.Int64()
				break
			}
		}
	}
	if inval < outval {
		return *big.NewInt(0), errors.New("Error building transaction: inputs less than outputs")
	}
	return *big.NewInt(inval - outval), err
}

func (w *SPVWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (addr btc.Address, redeemScript []byte, err error) {
	if uint32(timeout.Hours()) > 0 && timeoutKey == nil {
		return nil, nil, errors.New("Timeout key must be non nil when using an escrow timeout")
	}

	if len(keys) < threshold {
		return nil, nil, fmt.Errorf("unable to generate multisig script with "+
			"%d required signatures when there are only %d public "+
			"keys available", threshold, len(keys))
	}

	var ecKeys []*btcec.PublicKey
	for _, key := range keys {
		ecKey, err := key.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		ecKeys = append(ecKeys, ecKey)
	}

	builder := txscript.NewScriptBuilder()
	if uint32(timeout.Hours()) == 0 {

		builder.AddInt64(int64(threshold))
		for _, key := range ecKeys {
			builder.AddData(key.SerializeCompressed())
		}
		builder.AddInt64(int64(len(ecKeys)))
		builder.AddOp(txscript.OP_CHECKMULTISIG)

	} else {
		ecKey, err := timeoutKey.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		sequenceLock := blockchain.LockTimeToSequence(false, uint32(timeout.Hours()*6))
		builder.AddOp(txscript.OP_IF)
		builder.AddInt64(int64(threshold))
		for _, key := range ecKeys {
			builder.AddData(key.SerializeCompressed())
		}
		builder.AddInt64(int64(len(ecKeys)))
		builder.AddOp(txscript.OP_CHECKMULTISIG)
		builder.AddOp(txscript.OP_ELSE).
			AddInt64(int64(sequenceLock)).
			AddOp(txscript.OP_CHECKSEQUENCEVERIFY).
			AddOp(txscript.OP_DROP).
			AddData(ecKey.SerializeCompressed()).
			AddOp(txscript.OP_CHECKSIG).
			AddOp(txscript.OP_ENDIF)
	}
	redeemScript, err = builder.Script()
	if err != nil {
		return nil, nil, err
	}

	witnessProgram := sha256.Sum256(redeemScript)

	addr, err = btc.NewAddressWitnessScriptHash(witnessProgram[:], w.params)
	if err != nil {
		return nil, nil, err
	}
	return addr, redeemScript, nil
}

func (w *SPVWallet) CreateMultisigSignature(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte big.Int) ([]wallet.Signature, error) {
	var sigs []wallet.Signature
	tx := wire.NewMsgTx(1)
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return sigs, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		scriptPubKey, err := txscript.PayToAddrScript(out.Address)
		if err != nil {
			return sigs, err
		}
		output := wire.NewTxOut(out.Value.Int64(), scriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	txType := P2SH_2of3_Multisig
	_, err := LockTimeFromRedeemScript(redeemScript)
	if err == nil {
		txType = P2SH_Multisig_Timelock_2Sigs
	}
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false, txType)
	fee := estimatedSize * int(feePerByte.Uint64())
	if len(tx.TxOut) > 0 {
		feePerOutput := fee / len(tx.TxOut)
		for _, output := range tx.TxOut {
			output.Value -= int64(feePerOutput)
		}
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	signingKey, err := key.ECPrivKey()
	if err != nil {
		return sigs, err
	}

	hashes := txscript.NewTxSigHashes(tx)
	for i := range tx.TxIn {
		sig, err := txscript.RawTxInWitnessSignature(tx, hashes, i, ins[i].Value.Int64(), redeemScript, txscript.SigHashAll, signingKey)
		if err != nil {
			continue
		}
		bs := wallet.Signature{InputIndex: uint32(i), Signature: sig}
		sigs = append(sigs, bs)
	}
	return sigs, nil
}

func (w *SPVWallet) Multisign(ins []wallet.TransactionInput, outs []wallet.TransactionOutput, sigs1 []wallet.Signature, sigs2 []wallet.Signature, redeemScript []byte, feePerByte big.Int, broadcast bool) ([]byte, error) {
	tx := wire.NewMsgTx(1)
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return nil, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		scriptPubKey, err := txscript.PayToAddrScript(out.Address)
		if err != nil {
			return nil, err
		}
		output := wire.NewTxOut(out.Value.Int64(), scriptPubKey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	txType := P2SH_2of3_Multisig
	_, err := LockTimeFromRedeemScript(redeemScript)
	if err == nil {
		txType = P2SH_Multisig_Timelock_2Sigs
	}
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false, txType)
	fee := estimatedSize * int(feePerByte.Uint64())
	if len(tx.TxOut) > 0 {
		feePerOutput := fee / len(tx.TxOut)
		for _, output := range tx.TxOut {
			output.Value -= int64(feePerOutput)
		}
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	// Check if time locked
	var timeLocked bool
	if redeemScript[0] == txscript.OP_IF {
		timeLocked = true
	}

	for i, input := range tx.TxIn {
		var sig1 []byte
		var sig2 []byte
		for _, sig := range sigs1 {
			if int(sig.InputIndex) == i {
				sig1 = sig.Signature
				break
			}
		}
		for _, sig := range sigs2 {
			if int(sig.InputIndex) == i {
				sig2 = sig.Signature
				break
			}
		}

		witness := wire.TxWitness{[]byte{}, sig1, sig2}

		if timeLocked {
			witness = append(witness, []byte{0x01})
		}
		witness = append(witness, redeemScript)
		input.Witness = witness
	}
	// broadcast
	if broadcast {
		w.Broadcast(tx)
	}
	var buf bytes.Buffer
	tx.BtcEncode(&buf, wire.ProtocolVersion, wire.WitnessEncoding)
	return buf.Bytes(), nil
}

func (w *SPVWallet) SweepAddress(ins []wallet.TransactionInput, address *btc.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wallet.FeeLevel) (*chainhash.Hash, error) {
	var internalAddr btc.Address
	if address != nil {
		internalAddr = *address
	} else {
		internalAddr = w.CurrentAddress(wallet.INTERNAL)
	}
	script, err := txscript.PayToAddrScript(internalAddr)
	if err != nil {
		return nil, err
	}

	var val int64
	var inputs []*wire.TxIn
	additionalPrevScripts := make(map[wire.OutPoint][]byte)
	for _, in := range ins {
		val += in.Value.Int64()
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return nil, err
		}
		script, err := txscript.PayToAddrScript(in.LinkedAddress)
		if err != nil {
			return nil, err
		}
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
		inputs = append(inputs, input)
		additionalPrevScripts[*outpoint] = script
	}
	out := wire.NewTxOut(val, script)

	txType := P2PKH
	if redeemScript != nil {
		txType = P2SH_1of2_Multisig
		_, err := LockTimeFromRedeemScript(*redeemScript)
		if err == nil {
			txType = P2SH_Multisig_Timelock_1Sig
		}
	}
	estimatedSize := EstimateSerializeSize(len(ins), []*wire.TxOut{out}, false, txType)

	// Calculate the fee
	f := w.GetFeePerByte(feeLevel)
	feePerByte := int(f.Int64())
	fee := estimatedSize * feePerByte

	outVal := val - int64(fee)
	if outVal < 0 {
		outVal = 0
	}
	out.Value = outVal

	tx := &wire.MsgTx{
		Version:  wire.TxVersion,
		TxIn:     inputs,
		TxOut:    []*wire.TxOut{out},
		LockTime: 0,
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	// Sign tx
	privKey, err := key.ECPrivKey()
	if err != nil {
		return nil, err
	}
	pk := privKey.PubKey().SerializeCompressed()
	addressPub, err := btc.NewAddressPubKey(pk, w.params)

	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		if addressPub.EncodeAddress() == addr.EncodeAddress() {
			wif, err := btc.NewWIF(privKey, w.params, true)
			if err != nil {
				return nil, false, err
			}
			return wif.PrivKey, wif.CompressPubKey, nil
		}
		return nil, false, errors.New("Not found")
	})
	getScript := txscript.ScriptClosure(func(addr btc.Address) ([]byte, error) {
		if redeemScript == nil {
			return []byte{}, nil
		}
		return *redeemScript, nil
	})

	// Check if time locked
	var timeLocked bool
	if redeemScript != nil {
		rs := *redeemScript
		if rs[0] == txscript.OP_IF {
			timeLocked = true
			tx.Version = 2
			for _, txIn := range tx.TxIn {
				locktime, err := LockTimeFromRedeemScript(*redeemScript)
				if err != nil {
					return nil, err
				}
				txIn.Sequence = locktime
			}
		}
	}

	hashes := txscript.NewTxSigHashes(tx)
	for i, txIn := range tx.TxIn {
		if redeemScript == nil {
			prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
			script, err := txscript.SignTxOutput(w.params,
				tx, i, prevOutScript, txscript.SigHashAll, getKey,
				getScript, txIn.SignatureScript)
			if err != nil {
				return nil, errors.New("Failed to sign transaction")
			}
			txIn.SignatureScript = script
		} else {
			sig, err := txscript.RawTxInWitnessSignature(tx, hashes, i, ins[i].Value.Int64(), *redeemScript, txscript.SigHashAll, privKey)
			if err != nil {
				return nil, err
			}
			var witness wire.TxWitness
			if timeLocked {
				witness = wire.TxWitness{sig, []byte{}}
			} else {
				witness = wire.TxWitness{[]byte{}, sig}
			}
			witness = append(witness, *redeemScript)
			txIn.Witness = witness
		}
	}

	// broadcast
	w.Broadcast(tx)
	txid := tx.TxHash()
	return &txid, nil
}

func (w *SPVWallet) buildTx(amount int64, addr btc.Address, fee wallet.Fee, optionalOutput *wire.TxOut) (*wire.MsgTx, error) {
	// Check for dust
	script, _ := txscript.PayToAddrScript(addr)
	if isTxSizeDust(*big.NewInt(amount), len(script)) {
		return nil, wallet.ErrorDustAmount
	}

	var additionalPrevScripts map[wire.OutPoint][]byte
	var additionalKeysByAddress map[string]*btc.WIF

	// Create input source
	coinMap := w.gatherCoins()
	coins := make([]coinset.Coin, 0, len(coinMap))
	for k := range coinMap {
		coins = append(coins, k)
	}

	inputSource := func(target btc.Amount) (total btc.Amount, inputs []*wire.TxIn, inputValues []btc.Amount, scripts [][]byte, err error) {
		coinSelector := coinset.MaxValueAgeCoinSelector{MaxInputs: 10000, MinChangeAmount: btc.Amount(0)}
		coins, err := coinSelector.CoinSelect(target, coins)
		if err != nil {
			return total, inputs, []btc.Amount{}, scripts, wallet.ErrInsufficientFunds
		}
		additionalPrevScripts = make(map[wire.OutPoint][]byte)
		additionalKeysByAddress = make(map[string]*btc.WIF)
		for _, c := range coins.Coins() {
			total += c.Value()
			outpoint := wire.NewOutPoint(c.Hash(), c.Index())
			in := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
			in.Sequence = 0 // Opt-in RBF so we can bump fees
			inputs = append(inputs, in)
			additionalPrevScripts[*outpoint] = c.PkScript()
			key := coinMap[c]
			addr, err := key.Address(w.params)
			if err != nil {
				continue
			}
			privKey, err := key.ECPrivKey()
			if err != nil {
				continue
			}
			wif, _ := btc.NewWIF(privKey, w.params, true)
			additionalKeysByAddress[addr.EncodeAddress()] = wif
		}
		return total, inputs, []btc.Amount{}, scripts, nil
	}

	// Get the fee per kilobyte
	var feePerKB int64
	if fee.CustomFee != "" {
		customFee, err := strconv.Atoi(fee.CustomFee)
		if err != nil {
			return nil, err
		}
		feePerKB = int64(customFee * 1000)
	} else {
		f := w.GetFeePerByte(fee.FeeLevel)
		feePerKB = f.Int64() * 1000
	}

	// outputs
	out := wire.NewTxOut(amount, script)

	// Create change source
	changeSource := func() ([]byte, error) {
		addr := w.CurrentAddress(wallet.INTERNAL)
		script, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return []byte{}, err
		}
		return script, nil
	}

	outputs := []*wire.TxOut{out}
	if optionalOutput != nil {
		outputs = append(outputs, optionalOutput)
	}
	authoredTx, err := NewUnsignedTransaction(outputs, btc.Amount(feePerKB), inputSource, changeSource)
	if err != nil {
		return nil, err
	}

	// BIP 69 sorting
	txsort.InPlaceSort(authoredTx.Tx)

	// Sign tx
	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		addrStr := addr.EncodeAddress()
		wif := additionalKeysByAddress[addrStr]
		return wif.PrivKey, wif.CompressPubKey, nil
	})
	getScript := txscript.ScriptClosure(func(
		addr btc.Address) ([]byte, error) {
		return []byte{}, nil
	})
	for i, txIn := range authoredTx.Tx.TxIn {
		prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
		script, err := txscript.SignTxOutput(w.params,
			authoredTx.Tx, i, prevOutScript, txscript.SigHashAll, getKey,
			getScript, txIn.SignatureScript)
		if err != nil {
			return nil, errors.New("Failed to sign transaction")
		}
		txIn.SignatureScript = script
	}
	return authoredTx.Tx, nil
}

func (w *SPVWallet) buildSpendAllTx(addr btc.Address, fee wallet.Fee) (*wire.MsgTx, error) {
	tx := wire.NewMsgTx(1)

	coinMap := w.gatherCoins()
	inVals := make(map[wire.OutPoint]int64)
	totalIn := int64(0)
	additionalPrevScripts := make(map[wire.OutPoint][]byte)
	additionalKeysByAddress := make(map[string]*btc.WIF)

	for coin, key := range coinMap {
		outpoint := wire.NewOutPoint(coin.Hash(), coin.Index())
		in := wire.NewTxIn(outpoint, nil, nil)
		additionalPrevScripts[*outpoint] = coin.PkScript()
		tx.TxIn = append(tx.TxIn, in)
		val := int64(coin.Value().ToUnit(btc.AmountSatoshi))
		totalIn += val
		inVals[*outpoint] = val

		addr, err := key.Address(w.params)
		if err != nil {
			continue
		}
		privKey, err := key.ECPrivKey()
		if err != nil {
			continue
		}
		wif, _ := btc.NewWIF(privKey, w.params, true)
		additionalKeysByAddress[addr.EncodeAddress()] = wif
	}

	// outputs
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	// Get the fee
	var feePerByte int64
	if fee.CustomFee != "" {
		cf, err := strconv.Atoi(fee.CustomFee)
		if err != nil {
			return nil, err
		}
		feePerByte = int64(cf)
	} else {
		f := w.GetFeePerByte(fee.FeeLevel)
		feePerByte = f.Int64()
	}

	estimatedSize := EstimateSerializeSize(1, []*wire.TxOut{wire.NewTxOut(0, script)}, false, P2PKH)
	feeAmount := int64(estimatedSize) * feePerByte

	// Check for dust output
	if isTxSizeDust(*big.NewInt(totalIn - feeAmount), len(script)) {
		return nil, wallet.ErrorDustAmount
	}

	// Build the output
	out := wire.NewTxOut(totalIn-feeAmount, script)
	tx.TxOut = append(tx.TxOut, out)

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

	// Sign
	getKey := txscript.KeyClosure(func(addr btc.Address) (*btcec.PrivateKey, bool, error) {
		addrStr := addr.EncodeAddress()
		wif, ok := additionalKeysByAddress[addrStr]
		if !ok {
			return nil, false, errors.New("key not found")
		}
		return wif.PrivKey, wif.CompressPubKey, nil
	})
	getScript := txscript.ScriptClosure(func(
		addr btc.Address) ([]byte, error) {
		return []byte{}, nil
	})
	for i, txIn := range tx.TxIn {
		prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
		script, err := txscript.SignTxOutput(w.params,
			tx, i, prevOutScript, txscript.SigHashAll, getKey,
			getScript, txIn.SignatureScript)
		if err != nil {
			return nil, errors.New("failed to sign transaction")
		}
		txIn.SignatureScript = script
	}
	return tx, nil
}

func NewUnsignedTransaction(outputs []*wire.TxOut, feePerKb btc.Amount, fetchInputs txauthor.InputSource, fetchChange txauthor.ChangeSource) (*txauthor.AuthoredTx, error) {

	var targetAmount btc.Amount
	for _, txOut := range outputs {
		targetAmount += btc.Amount(txOut.Value)
	}

	estimatedSize := EstimateSerializeSize(1, outputs, true, P2PKH)
	targetFee := txrules.FeeForSerializeSize(feePerKb, estimatedSize)

	for {
		inputAmount, inputs, _, scripts, err := fetchInputs(targetAmount + targetFee)
		if err != nil {
			return nil, err
		}
		if inputAmount < targetAmount+targetFee {
			return nil, errors.New("insufficient funds available to construct transaction")
		}

		maxSignedSize := EstimateSerializeSize(len(inputs), outputs, true, P2PKH)
		maxRequiredFee := txrules.FeeForSerializeSize(feePerKb, maxSignedSize)
		remainingAmount := inputAmount - targetAmount
		if remainingAmount < maxRequiredFee {
			targetFee = maxRequiredFee
			continue
		}

		unsignedTransaction := &wire.MsgTx{
			Version:  wire.TxVersion,
			TxIn:     inputs,
			TxOut:    outputs,
			LockTime: 0,
		}
		changeIndex := -1
		changeAmount := inputAmount - targetAmount - maxRequiredFee
		if changeAmount != 0 && !isTxSizeDust(*big.NewInt(int64(changeAmount)), P2PKHOutputSize) {
			changeScript, err := fetchChange()
			if err != nil {
				return nil, err
			}
			if len(changeScript) > P2PKHPkScriptSize {
				return nil, errors.New("fee estimation requires change " +
					"scripts no larger than P2PKH output scripts")
			}
			change := wire.NewTxOut(int64(changeAmount), changeScript)
			l := len(outputs)
			unsignedTransaction.TxOut = append(outputs[:l:l], change)
			changeIndex = l
		}

		return &txauthor.AuthoredTx{
			Tx:          unsignedTransaction,
			PrevScripts: scripts,
			TotalInput:  inputAmount,
			ChangeIndex: changeIndex,
		}, nil
	}
}

func (w *SPVWallet) GetFeePerByte(feeLevel wallet.FeeLevel) big.Int {
	return *big.NewInt(int64(w.feeProvider.GetFeePerByte(feeLevel)))
}

func LockTimeFromRedeemScript(redeemScript []byte) (uint32, error) {
	if len(redeemScript) < 113 {
		return 0, errors.New("Redeem script invalid length")
	}
	if redeemScript[106] != 103 {
		return 0, errors.New("Invalid redeem script")
	}
	if redeemScript[107] == 0 {
		return 0, nil
	}
	if 81 <= redeemScript[107] && redeemScript[107] <= 96 {
		return uint32((redeemScript[107] - 81) + 1), nil
	}
	var v []byte
	op := redeemScript[107]
	if 1 <= op && op <= 75 {
		for i := 0; i < int(op); i++ {
			v = append(v, []byte{redeemScript[108+i]}...)
		}
	} else {
		return 0, errors.New("Too many bytes pushed for sequence")
	}
	var result int64
	for i, val := range v {
		result |= int64(val) << uint8(8*i)
	}

	return uint32(result), nil
}
