package core

import (
	"crypto/sha256"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/golang/protobuf/ptypes"
	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	ps "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	mh "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
)

// Hash with SHA-256 and encode as a multihash
func EncodeCID(b []byte) (*cid.Cid, error) {
	multihash, err := EncodeMultihash(b)
	if err != nil {
		return nil, err
	}
	id := cid.NewCidV1(cid.Raw, *multihash)
	return id, err
}

func EncodeMultihash(b []byte) (*mh.Multihash, error) {
	h := sha256.Sum256(b)
	encoded, err := mh.Encode(h[:], mh.SHA2_256)
	if err != nil {
		return nil, err
	}
	multihash, err := mh.Cast(encoded)
	if err != nil {
		return nil, err
	}
	return &multihash, err
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
	return val, nil
}

// Used by the GET order API to build transaction records suitable to be included in the order response
func (n *OpenBazaarNode) BuildTransactionRecords(contract *pb.RicardianContract, records []*wallet.TransactionRecord, state pb.OrderState) ([]*pb.TransactionRecord, *pb.TransactionRecord, error) {
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
