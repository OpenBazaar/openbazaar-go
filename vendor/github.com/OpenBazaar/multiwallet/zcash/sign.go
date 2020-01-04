package zcash

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/minio/blake2b-simd"
	"time"

	"github.com/OpenBazaar/spvwallet"
	wi "github.com/OpenBazaar/wallet-interface"
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

	"github.com/OpenBazaar/multiwallet/util"
	zaddr "github.com/OpenBazaar/multiwallet/zcash/address"
)

var (
	txHeaderBytes          = []byte{0x04, 0x00, 0x00, 0x80}
	txNVersionGroupIDBytes = []byte{0x85, 0x20, 0x2f, 0x89}

	hashPrevOutPersonalization  = []byte("ZcashPrevoutHash")
	hashSequencePersonalization = []byte("ZcashSequencHash")
	hashOutputsPersonalization  = []byte("ZcashOutputsHash")
	sigHashPersonalization      = []byte("ZcashSigHash")
)

const (
	sigHashMask = 0x1f
	branchID    = 0x2BB40E60
)

func (w *ZCashWallet) buildTx(amount int64, addr btc.Address, feeLevel wi.FeeLevel, optionalOutput *wire.TxOut) (*wire.MsgTx, error) {
	// Check for dust
	script, err := zaddr.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}
	if txrules.IsDustAmount(btc.Amount(amount), len(script), txrules.DefaultRelayFeePerKb) {
		return nil, wi.ErrorDustAmount
	}

	var (
		additionalPrevScripts   map[wire.OutPoint][]byte
		additionalKeysByAddress map[string]*btc.WIF
		inVals                  map[wire.OutPoint]btc.Amount
	)

	// Create input source
	height, _ := w.ws.ChainTip()
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		return nil, err
	}
	coinMap := util.GatherCoins(height, utxos, w.ScriptToAddress, w.km.GetKeyForScript)

	coins := make([]coinset.Coin, 0, len(coinMap))
	for k := range coinMap {
		coins = append(coins, k)
	}
	inputSource := func(target btc.Amount) (total btc.Amount, inputs []*wire.TxIn, inputValues []btc.Amount, scripts [][]byte, err error) {
		coinSelector := coinset.MaxValueAgeCoinSelector{MaxInputs: 10000, MinChangeAmount: btc.Amount(0)}
		coins, err := coinSelector.CoinSelect(target, coins)
		if err != nil {
			return total, inputs, inputValues, scripts, wi.ErrorInsuffientFunds
		}
		additionalPrevScripts = make(map[wire.OutPoint][]byte)
		additionalKeysByAddress = make(map[string]*btc.WIF)
		inVals = make(map[wire.OutPoint]btc.Amount)
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
			inVals[*outpoint] = c.Value()
		}
		return total, inputs, inputValues, scripts, nil
	}

	// Get the fee per kilobyte
	feePerKB := int64(w.GetFeePerByte(feeLevel)) * 1000

	// outputs
	out := wire.NewTxOut(amount, script)

	// Create change source
	changeSource := func() ([]byte, error) {
		addr := w.CurrentAddress(wi.INTERNAL)
		script, err := zaddr.PayToAddrScript(addr)
		if err != nil {
			return []byte{}, err
		}
		return script, nil
	}

	outputs := []*wire.TxOut{out}
	if optionalOutput != nil {
		outputs = append(outputs, optionalOutput)
	}
	authoredTx, err := newUnsignedTransaction(outputs, btc.Amount(feePerKB), inputSource, changeSource)
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
	for i, txIn := range authoredTx.Tx.TxIn {
		prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(prevOutScript, w.params)
		if err != nil {
			return nil, err
		}
		key, _, err := getKey(addrs[0])
		if err != nil {
			return nil, err
		}
		val := int64(inVals[txIn.PreviousOutPoint].ToUnit(btc.AmountSatoshi))
		sig, err := rawTxInSignature(authoredTx.Tx, i, prevOutScript, txscript.SigHashAll, key, val)
		if err != nil {
			return nil, errors.New("failed to sign transaction")
		}
		builder := txscript.NewScriptBuilder()
		builder.AddData(sig)
		builder.AddData(key.PubKey().SerializeCompressed())
		script, err := builder.Script()
		if err != nil {
			return nil, err
		}
		txIn.SignatureScript = script
	}
	return authoredTx.Tx, nil
}

