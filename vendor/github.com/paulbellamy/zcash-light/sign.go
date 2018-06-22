package zcash

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	btc "github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcwallet/wallet/txauthor"
	"github.com/btcsuite/btcwallet/wallet/txrules"
)

// Special case nIn for signing JoinSplits.
const NotAnInput int = -1

// TODO: Support pre-overwinter v2 joinsplit transactions here (maybe)
func ProduceSignature(
	params *chaincfg.Params,
	tx *Transaction,
	idx int,
	pkScript []byte,
	hashType txscript.SigHashType,
	kdb txscript.KeyDB,
	sdb txscript.ScriptDB,
	previousScript []byte,
) ([]byte, error) {
	creator := TransactionSignatureCreator(kdb, sdb, tx, idx, hashType)
	results, _, ok := SignStep(params, creator, pkScript, tx.VersionGroupID)
	if !ok {
		return nil, fmt.Errorf("unable to sign transaction")
	}
	return PushAll(results)
}

func PushAll(scripts [][]byte) ([]byte, error) {
	result := txscript.NewScriptBuilder()
	for _, script := range scripts {
		switch {
		case len(script) == 0:
			result = result.AddOp(txscript.OP_0)
		case len(script) == 1 && 1 <= script[0] && script[0] <= 16:
			result = result.AddOp(script[0])
		default:
			result = result.AddData(script)
		}
	}
	return result.Script()
}

func Sign1(address btc.Address, creator SignatureCreator, scriptCode []byte, consensusBranchId uint32) ([]byte, bool) {
	return creator.CreateSig(address, scriptCode, consensusBranchId)
}

func SignN(params *chaincfg.Params, multisigdata [][]byte, creator SignatureCreator, scriptCode []byte, consensusBranchId uint32) ([][]byte, bool) {
	var ret [][]byte
	var nSigned int = 0
	var nRequired int = int(multisigdata[0][0])
	for i := 1; i < len(multisigdata)-1 && nSigned < nRequired; i++ {
		pubkey := multisigdata[i]
		address, err := NewAddressPubKeyHash(btc.Hash160(pubkey), params)
		if err != nil {
			continue
		}
		sig, ok := Sign1(address, creator, scriptCode, consensusBranchId)
		if !ok {
			continue
		}
		nSigned++
		ret = append(ret, sig)
	}
	return ret, nSigned == nRequired
}

/**
 * Sign scriptPubKey using signature made with creator.
 * Signatures are returned in scriptSigRet (or returns false if scriptPubKey can't be signed),
 * unless scriptClass is txscript.ScriptHashTy, in which case scriptSigRet is the redemption script.
 * Returns false if scriptPubKey could not be completely satisfied.
 */
func SignStep(params *chaincfg.Params, creator SignatureCreator, scriptPubKey []byte, consensusBranchId uint32) ([][]byte, txscript.ScriptClass, bool) {
	addr, err := ExtractPkScriptAddrs(scriptPubKey, params)
	if err != nil {
		return nil, 0, false
	}

	scriptClass := txscript.GetScriptClass(scriptPubKey)
	switch scriptClass {
	case txscript.NonStandardTy, txscript.NullDataTy:
		return nil, scriptClass, false

	case txscript.PubKeyTy:
		sig, ok := Sign1(addr, creator, scriptPubKey, consensusBranchId)
		return [][]byte{sig}, scriptClass, ok

	case txscript.PubKeyHashTy:
		sig, ok := Sign1(addr, creator, scriptPubKey, consensusBranchId)
		if !ok {
			return nil, scriptClass, false
		}
		privKey, compressed, err := creator.GetKey(addr)
		if err != nil {
			return nil, scriptClass, false
		}
		pubKey := (*btcec.PublicKey)(&privKey.PublicKey)
		var pkData []byte
		if compressed {
			pkData = pubKey.SerializeCompressed()
		} else {
			pkData = pubKey.SerializeUncompressed()
		}

		return append([][]byte{sig}, pkData), scriptClass, true

	case txscript.ScriptHashTy:
		scriptRet, err := creator.GetScript(addr)
		if err != nil {
			return nil, scriptClass, false
		}

		// Solver returns the subscript that needs to be evaluated;
		// the final scriptSig is the signatures from that
		// and then the serialized subscript:
		var subscript = scriptRet
		subscriptResults, subscriptType, ok := SignStep(params, creator, subscript, consensusBranchId)
		if !ok || subscriptType == txscript.ScriptHashTy {
			return nil, scriptClass, false
		}
		return append([][]byte{subscript}, subscriptResults...), scriptClass, true

	case txscript.MultiSigTy:
		// TODO: Overwinter multisig signing not implemented
		return nil, scriptClass, false

	default:
		return nil, scriptClass, false
	}
}

type InputSource func(target btc.Amount) (total btc.Amount, inputs []Input, scripts [][]byte, err error)

// NewUnsignedTransaction is reused from spvwallet and modified to be less btc-specific
func NewUnsignedTransaction(outputs []Output, feePerKb btc.Amount, fetchInputs InputSource, fetchChange txauthor.ChangeSource, isOverwinter bool) (*Transaction, error) {

	var targetAmount btc.Amount
	for _, output := range outputs {
		targetAmount += btc.Amount(output.Value)
	}

	estimatedSize := EstimateSerializeSize(1, outputs, true, P2PKH)
	targetFee := txrules.FeeForSerializeSize(feePerKb, estimatedSize)

	for {
		inputAmount, inputs, _, err := fetchInputs(targetAmount + targetFee)
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

		unsignedTransaction := &Transaction{
			IsOverwinter: isOverwinter,
			Version:      1,
			Inputs:       inputs,
			Outputs:      outputs,
		}
		if isOverwinter {
			unsignedTransaction.Version = 3
			unsignedTransaction.VersionGroupID = OverwinterVersionGroupID
		}
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
			change := Output{Value: int64(changeAmount), ScriptPubKey: changeScript}
			l := len(outputs)
			unsignedTransaction.Outputs = append(outputs[:l:l], change)
		}

		return unsignedTransaction, nil
	}
}
