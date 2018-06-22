package zcash

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	btc "github.com/btcsuite/btcutil"
	"golang.org/x/crypto/blake2b"
)

type SignatureCreator interface {
	CreateSig(address btc.Address, scriptCode []byte, consensusBranchId uint32) ([]byte, bool)
	txscript.KeyDB
	txscript.ScriptDB
}

func TransactionSignatureCreator(kdb txscript.KeyDB, sdb txscript.ScriptDB, tx *Transaction, idx int, hashType txscript.SigHashType) SignatureCreator {
	return &signatureCreator{
		KeyDB:    kdb,
		ScriptDB: sdb,
		tx:       tx,
		idx:      idx,
		hashType: hashType,
	}
}

type signatureCreator struct {
	txscript.KeyDB
	txscript.ScriptDB
	tx       *Transaction
	idx      int
	hashType txscript.SigHashType
}

func (s *signatureCreator) CreateSig(address btc.Address, scriptCode []byte, consensusBranchId uint32) ([]byte, bool) {
	key, _, err := s.GetKey(address)
	if err != nil {
		return nil, false
	}

	hash, err := SignatureHash(scriptCode, s.tx, s.idx, s.hashType, consensusBranchId)
	if err != nil {
		return nil, false
	}

	signature, err := key.Sign(hash)
	if err != nil {
		return nil, false
	}

	return append(signature.Serialize(), byte(s.hashType)), true
}

var (
	PrevoutsHashPersonalization   = []byte("ZcashPrevoutHash")
	SequenceHashPersonalization   = []byte("ZcashSequencHash")
	OutputsHashPersonalization    = []byte("ZcashOutputsHash")
	JoinSplitsHashPersonalization = []byte("ZcashJSplitsHash")
)

func SignatureHash(scriptCode []byte, tx *Transaction, idx int, hashType txscript.SigHashType, consensusBranchId uint32) ([]byte, error) {
	if idx >= len(tx.Inputs) && idx != NotAnInput {
		// index out of range
		return nil, fmt.Errorf("input index is out of range")
	}

	if tx.IsOverwinter {
		return overwinterSignatureHash(scriptCode, tx, idx, hashType, consensusBranchId)
	}
	return sproutSignatureHash(scriptCode, tx, idx, hashType)
}

func overwinterSignatureHash(scriptCode []byte, tx *Transaction, idx int, hashType txscript.SigHashType, consensusBranchId uint32) ([]byte, error) {
	/*
			BLAKE2b-256 hash of the serialization of:
		  1. header of the transaction (4-byte little endian)
		  2. nVersionGroupId of the transaction (4-byte little endian)
		  3. hashPrevouts (32-byte hash)
		  4. hashSequence (32-byte hash)
		  5. hashOutputs (32-byte hash)
		  6. hashJoinSplits (32-byte hash)
		  7. nLockTime of the transaction (4-byte little endian)
		  8. nExpiryHeight of the transaction (4-byte little endian)
		  9. sighash type of the signature (4-byte little endian)
		 10. If we are serializing an input (i.e. this is not a JoinSplit signature hash):
		     a. outpoint (32-byte hash + 4-byte little endian)
		     b. scriptCode of the input (serialized as scripts inside CTxOuts)
		     c. value of the output spent by this input (8-byte little endian)
		     d. nSequence of the input (4-byte little endian)
	*/

	// The default values are zeroes
	var hashPrevouts, hashSequence, hashOutputs, hashJoinSplits []byte

	if (hashType & txscript.SigHashAnyOneCanPay) == 0 {
		ss, err := blake2b.New256(PrevoutsHashPersonalization)
		if err != nil {
			return nil, err
		}
		for _, input := range tx.Inputs {
			if err := input.writeOutPoint(ss); err != nil {
				return nil, err
			}
		}
		hashPrevouts = ss.Sum(nil)
	}

	if (hashType&txscript.SigHashAnyOneCanPay == 0) && (hashType&sigHashMask) != txscript.SigHashSingle && (hashType&sigHashMask) != txscript.SigHashNone {
		ss, err := blake2b.New256(SequenceHashPersonalization)
		if err != nil {
			return nil, err
		}
		for _, input := range tx.Inputs {
			if err := writeField(input.Sequence)(ss); err != nil {
				return nil, err
			}
		}
		hashSequence = ss.Sum(nil)
	}

	if (hashType&sigHashMask) != txscript.SigHashSingle && (hashType&sigHashMask) != txscript.SigHashNone {
		ss, err := blake2b.New256(OutputsHashPersonalization)
		if err != nil {
			return nil, err
		}
		for _, output := range tx.Outputs {
			if _, err := output.WriteTo(ss); err != nil {
				return nil, err
			}
		}
		hashOutputs = ss.Sum(nil)
	} else if (hashType&sigHashMask) == txscript.SigHashSingle && idx < len(tx.Outputs) {
		ss, err := blake2b.New256(OutputsHashPersonalization)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Outputs[idx].WriteTo(ss); err != nil {
			return nil, err
		}
		hashOutputs = ss.Sum(nil)
	}

	if len(tx.JoinSplits) > 0 {
		ss, err := blake2b.New256(JoinSplitsHashPersonalization)
		if err != nil {
			return nil, err
		}
		for _, js := range tx.JoinSplits {
			if _, err := js.WriteTo(ss); err != nil {
				return nil, err
			}
		}
		if err := writeBytes(tx.JoinSplitPubKey[:])(ss); err != nil {
			return nil, err
		}
		hashJoinSplits = ss.Sum(nil)
	}

	personalization := bytes.NewBufferString("ZcashSigHash")
	if err := writeField(consensusBranchId)(personalization); err != nil {
		return nil, err
	}

	ss, err := blake2b.New256(personalization.Bytes())
	if err != nil {
		return nil, err
	}
	if err := writeAll(ss,
		// fOverwintered and nVersion
		tx.GetHeader(),
		// Version group ID
		tx.VersionGroupID,
		// Input prevouts/nSequence (none/all, depending on flags)
		hashPrevouts,
		hashSequence,
		// Outputs (none/one/all, depending on flags)
		hashOutputs,
		// JoinSplits
		hashJoinSplits,
		// Locktime
		tx.LockTime,
		// Expiry height
		tx.ExpiryHeight,
		// Sighash type
		hashType,
	); err != nil {
		return nil, err
	}

	if idx != NotAnInput {
		// The input being signed (replacing the scriptSig with scriptCode + amount)
		// The prevout may already be contained in hashPrevout, and the nSequence
		// may already be contained in hashSequence.
		var amountIn int64
		if idx < len(tx.Outputs) {
			amountIn = tx.Outputs[idx].Value
		}

		if err := tx.Inputs[idx].writeOutPoint(ss); err != nil {
			return nil, err
		}
		if err := writeAll(ss, scriptCode, amountIn, tx.Inputs[idx].Sequence); err != nil {
			return nil, err
		}
	}

	return ss.Sum(nil), nil
}