func (w *ZCashWallet) buildSpendAllTx(addr btc.Address, feeLevel wi.FeeLevel) (*wire.MsgTx, error) {
	tx := wire.NewMsgTx(1)

	height, _ := w.ws.ChainTip()
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		return nil, err
	}
	coinMap := util.GatherCoins(height, utxos, w.ScriptToAddress, w.km.GetKeyForScript)

	totalIn, inVals, additionalPrevScripts, additionalKeysByAddress := util.LoadAllInputs(tx, coinMap, w.params)

	// outputs
	script, err := zaddr.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	// Get the fee
	feePerByte := int64(w.GetFeePerByte(feeLevel))
	estimatedSize := EstimateSerializeSize(1, []*wire.TxOut{wire.NewTxOut(0, script)}, false, P2PKH)
	fee := int64(estimatedSize) * feePerByte

	// Check for dust output
	if txrules.IsDustAmount(btc.Amount(totalIn-fee), len(script), txrules.DefaultRelayFeePerKb) {
		return nil, wi.ErrorDustAmount
	}

	// Build the output
	out := wire.NewTxOut(totalIn-fee, script)
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
	for i, txIn := range tx.TxIn {
		prevOutScript := additionalPrevScripts[txIn.PreviousOutPoint]
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(prevOutScript, w.params)
		if err != nil {
			return nil, err
		}
		key, _, err := getKey(addrs[0])
		if err != nil {
			return nil, err
		}
		sig, err := rawTxInSignature(tx, i, prevOutScript, txscript.SigHashAll, key, inVals[txIn.PreviousOutPoint])
		if err != nil {
			return nil, errors.New("failed to sign transaction")
		}
		builder := txscript.NewScriptBuilder()
		builder.AddData(sig)
		builder.AddData(key.PubKey().SerializeCompressed())
		script, err := builder.Script()
		if err != nil {
			return nil, err
		}
		txIn.SignatureScript = script
	}
	return tx, nil
}

