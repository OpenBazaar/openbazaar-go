package core

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/golang/protobuf/ptypes/timestamp"
	"gx/ipfs/QmT6n4mspWYEya864BhCUJEgyxiRfmiSY9ruQwTUNpRKaM/protobuf/proto"
	"time"
)

func (n *OpenBazaarNode) OpenDispute(orderID string, contract *pb.RicardianContract, records []*spvwallet.TransactionRecord, claim string) error {
	var isPurchase bool
	if n.IpfsNode.Identity.Pretty() == contract.BuyerOrder.BuyerID.Guid {
		isPurchase = true
	}

	dispute := new(pb.Dispute)

	// Create timestamp
	ts := new(timestamp.Timestamp)
	ts.Seconds = time.Now().Unix()
	ts.Nanos = 0
	dispute.Timestamp = ts

	// Add claim
	dispute.Claim = claim

	// Create outpoints
	var outpoints []*pb.Dispute_Outpoint
	for _, r := range records {
		var o *pb.Dispute_Outpoint
		o.Hash = r.Txid
		o.Index = r.Index
		outpoints = append(outpoints, o)
	}
	dispute.Outpoints = outpoints

	// Maybe add rating keys
	if isPurchase {
		var keys [][]byte
		for _, k := range contract.BuyerOrder.RatingKeys {
			keys = append(keys, k)
		}
		dispute.RatingKeys = keys
	}

	// Serialize contract
	ser, err := proto.Marshal(contract)
	if err != nil {
		return err
	}
	dispute.SerializedContract = ser

	// Sign dispute
	rc := new(pb.RicardianContract)
	rc.Dispute = dispute
	rc, err = n.SignDispute(rc)
	if err != nil {
		return err
	}
	contract.Dispute = dispute
	contract.Signatures = append(contract.Signatures, rc.Signatures[0])

	// Send to moderator
	err = n.SendDisputeOpen(contract.BuyerOrder.Payment.Moderator, rc)
	if err != nil {
		return err
	}

	// Send to counterparty
	var counterparty string
	if isPurchase {
		counterparty = contract.VendorListings[0].VendorID.Guid
	} else {
		counterparty = contract.BuyerOrder.BuyerID.Guid
	}
	err = n.SendDisputeOpen(counterparty, rc)
	if err != nil {
		return err
	}

	// Update database
	if isPurchase {
		n.Datastore.Purchases().Put(orderID, *contract, pb.OrderState_DISPUTED, true)
	} else {
		n.Datastore.Sales().Put(orderID, *contract, pb.OrderState_DISPUTED, true)
	}
	return nil
}

func (n *OpenBazaarNode) SignDispute(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedDispute, err := proto.Marshal(contract.Dispute)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_DISPUTE
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedDispute)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}