// sigHashMask defines the number of bits of the hash type which is used
// to identify which outputs are signed.
const sigHashMask = 0x1f

func sproutSignatureHash(scriptCode []byte, tx *Transaction, idx int, hashType txscript.SigHashType) ([]byte, error) {
	var one chainhash.Hash
	one[0] = 0x01
	if idx >= len(tx.Inputs) || idx == NotAnInput {
		return one[:], nil
	}

	txCopy := tx.shallowCopy()

	// Blank out other inputs' signatures
	for i := range txCopy.Inputs {
		txCopy.Inputs[i].SignatureScript = nil
	}
	txCopy.Inputs[idx].SignatureScript = scriptCode

	switch hashType & sigHashMask {
	case txscript.SigHashNone:
		txCopy.Outputs = txCopy.Outputs[0:0] // Empty slice.
		for i := range txCopy.Inputs {
			if i != idx {
				txCopy.Inputs[i].Sequence = 0
			}
		}

	case txscript.SigHashSingle:
		if idx >= len(tx.Outputs) {
			//  nOut out of range
			return nil, fmt.Errorf("no matching output for SIGHASH_SINGLE")
		}

		// Resize output array to up to and including requested index.
		txCopy.Outputs = txCopy.Outputs[:idx+1]

		// All but current output get zeroed out.
		for i := 0; i < idx; i++ {
			txCopy.Outputs[i].Value = -1
			txCopy.Outputs[i].ScriptPubKey = nil
		}

		// Sequence on all other inputs is 0, too.
		for i := range txCopy.Inputs {
			if i != idx {
				txCopy.Inputs[i].Sequence = 0
			}
		}

	default:
		// Consensus treats undefined hashtypes like normal SigHashAll
		// for purposes of hash generation.
		fallthrough
	case txscript.SigHashOld:
		fallthrough
	case txscript.SigHashAll:
		// Nothing special here.
	}

	// Blank out other inputs completely, not recommended for open transactions
	if hashType&txscript.SigHashAnyOneCanPay != 0 {
		txCopy.Inputs = txCopy.Inputs[idx : idx+1]
	}

	// Blank out the joinsplit signature.
	txCopy.JoinSplitSignature = [64]byte{}

	// Serialize and hash
	buf := &bytes.Buffer{}
	txCopy.WriteTo(buf)
	binary.Write(buf, binary.LittleEndian, hashType)
	return chainhash.DoubleHashB(buf.Bytes()), nil
}

// shallowCopy creates a shallow copy of the transaction for use when
// calculating the signature hash.  It is used over the Copy method on the
// transaction itself since that is a deep copy and therefore does more work and
// allocates much more space than needed.
func (tx Transaction) shallowCopy() Transaction {
	// As an additional memory optimization, use contiguous backing arrays
	// for the copied inputs and outputs and point the final slice of
	// pointers into the contiguous arrays.  This avoids a lot of small
	// allocations.
	txCopy := tx
	txCopy.Inputs = make([]Input, len(tx.Inputs))
	txCopy.Outputs = make([]Output, len(tx.Outputs))
	txCopy.JoinSplits = make([]JoinSplit, len(tx.JoinSplits))
	txCopy.JoinSplitPubKey = [32]byte{}
	txCopy.JoinSplitSignature = [64]byte{}

	for i, input := range tx.Inputs {
		txCopy.Inputs[i] = input
	}
	for i, output := range tx.Outputs {
		txCopy.Outputs[i] = output
	}
	for i, joinSplit := range tx.JoinSplits {
		txCopy.JoinSplits[i] = joinSplit
	}
	copy(txCopy.JoinSplitPubKey[:], tx.JoinSplitPubKey[:])
	copy(txCopy.JoinSplitSignature[:], tx.JoinSplitSignature[:])
	return txCopy
}