func newUnsignedTransaction(outputs []*wire.TxOut, feePerKb btc.Amount, fetchInputs txauthor.InputSource, fetchChange txauthor.ChangeSource) (*txauthor.AuthoredTx, error) {

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
		if changeAmount != 0 && !txrules.IsDustAmount(changeAmount,
			P2PKHOutputSize, txrules.DefaultRelayFeePerKb) {
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

func (w *ZCashWallet) bumpFee(txid chainhash.Hash) (*chainhash.Hash, error) {
	txn, err := w.db.Txns().Get(txid)
	if err != nil {
		return nil, err
	}
	if txn.Height > 0 {
		return nil, spvwallet.BumpFeeAlreadyConfirmedError
	}
	if txn.Height < 0 {
		return nil, spvwallet.BumpFeeTransactionDeadError
	}
	// Check utxos for CPFP
	utxos, _ := w.db.Utxos().GetAll()
	for _, u := range utxos {
		if u.Op.Hash.IsEqual(&txid) && u.AtHeight == 0 {
			addr, err := w.ScriptToAddress(u.ScriptPubkey)
			if err != nil {
				return nil, err
			}
			key, err := w.km.GetKeyForScript(addr.ScriptAddress())
			if err != nil {
				return nil, err
			}
			h, err := hex.DecodeString(u.Op.Hash.String())
			if err != nil {
				return nil, err
			}
			in := wi.TransactionInput{
				LinkedAddress: addr,
				OutpointIndex: u.Op.Index,
				OutpointHash:  h,
				Value:         int64(u.Value),
			}
			transactionID, err := w.sweepAddress([]wi.TransactionInput{in}, nil, key, nil, wi.FEE_BUMP)
			if err != nil {
				return nil, err
			}
			return transactionID, nil
		}
	}
	return nil, spvwallet.BumpFeeNotFoundError
}

func (w *ZCashWallet) sweepAddress(ins []wi.TransactionInput, address *btc.Address, key *hd.ExtendedKey, redeemScript *[]byte, feeLevel wi.FeeLevel) (*chainhash.Hash, error) {
	var internalAddr btc.Address
	if address != nil {
		internalAddr = *address
	} else {
		internalAddr = w.CurrentAddress(wi.INTERNAL)
	}
	script, err := zaddr.PayToAddrScript(internalAddr)
	if err != nil {
		return nil, err
	}

	var val int64
	var inputs []*wire.TxIn
	additionalPrevScripts := make(map[wire.OutPoint][]byte)
	var values []int64
	for _, in := range ins {
		val += in.Value
		values = append(values, in.Value)
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return nil, err
		}
		script, err := zaddr.PayToAddrScript(in.LinkedAddress)
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
		_, err := spvwallet.LockTimeFromRedeemScript(*redeemScript)
		if err == nil {
			txType = P2SH_Multisig_Timelock_1Sig
		}
	} else {
		redeemScript = &[]byte{}
	}
	estimatedSize := EstimateSerializeSize(len(ins), []*wire.TxOut{out}, false, txType)

	// Calculate the fee
	feePerByte := int(w.GetFeePerByte(feeLevel))
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

	for i, txIn := range tx.TxIn {
		sig, err := rawTxInSignature(tx, i, *redeemScript, txscript.SigHashAll, privKey, values[i])
		if err != nil {
			return nil, errors.New("failed to sign transaction")
		}
		builder := txscript.NewScriptBuilder()
		builder.AddOp(txscript.OP_0)
		builder.AddData(sig)
		if redeemScript != nil {
			builder.AddData(*redeemScript)
		}
		script, err := builder.Script()
		if err != nil {
			return nil, err
		}
		txIn.SignatureScript = script
	}

	// broadcast
	txid, err := w.Broadcast(tx)
	if err != nil {
		return nil, err
	}
	return chainhash.NewHashFromStr(txid)
}

func (w *ZCashWallet) createMultisigSignature(ins []wi.TransactionInput, outs []wi.TransactionOutput, key *hd.ExtendedKey, redeemScript []byte, feePerByte uint64) ([]wi.Signature, error) {
	var sigs []wi.Signature
	tx := wire.NewMsgTx(1)
	var values []int64
	for _, in := range ins {
		ch, err := chainhash.NewHashFromStr(hex.EncodeToString(in.OutpointHash))
		if err != nil {
			return sigs, err
		}
		values = append(values, in.Value)
		outpoint := wire.NewOutPoint(ch, in.OutpointIndex)
		input := wire.NewTxIn(outpoint, []byte{}, [][]byte{})
		tx.TxIn = append(tx.TxIn, input)
	}
	for _, out := range outs {
		scriptPubkey, err := zaddr.PayToAddrScript(out.Address)
		if err != nil {
			return sigs, err
		}
		output := wire.NewTxOut(out.Value, scriptPubkey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false, P2SH_2of3_Multisig)
	fee := estimatedSize * int(feePerByte)
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

	for i := range tx.TxIn {
		sig, err := rawTxInSignature(tx, i, redeemScript, txscript.SigHashAll, signingKey, values[i])
		if err != nil {
			continue
		}
		bs := wi.Signature{InputIndex: uint32(i), Signature: sig}
		sigs = append(sigs, bs)
	}
	return sigs, nil
}

func (w *ZCashWallet) multisign(ins []wi.TransactionInput, outs []wi.TransactionOutput, sigs1 []wi.Signature, sigs2 []wi.Signature, redeemScript []byte, feePerByte uint64, broadcast bool) ([]byte, error) {
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
		scriptPubkey, err := zaddr.PayToAddrScript(out.Address)
		if err != nil {
			return nil, err
		}
		output := wire.NewTxOut(out.Value, scriptPubkey)
		tx.TxOut = append(tx.TxOut, output)
	}

	// Subtract fee
	estimatedSize := EstimateSerializeSize(len(ins), tx.TxOut, false, P2SH_2of3_Multisig)
	fee := estimatedSize * int(feePerByte)
	if len(tx.TxOut) > 0 {
		feePerOutput := fee / len(tx.TxOut)
		for _, output := range tx.TxOut {
			output.Value -= int64(feePerOutput)
		}
	}

	// BIP 69 sorting
	txsort.InPlaceSort(tx)

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

		builder := txscript.NewScriptBuilder()
		builder.AddOp(txscript.OP_0)
		builder.AddData(sig1)
		builder.AddData(sig2)
		builder.AddData(redeemScript)
		scriptSig, err := builder.Script()
		if err != nil {
			return nil, err
		}
		input.SignatureScript = scriptSig
	}
	// broadcast
	if broadcast {
		if _, err := w.Broadcast(tx); err != nil {
			return nil, err
		}
	}
	return serializeVersion4Transaction(tx, 0)
}

