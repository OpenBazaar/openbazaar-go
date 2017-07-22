package core

import (
	"crypto/sha256"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/golang/protobuf/ptypes"
	mh "gx/ipfs/QmVGtdTZdTFaLsaj2RwdVG8jcjNNcp1DE914DKZ2kHmXHw/go-multihash"
	ps "gx/ipfs/QmXZSd1qR5BxZkPyuwfT5jpqQFScZccoZvDneXsKzCNHWX/go-libp2p-peerstore"
	cid "gx/ipfs/QmYhQaCYEcaPPjxJX7YcPcVKkQfRy6sJ7B3XmGFk82XYdQ/go-cid"
	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
)

// Hash with SHA-256 and encode as a multihash
func EncodeMultihashCID(b []byte) (*cid.Cid, error) {
	h := sha256.Sum256(b)
	encoded, err := mh.Encode(h[:], mh.SHA2_256)
	if err != nil {
		return nil, err
	}
	multihash, err := mh.Cast(encoded)
	if err != nil {
		return nil, err
	}
	id := cid.NewCidV1(cid.DagCBOR, multihash)
	return id, err
}

// Certain pointers, such as moderators, contain a peerID. This function
// will extract the ID from the underlying PeerInfo object.
func ExtractIDFromPointer(pi ps.PeerInfo) (string, error) {
	if len(pi.Addrs) == 0 {
		return "", errors.New("PeerInfo object has no addresses")
	}
	addr := pi.Addrs[0]
	if addr.Protocols()[0].Code != ma.P_IPFS {
		return "", errors.New("IPFS protocol not found in address")
	}
	val, err := addr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		return "", err
	}
	_, err = mh.FromB58String(val)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// Used by the GET order API to build transaction records suitable to be included in the order response
func (n *OpenBazaarNode) BuildTransactionRecords(contract *pb.RicardianContract, records []*spvwallet.TransactionRecord, state pb.OrderState) ([]*pb.TransactionRecord, *pb.TransactionRecord, error) {
	paymentRecords := []*pb.TransactionRecord{}
	payments := make(map[string]*pb.TransactionRecord)

	// Consolidate any transactions with multiple outputs into a single record
	for _, r := range records {
		record, ok := payments[r.Txid]
		if ok {
			record.Value += r.Value
			payments[r.Txid] = record
		} else {
			tx := new(pb.TransactionRecord)
			tx.Txid = r.Txid
			tx.Value = r.Value
			ts, err := ptypes.TimestampProto(r.Timestamp)
			if err != nil {
				return paymentRecords, nil, err
			}
			tx.Timestamp = ts
			ch, err := chainhash.NewHashFromStr(tx.Txid)
			if err != nil {
				return paymentRecords, nil, err
			}
			confirmations, height, err := n.Wallet.GetConfirmations(*ch)
			if err != nil {
				return paymentRecords, nil, err
			}
			tx.Height = height
			tx.Confirmations = confirmations
			payments[r.Txid] = tx
		}
	}
	for _, rec := range payments {
		paymentRecords = append(paymentRecords, rec)
	}
	var refundRecord *pb.TransactionRecord
	if contract != nil && (state == pb.OrderState_REFUNDED || state == pb.OrderState_DECLINED || state == pb.OrderState_CANCELED) && contract.BuyerOrder != nil && contract.BuyerOrder.Payment != nil {
		// For multisig we can use the outgoing from the payment address
		if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED || state == pb.OrderState_DECLINED || state == pb.OrderState_CANCELED {
			for _, rec := range payments {
				if rec.Value < 0 {
					refundRecord = new(pb.TransactionRecord)
					refundRecord.Txid = rec.Txid
					refundRecord.Value = -rec.Value
					refundRecord.Confirmations = rec.Confirmations
					refundRecord.Height = rec.Height
					refundRecord.Timestamp = rec.Timestamp
					break
				}
			}
		} else if contract.Refund != nil && contract.Refund.RefundTransaction != nil && contract.Refund.Timestamp != nil {
			refundRecord = new(pb.TransactionRecord)
			// Direct we need to use the transaction info in the contract's refund object
			ch, err := chainhash.NewHashFromStr(contract.Refund.RefundTransaction.Txid)
			if err != nil {
				return paymentRecords, refundRecord, err
			}
			confirmations, height, err := n.Wallet.GetConfirmations(*ch)
			if err != nil {
				return paymentRecords, refundRecord, err
			}
			refundRecord.Txid = contract.Refund.RefundTransaction.Txid
			refundRecord.Value = int64(contract.Refund.RefundTransaction.Value)
			refundRecord.Timestamp = contract.Refund.Timestamp
			refundRecord.Confirmations = confirmations
			refundRecord.Height = height
		}
	}
	return paymentRecords, refundRecord, nil
}