func (w *ZCashWallet) generateMultisigScript(keys []hd.ExtendedKey, threshold int, timeout time.Duration, timeoutKey *hd.ExtendedKey) (addr btc.Address, redeemScript []byte, err error) {
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
	builder.AddInt64(int64(threshold))
	for _, key := range ecKeys {
		builder.AddData(key.SerializeCompressed())
	}
	builder.AddInt64(int64(len(ecKeys)))
	builder.AddOp(txscript.OP_CHECKMULTISIG)

	redeemScript, err = builder.Script()
	if err != nil {
		return nil, nil, err
	}

	addr, err = zaddr.NewAddressScriptHash(redeemScript, w.params)
	if err != nil {
		return nil, nil, err
	}
	return addr, redeemScript, nil
}

func (w *ZCashWallet) estimateSpendFee(amount int64, feeLevel wi.FeeLevel) (uint64, error) {
	// Since this is an estimate we can use a dummy output address. Let's use a long one so we don't under estimate.
	addr, err := zaddr.DecodeAddress("t1hASvMj8e6TXWryuB3L5TKXJB7XfNioZP3", &chaincfg.MainNetParams)
	if err != nil {
		return 0, err
	}
	tx, err := w.buildTx(amount, addr, feeLevel, nil)
	if err != nil {
		return 0, err
	}
	var outval int64
	for _, output := range tx.TxOut {
		outval += output.Value
	}
	var inval int64
	utxos, err := w.db.Utxos().GetAll()
	if err != nil {
		return 0, err
	}
	for _, input := range tx.TxIn {
		for _, utxo := range utxos {
			if utxo.Op.Hash.IsEqual(&input.PreviousOutPoint.Hash) && utxo.Op.Index == input.PreviousOutPoint.Index {
				inval += utxo.Value
				break
			}
		}
	}
	if inval < outval {
		return 0, errors.New("Error building transaction: inputs less than outputs")
	}
	return uint64(inval - outval), err
}

// rawTxInSignature returns the serialized ECDSA signature for the input idx of
// the given transaction, with hashType appended to it.
func rawTxInSignature(tx *wire.MsgTx, idx int, prevScriptBytes []byte,
	hashType txscript.SigHashType, key *btcec.PrivateKey, amt int64) ([]byte, error) {

	hash, err := calcSignatureHash(prevScriptBytes, hashType, tx, idx, amt, 0)
	if err != nil {
		return nil, err
	}
	signature, err := key.Sign(hash)
	if err != nil {
		return nil, fmt.Errorf("cannot sign tx input: %s", err)
	}

	return append(signature.Serialize(), byte(hashType)), nil
}

func calcSignatureHash(prevScriptBytes []byte, hashType txscript.SigHashType, tx *wire.MsgTx, idx int, amt int64, expiry uint32) ([]byte, error) {

	// As a sanity check, ensure the passed input index for the transaction
	// is valid.
	if idx > len(tx.TxIn)-1 {
		return nil, fmt.Errorf("idx %d but %d txins", idx, len(tx.TxIn))
	}

	// We'll utilize this buffer throughout to incrementally calculate
	// the signature hash for this transaction.
	var sigHash bytes.Buffer

	// Write header
	_, err := sigHash.Write(txHeaderBytes)
	if err != nil {
		return nil, err
	}

	// Write group ID
	_, err = sigHash.Write(txNVersionGroupIDBytes)
	if err != nil {
		return nil, err
	}

	// Next write out the possibly pre-calculated hashes for the sequence
	// numbers of all inputs, and the hashes of the previous outs for all
	// outputs.
	var zeroHash chainhash.Hash

	// If anyone can pay isn't active, then we can use the cached
	// hashPrevOuts, otherwise we just write zeroes for the prev outs.
	if hashType&txscript.SigHashAnyOneCanPay == 0 {
		sigHash.Write(calcHashPrevOuts(tx))
	} else {
		sigHash.Write(zeroHash[:])
	}

	// If the sighash isn't anyone can pay, single, or none, the use the
	// cached hash sequences, otherwise write all zeroes for the
	// hashSequence.
	if hashType&txscript.SigHashAnyOneCanPay == 0 &&
		hashType&sigHashMask != txscript.SigHashSingle &&
		hashType&sigHashMask != txscript.SigHashNone {
		sigHash.Write(calcHashSequence(tx))
	} else {
		sigHash.Write(zeroHash[:])
	}

	// If the current signature mode isn't single, or none, then we can
	// re-use the pre-generated hashoutputs sighash fragment. Otherwise,
	// we'll serialize and add only the target output index to the signature
	// pre-image.
	if hashType&sigHashMask != txscript.SigHashSingle &&
		hashType&sigHashMask != txscript.SigHashNone {
		sigHash.Write(calcHashOutputs(tx))
	} else if hashType&sigHashMask == txscript.SigHashSingle && idx < len(tx.TxOut) {
		var b bytes.Buffer
		wire.WriteTxOut(&b, 0, 0, tx.TxOut[idx])
		sigHash.Write(chainhash.DoubleHashB(b.Bytes()))
	} else {
		sigHash.Write(zeroHash[:])
	}

	// Write hash JoinSplits
	sigHash.Write(make([]byte, 32))

	// Write hash ShieldedSpends
	sigHash.Write(make([]byte, 32))

	// Write hash ShieldedOutputs
	sigHash.Write(make([]byte, 32))

	// Write out the transaction's locktime, and the sig hash
	// type.
	var bLockTime [4]byte
	binary.LittleEndian.PutUint32(bLockTime[:], tx.LockTime)
	sigHash.Write(bLockTime[:])

	// Write expiry
	var bExpiryTime [4]byte
	binary.LittleEndian.PutUint32(bExpiryTime[:], expiry)
	sigHash.Write(bExpiryTime[:])

	// Write valueblance
	sigHash.Write(make([]byte, 8))

	// Write the hash type
	var bHashType [4]byte
	binary.LittleEndian.PutUint32(bHashType[:], uint32(hashType))
	sigHash.Write(bHashType[:])

	// Next, write the outpoint being spent.
	sigHash.Write(tx.TxIn[idx].PreviousOutPoint.Hash[:])
	var bIndex [4]byte
	binary.LittleEndian.PutUint32(bIndex[:], tx.TxIn[idx].PreviousOutPoint.Index)
	sigHash.Write(bIndex[:])

	// Write the previous script bytes
	wire.WriteVarBytes(&sigHash, 0, prevScriptBytes)

	// Next, add the input amount, and sequence number of the input being
	// signed.
	var bAmount [8]byte
	binary.LittleEndian.PutUint64(bAmount[:], uint64(amt))
	sigHash.Write(bAmount[:])
	var bSequence [4]byte
	binary.LittleEndian.PutUint32(bSequence[:], tx.TxIn[idx].Sequence)
	sigHash.Write(bSequence[:])

	leBranchID := make([]byte, 4)
	binary.LittleEndian.PutUint32(leBranchID, branchID)
	bl, _ := blake2b.New(&blake2b.Config{
		Size:   32,
		Person: append(sigHashPersonalization, leBranchID...),
	})
	bl.Write(sigHash.Bytes())
	h := bl.Sum(nil)
	return h[:], nil
}

// serializeVersion4Transaction serializes a wire.MsgTx into the zcash version four
// wire transaction format.
func serializeVersion4Transaction(tx *wire.MsgTx, expiryHeight uint32) ([]byte, error) {
	var buf bytes.Buffer

	// Write header
	_, err := buf.Write(txHeaderBytes)
	if err != nil {
		return nil, err
	}

	// Write group ID
	_, err = buf.Write(txNVersionGroupIDBytes)
	if err != nil {
		return nil, err
	}

	// Write varint input count
	count := uint64(len(tx.TxIn))
	err = wire.WriteVarInt(&buf, wire.ProtocolVersion, count)
	if err != nil {
		return nil, err
	}

	// Write inputs
	for _, ti := range tx.TxIn {
		// Write outpoint hash
		_, err := buf.Write(ti.PreviousOutPoint.Hash[:])
		if err != nil {
			return nil, err
		}
		// Write outpoint index
		index := make([]byte, 4)
		binary.LittleEndian.PutUint32(index, ti.PreviousOutPoint.Index)
		_, err = buf.Write(index)
		if err != nil {
			return nil, err
		}
		// Write sigscript
		err = wire.WriteVarBytes(&buf, wire.ProtocolVersion, ti.SignatureScript)
		if err != nil {
			return nil, err
		}
		// Write sequence
		sequence := make([]byte, 4)
		binary.LittleEndian.PutUint32(sequence, ti.Sequence)
		_, err = buf.Write(sequence)
		if err != nil {
			return nil, err
		}
	}
	// Write varint output count
	count = uint64(len(tx.TxOut))
	err = wire.WriteVarInt(&buf, wire.ProtocolVersion, count)
	if err != nil {
		return nil, err
	}
	// Write outputs
	for _, to := range tx.TxOut {
		// Write value
		val := make([]byte, 8)
		binary.LittleEndian.PutUint64(val, uint64(to.Value))
		_, err = buf.Write(val)
		if err != nil {
			return nil, err
		}
		// Write pkScript
		err = wire.WriteVarBytes(&buf, wire.ProtocolVersion, to.PkScript)
		if err != nil {
			return nil, err
		}
	}
	// Write nLocktime
	nLockTime := make([]byte, 4)
	binary.LittleEndian.PutUint32(nLockTime, tx.LockTime)
	_, err = buf.Write(nLockTime)
	if err != nil {
		return nil, err
	}

	// Write nExpiryHeight
	expiry := make([]byte, 4)
	binary.LittleEndian.PutUint32(expiry, expiryHeight)
	_, err = buf.Write(expiry)
	if err != nil {
		return nil, err
	}

	// Write nil value balance
	_, err = buf.Write(make([]byte, 8))
	if err != nil {
		return nil, err
	}

	// Write nil value vShieldedSpend
	_, err = buf.Write(make([]byte, 1))
	if err != nil {
		return nil, err
	}

	// Write nil value vShieldedOutput
	_, err = buf.Write(make([]byte, 1))
	if err != nil {
		return nil, err
	}

	// Write nil value vJoinSplit
	_, err = buf.Write(make([]byte, 1))
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func calcHashPrevOuts(tx *wire.MsgTx) []byte {
	var b bytes.Buffer
	for _, in := range tx.TxIn {
		// First write out the 32-byte transaction ID one of whose
		// outputs are being referenced by this input.
		b.Write(in.PreviousOutPoint.Hash[:])

		// Next, we'll encode the index of the referenced output as a
		// little endian integer.
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], in.PreviousOutPoint.Index)
		b.Write(buf[:])
	}
	bl, _ := blake2b.New(&blake2b.Config{
		Size:   32,
		Person: hashPrevOutPersonalization,
	})
	bl.Write(b.Bytes())
	h := bl.Sum(nil)
	return h[:]
}

func calcHashSequence(tx *wire.MsgTx) []byte {
	var b bytes.Buffer
	for _, in := range tx.TxIn {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], in.Sequence)
		b.Write(buf[:])
	}
	bl, _ := blake2b.New(&blake2b.Config{
		Size:   32,
		Person: hashSequencePersonalization,
	})
	bl.Write(b.Bytes())
	h := bl.Sum(nil)
	return h[:]
}

func calcHashOutputs(tx *wire.MsgTx) []byte {
	var b bytes.Buffer
	for _, out := range tx.TxOut {
		wire.WriteTxOut(&b, 0, 0, out)
	}
	bl, _ := blake2b.New(&blake2b.Config{
		Size:   32,
		Person: hashOutputsPersonalization,
	})
	bl.Write(b.Bytes())
	h := bl.Sum(nil)
	return h[:]
}
