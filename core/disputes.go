package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	libp2p "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"

	"strconv"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"golang.org/x/net/context"
)

// ConfirmationsPerHour is temporary until the Wallet interface has Attributes() to provide this value
const ConfirmationsPerHour = 6

// DisputeWg - waitgroup for disputes
var DisputeWg = new(sync.WaitGroup)

// ErrCaseNotFound - case not found err
var ErrCaseNotFound = errors.New("case not found")

// ErrCloseFailureCaseExpired - tried closing expired case err
var ErrCloseFailureCaseExpired = errors.New("unable to close expired case")

// ErrCloseFailureNoOutpoints indicates when a dispute cannot be closed due to neither party
// including outpoints with their dispute
var ErrCloseFailureNoOutpoints = errors.New("unable to close case with missing outpoints")

// ErrOpenFailureOrderExpired - tried disputing expired order err
var ErrOpenFailureOrderExpired = errors.New("unable to open case because order is too old to dispute")

// ErrVendorListingIsMissing - listing was missing from contract
var ErrVendorListingIsMissing = errors.New("contract has no vendor listings attached")

// ErrNoModerator - contract is missing a moderator
var ErrNoModerator = errors.New("contract has no moderator specified")

// ErrNoDispute - contract is missing the dispute
var ErrNoDispute = errors.New("contract has no dispute information")

// OpenDispute - open a dispute
func (n *OpenBazaarNode) OpenDispute(orderID string, contract *pb.RicardianContract, records []*wallet.TransactionRecord, claim string) error {
	if !n.VerifyEscrowFundsAreDisputable(contract, records) {
		return ErrOpenFailureOrderExpired
	}

	if contract.BuyerOrder.Payment.Moderator == "" {
		return ErrNoModerator
	}

	dispute, err := GetFreshDispute()
	if err != nil {
		return err
	}

	dispute.Claim = claim
	dispute.Outpoints = GetRecordOutpoints(records)

	dispute.PayoutAddress, err = n.GetPayoutAddress(contract.BuyerOrder.Payment.Coin)
	if err != nil {
		return err
	}

	dispute.SerializedContract, err = GetSerializedContract(contract)
	if err != nil {
		return err
	}

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
	err = n.SendDisputeOpen(contract.BuyerOrder.Payment.Moderator, nil, rc)
	if err != nil {
		return err
	}

	// Send to counterparty
	var counterparty string
	var counterkey libp2p.PubKey

	isPurchase := n.IsMyPeerID(contract.BuyerOrder.BuyerID.PeerID)

	if isPurchase {
		if len(contract.VendorListings) < 1 {
			return ErrVendorListingIsMissing
		}
		counterparty = contract.VendorListings[0].VendorID.PeerID
		counterkey, err = libp2p.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
		if err != nil {
			return err
		}
	} else {
		counterparty = contract.BuyerOrder.BuyerID.PeerID
		counterkey, err = libp2p.UnmarshalPublicKey(contract.BuyerOrder.BuyerID.Pubkeys.Identity)
		if err != nil {
			return err
		}
	}
	err = n.SendDisputeOpen(counterparty, &counterkey, rc)
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

func GetSerializedContract(contract *pb.RicardianContract) ([]byte, error) {
	ser, err := proto.Marshal(contract)
	if err != nil {
		return nil, err
	}
	return ser, nil
}

func (n *OpenBazaarNode) GetPayoutAddress(coinType string) (string, error) {
	wal, err := n.Multiwallet.WalletForCurrencyCode(coinType)
	if err != nil {
		return "", err
	}

	// Add payout address
	return wal.CurrentAddress(wallet.EXTERNAL).EncodeAddress(), nil
}

func GetRecordOutpoints(records []*wallet.TransactionRecord) []*pb.Outpoint {
	var outpoints []*pb.Outpoint
	for _, r := range records {
		o := new(pb.Outpoint)
		o.Hash = r.Txid
		o.Index = r.Index
		o.Value = uint64(r.Value)
		outpoints = append(outpoints, o)
	}
	return outpoints
}

func (n *OpenBazaarNode) IsMyPeerID(peerID string) bool {
	return n.IpfsNode.Identity.Pretty() == peerID
}

func GetFreshDispute() (*pb.Dispute, error) {
	dispute := new(pb.Dispute)

	// Create timestamp
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return nil, err
	}
	dispute.Timestamp = ts

	return dispute, nil
}

func (n *OpenBazaarNode) VerifyEscrowFundsAreDisputable(contract *pb.RicardianContract, records []*wallet.TransactionRecord) bool {
	if len(contract.VendorListings) < 1 {
		log.Error("There are no vendor listings in the contract")
		return false
	}

	confirmationsForTimeout := contract.VendorListings[0].Metadata.EscrowTimeoutHours * ConfirmationsPerHour
	wal, err := n.Multiwallet.WalletForCurrencyCode(contract.BuyerOrder.Payment.Coin)
	if err != nil {
		log.Errorf("Failed VerifyEscrowFundsAreDisputable(): %s", err.Error())
		return false
	}
	for _, r := range records {
		hash, err := chainhash.NewHashFromStr(r.Txid)
		if err != nil {
			log.Errorf("Failed NewHashFromStr(%s): %s", r.Txid, err.Error())
			return false
		}
		actualConfirmations, _, err := wal.GetConfirmations(*hash)
		if err != nil {
			log.Errorf("Failed GetConfirmations(%s): %s", hash.String(), err.Error())
			return false
		}
		if actualConfirmations >= confirmationsForTimeout {
			return false
		}
	}
	return true
}

// SignDispute - sign the dispute
func (n *OpenBazaarNode) SignDispute(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedDispute, err := proto.Marshal(contract.Dispute)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_DISPUTE
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedDispute)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

// VerifySignatureOnDisputeOpen - verify signatures in an open dispute
func (n *OpenBazaarNode) VerifySignatureOnDisputeOpen(contract *pb.RicardianContract, peerID string) error {
	var pubkey []byte
	deser := new(pb.RicardianContract)

	if contract.Dispute == nil {
		log.Error("Contract has no dispute attached")
		return ErrNoDispute
	}

	err := proto.Unmarshal(contract.Dispute.SerializedContract, deser)
	if err != nil {
		log.Error("Could not unmarshal the contract")
		return err
	}
	if len(deser.VendorListings) == 0 || deser.BuyerOrder.BuyerID == nil {
		return errors.New("invalid serialized contract")
	}

	if peerID == deser.BuyerOrder.BuyerID.PeerID {
		pubkey = deser.BuyerOrder.BuyerID.Pubkeys.Identity
	} else if peerID == deser.VendorListings[0].VendorID.PeerID {
		pubkey = deser.VendorListings[0].VendorID.Pubkeys.Identity
	} else {
		return errors.New("peer ID doesn't match either buyer or vendor")
	}

	if err := verifyMessageSignature(
		contract.Dispute,
		pubkey,
		contract.Signatures,
		pb.Signature_DISPUTE,
		peerID,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("contract does not contain a signature for the dispute")
		case invalidSigError:
			return errors.New("guid signature on contract failed to verify")
		case matchKeyError:
			return errors.New("public key in dispute does not match reported ID")
		default:
			return err
		}
	}
	return nil
}

func (n OpenBazaarNode) UpdateDisputeCase(orderID string, state pb.OrderState, openedByBuyer bool, dispute *pb.Dispute, contract *pb.RicardianContract, errors []string) error {
	err := n.Datastore.Cases().Put(orderID, pb.OrderState_DISPUTED, false, dispute.Claim, db.PaymentCoinForContract(contract), db.CoinTypeForContract(contract))
	if err != nil {
		return err
	}

	if openedByBuyer {
		err = n.Datastore.Cases().UpdateBuyerInfo(orderID, contract, errors, dispute.PayoutAddress, dispute.Outpoints)
		if err != nil {
			return err
		}
	} else {
		err = n.Datastore.Cases().UpdateVendorInfo(orderID, contract, errors, dispute.PayoutAddress, dispute.Outpoints)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n OpenBazaarNode) GetMatchingSaleForDispute(orderID string) (*pb.RicardianContract, pb.OrderState, []*wallet.TransactionRecord, error) {
	// Load out version of the contract from the db
	contract, orderState, _, txRecords, _, _, err := n.Datastore.Sales().GetByOrderId(orderID)
	if err != nil {
		log.Error("cannot find a matching sale for this dispute")
		return nil, 0, nil, net.OutOfOrderMessage
	}
	return contract, orderState, txRecords, err
}

func (n OpenBazaarNode) GetMatchingPurchaseForDispute(orderID string) (*pb.RicardianContract, pb.OrderState, []*wallet.TransactionRecord, error) {
	contract, orderState, _, txRecords, _, _, err := n.Datastore.Purchases().GetByOrderId(orderID)
	if err != nil {
		log.Error("cannot find a matching purchase for this dispute")
		return nil, 0, nil, net.OutOfOrderMessage
	}
	return contract, orderState, txRecords, err
}

func (n OpenBazaarNode) UpdateDisputeInDatabase(localContract *pb.RicardianContract, serializedContract *pb.RicardianContract, orderID string) error {
	// Append the dispute and signature
	localContract.Dispute = serializedContract.Dispute
	for _, sig := range serializedContract.Signatures {
		if sig.Section == pb.Signature_DISPUTE {
			localContract.Signatures = append(localContract.Signatures, sig)
		}
	}

	orderType := GetContractOrderType(localContract, n.IpfsNode.Identity.Pretty())

	// Save it back to the db with the new state
	if orderType == "sale" {
		err := n.Datastore.Sales().Put(orderID, *localContract, pb.OrderState_DISPUTED, false)
		if err != nil {
			return err
		}
	} else if orderType == "purchase" {
		err := n.Datastore.Purchases().Put(orderID, *localContract, pb.OrderState_DISPUTED, false)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *OpenBazaarNode) SendDisputeNotification(orderID string, thumbs map[string]string, disputerID string, disputerHandle string, disputeeID string, disputeeHandle string, buyer string) {
	notif := repo.DisputeOpenNotification{
		ID:             repo.NewNotificationID(),
		Type:           "disputeOpen",
		OrderId:        orderID,
		Thumbnail:      repo.Thumbnail{Tiny: thumbs["tiny"], Small: thumbs["small"]},
		DisputerID:     disputerID,
		DisputerHandle: disputerHandle,
		DisputeeID:     disputeeID,
		DisputeeHandle: disputeeHandle,
		Buyer:          buyer,
	}
	n.Broadcast <- notif
	n.Datastore.Notifications().PutRecord(repo.NewNotification(notif, time.Now(), false))
}

// ProcessDisputeOpen - process an open dispute
func (n *OpenBazaarNode) ProcessDisputeOpen(rc *pb.RicardianContract, openerPeerID string) error {
	DisputeWg.Add(1)
	defer DisputeWg.Done()

	myPeerID := n.IpfsNode.Identity.Pretty()

	if rc.Dispute == nil {
		return errors.New("dispute message is nil")
	}

	contract, err := GetDeserializedContract(rc.Dispute.SerializedContract)
	if err != nil {
		return err
	}

	err = CheckContractFormatting(contract)
	if err != nil {
		return err
	}

	// Calculate the order ID multihash
	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return err
	}

	wal, err := n.Multiwallet.WalletForCurrencyCode(contract.BuyerOrder.Payment.Coin)
	if err != nil {
		return err
	}

	thumbs := GetThumbnails(contract)

	// Figure out what role we have in this dispute and process it

	myRole := GetRole(contract, myPeerID)
	if myRole == "" {
		return errors.New("could not figure out what role matches the peer id")
	}

	var DisputerID string
	var DisputerHandle string
	var DisputeeID string
	var DisputeeHandle string

	vendor := GetVendor(contract)
	vendorHandle := GetVendorHandle(contract)
	buyer := GetBuyer(contract)
	buyerHandle := GetBuyerHandle(contract)
	moderator := GetModerator(contract)

	if myRole == "moderator" {
		validationErrors := n.ValidateCaseContract(contract)
		var err error
		fmt.Println(validationErrors)
		if vendor == openerPeerID {
			DisputerID = vendor
			DisputerHandle = vendorHandle
			DisputeeID = buyer
			DisputeeHandle = buyerHandle

			err = n.UpdateDisputeCase(orderID, pb.OrderState_DISPUTED, false, rc.Dispute, contract, validationErrors)
			if err != nil {
				return err
			}

		} else if buyer == openerPeerID {
			DisputerID = buyer
			DisputerHandle = buyerHandle
			DisputeeID = vendor
			DisputeeHandle = vendorHandle

			err = n.UpdateDisputeCase(orderID, pb.OrderState_DISPUTED, true, rc.Dispute, contract, validationErrors)
			if err != nil {
				return err
			}

		} else {
			return errors.New("peer ID doesn't match either buyer or vendor")
		}
	} else if myRole == "vendor" {
		DisputerID = buyer
		DisputerHandle = buyerHandle
		DisputeeID = vendor
		DisputeeHandle = vendorHandle

		localContract, orderState, txRecords, err := n.GetMatchingSaleForDispute(orderID)
		if err != nil {
			return err
		}

		// Check this order is currently in a state which can be disputed
		if !IsOrderStateDisputableForVendor(orderState) {
			return errors.New("contract can no longer be disputed")
		}

		// Build dispute update message
		payoutAddress := wal.CurrentAddress(wallet.EXTERNAL).EncodeAddress()
		updateMessage, err := GetDisputeUpdateMessage(localContract, orderID, payoutAddress, txRecords)

		// Send the message
		err = n.SendDisputeUpdate(moderator, updateMessage)
		if err != nil {
			return err
		}

		err = n.UpdateDisputeInDatabase(localContract, rc, orderID)
		if err != nil {
			return err
		}

	} else if myRole == "buyer" { // Buyer
		DisputerID = vendor
		DisputerHandle = vendorHandle
		DisputeeID = buyer
		DisputeeHandle = buyerHandle

		localContract, orderState, txRecords, err := n.GetMatchingPurchaseForDispute(orderID)
		if err != nil {
			return err
		}

		if orderState == pb.OrderState_AWAITING_PAYMENT || orderState == pb.OrderState_AWAITING_FULFILLMENT ||
			orderState == pb.OrderState_PARTIALLY_FULFILLED || orderState == pb.OrderState_PENDING {
			return net.OutOfOrderMessage
		}

		// Check this order is currently in a state which can be disputed
		if !IsOrderStateDisputableForBuyer(orderState) {
			return errors.New("contract can no longer be disputed")
		}

		// Build dispute update message
		payoutAddress := wal.CurrentAddress(wallet.EXTERNAL).EncodeAddress()
		updateMessage, err := GetDisputeUpdateMessage(localContract, orderID, payoutAddress, txRecords)

		// Send the message
		err = n.SendDisputeUpdate(moderator, updateMessage)
		if err != nil {
			return err
		}

		err = n.UpdateDisputeInDatabase(localContract, rc, orderID)
		if err != nil {
			return err
		}
	} else {
		return errors.New("we are not involved in this dispute")
	}

	n.SendDisputeNotification(orderID, thumbs, DisputerID, DisputerHandle, DisputeeID, DisputeeHandle, buyer)

	return nil
}

// CloseDispute - close a dispute
func (n *OpenBazaarNode) CloseDispute(orderID string, buyerPercentage, vendorPercentage float32, resolution string, paymentCoinHint *repo.CurrencyCode) error {
	var payDivision = repo.PayoutRatio{Buyer: buyerPercentage, Vendor: vendorPercentage}
	if err := payDivision.Validate(); err != nil {
		return err
	}

	dispute, err := n.Datastore.Cases().GetByCaseID(orderID)
	if err != nil {
		return ErrCaseNotFound
	}

	if dispute.OrderState != pb.OrderState_DISPUTED {
		log.Errorf("unable to resolve expired dispute for order %s", orderID)
		return errors.New("A dispute for this order is not open")
	}
	if dispute.IsExpiredNow() {
		log.Errorf("unable to resolve expired dispute for order %s", orderID)
		return ErrCloseFailureCaseExpired
	}

	var outpoints = dispute.ResolutionPaymentOutpoints(payDivision)
	if outpoints == nil {
		log.Errorf("no outpoints to resolve in dispute for order %s", orderID)
		return ErrCloseFailureNoOutpoints
	}

	if dispute.VendorContract == nil && vendorPercentage > 0 {
		return errors.New("vendor must provide his copy of the contract before you can release funds to the vendor")
	}

	if dispute.BuyerContract == nil {
		dispute.BuyerContract = dispute.VendorContract
	}
	preferredContract := dispute.ResolutionPaymentContract(payDivision)

	// TODO: Remove once broken contracts are migrated
	paymentCoin := preferredContract.BuyerOrder.Payment.Coin
	_, err = repo.LoadCurrencyDefinitions().Lookup(paymentCoin)
	if err != nil {
		log.Warningf("invalid BuyerOrder.Payment.Coin (%s) on order (%s)", paymentCoin, orderID)
		preferredContract.BuyerOrder.Payment.Coin = paymentCoinHint.String()
	}

	var d = new(pb.DisputeResolution)

	// Add timestamp
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	d.Timestamp = ts

	// Add orderId
	d.OrderId = orderID

	// Set self (moderator) as the party that made the resolution proposal
	d.ProposedBy = n.IpfsNode.Identity.Pretty()

	// Set resolution
	d.Resolution = resolution

	var (
		vendorID = preferredContract.VendorListings[0].VendorID.PeerID
		buyerID  = preferredContract.BuyerOrder.BuyerID.PeerID
	)
	buyerKey, err := libp2p.UnmarshalPublicKey(preferredContract.BuyerOrder.BuyerID.Pubkeys.Identity)
	if err != nil {
		return err
	}
	vendorKey, err := libp2p.UnmarshalPublicKey(preferredContract.VendorListings[0].VendorID.Pubkeys.Identity)
	if err != nil {
		return err
	}

	// Calculate total out value
	var totalOut uint64
	for _, o := range outpoints {
		totalOut += o.Value
	}

	wal, err := n.Multiwallet.WalletForCurrencyCode(preferredContract.BuyerOrder.Payment.Coin)
	if err != nil {
		return err
	}

	// Create outputs using full value. We will subtract the fee off each output later.
	outMap := make(map[string]wallet.TransactionOutput)
	var outputs []wallet.TransactionOutput
	var modAddr btcutil.Address
	var modValue uint64
	modAddr = wal.CurrentAddress(wallet.EXTERNAL)
	modValue, err = n.GetModeratorFee(totalOut, preferredContract.BuyerOrder.Payment.Coin, wal.CurrencyCode())
	if err != nil {
		return err
	}
	if modValue > 0 {
		out := wallet.TransactionOutput{
			Address: modAddr,
			Value:   int64(modValue),
		}
		outputs = append(outputs, out)
		outMap["moderator"] = out
	}

	var buyerAddr btcutil.Address
	var buyerValue uint64
	if payDivision.BuyerAny() {
		buyerAddr, err = wal.DecodeAddress(dispute.BuyerPayoutAddress)
		if err != nil {
			return err
		}
		buyerValue = uint64((float64(totalOut) - float64(modValue)) * (float64(buyerPercentage) / 100))
		out := wallet.TransactionOutput{
			Address: buyerAddr,
			Value:   int64(buyerValue),
		}
		outputs = append(outputs, out)
		outMap["buyer"] = out
	}
	var vendorAddr btcutil.Address
	var vendorValue uint64
	if payDivision.VendorAny() {
		vendorAddr, err = wal.DecodeAddress(dispute.VendorPayoutAddress)
		if err != nil {
			return err
		}
		vendorValue = uint64((float64(totalOut) - float64(modValue)) * (float64(vendorPercentage) / 100))
		out := wallet.TransactionOutput{
			Address: vendorAddr,
			Value:   int64(vendorValue),
		}
		outputs = append(outputs, out)
		outMap["vendor"] = out
	}

	if len(outputs) == 0 {
		return errors.New("transaction has no outputs")
	}

	// Create inputs
	var inputs []wallet.TransactionInput
	for _, o := range outpoints {
		decodedHash, err := hex.DecodeString(o.Hash)
		if err != nil {
			return err
		}
		input := wallet.TransactionInput{
			OutpointHash:  decodedHash,
			OutpointIndex: o.Index,
			Value:         int64(o.Value),
		}
		inputs = append(inputs, input)
	}

	if len(inputs) == 0 {
		return errors.New("transaction has no inputs")
	}

	// Calculate total fee
	defaultFee := wal.GetFeePerByte(wallet.NORMAL)
	txFee := wal.EstimateFee(inputs, outputs, dispute.ResolutionPaymentFeePerByte(payDivision, defaultFee))

	// Subtract fee from each output in proportion to output value
	var outs []wallet.TransactionOutput
	for role, output := range outMap {
		outPercentage := float64(output.Value) / float64(totalOut)
		outputShareOfFee := outPercentage * float64(txFee)
		val := output.Value - int64(outputShareOfFee)
		if !wal.IsDust(val) {
			o := wallet.TransactionOutput{
				Value:   val,
				Address: output.Address,
				Index:   output.Index,
			}
			outs = append(outs, o)
		} else {
			delete(outMap, role)
		}
	}

	// Create moderator key
	chaincode := preferredContract.BuyerOrder.Payment.Chaincode
	chaincodeBytes, err := hex.DecodeString(chaincode)
	if err != nil {
		return err
	}
	mPrivKey := n.MasterPrivateKey
	if err != nil {
		return err
	}
	mECKey, err := mPrivKey.ECPrivKey()
	if err != nil {
		return err
	}
	moderatorKey, err := wal.ChildKey(mECKey.Serialize(), chaincodeBytes, true)
	if err != nil {
		return err
	}

	// Sign buyer rating key
	if dispute.BuyerContract != nil {
		ecPriv, err := moderatorKey.ECPrivKey()
		if err != nil {
			return err
		}
		for _, key := range dispute.BuyerContract.BuyerOrder.RatingKeys {
			hashed := sha256.Sum256(key)
			sig, err := ecPriv.Sign(hashed[:])
			if err != nil {
				return err
			}
			d.ModeratorRatingSigs = append(d.ModeratorRatingSigs, sig.Serialize())
		}
	}

	// Create signatures
	redeemScript := preferredContract.BuyerOrder.Payment.RedeemScript
	redeemScriptBytes, err := hex.DecodeString(redeemScript)
	if err != nil {
		return err
	}
	sigs, err := wal.CreateMultisigSignature(inputs, outs, moderatorKey, redeemScriptBytes, 0)
	if err != nil {
		return err
	}
	var bitcoinSigs []*pb.BitcoinSignature
	for _, sig := range sigs {
		s := new(pb.BitcoinSignature)
		s.InputIndex = sig.InputIndex
		s.Signature = sig.Signature
		bitcoinSigs = append(bitcoinSigs, s)
	}

	// Create payout object
	payout := new(pb.DisputeResolution_Payout)
	payout.Inputs = outpoints
	payout.Sigs = bitcoinSigs
	if _, ok := outMap["buyer"]; ok {
		outputShareOfFee := (float64(buyerValue) / float64(totalOut)) * float64(txFee)
		amt := int64(buyerValue) - int64(outputShareOfFee)
		if amt < 0 {
			amt = 0
		}
		payout.BuyerOutput = &pb.DisputeResolution_Payout_Output{ScriptOrAddress: &pb.DisputeResolution_Payout_Output_Address{buyerAddr.String()}, Amount: uint64(amt)}
	}
	if _, ok := outMap["vendor"]; ok {
		outputShareOfFee := (float64(vendorValue) / float64(totalOut)) * float64(txFee)
		amt := int64(vendorValue) - int64(outputShareOfFee)
		if amt < 0 {
			amt = 0
		}
		payout.VendorOutput = &pb.DisputeResolution_Payout_Output{ScriptOrAddress: &pb.DisputeResolution_Payout_Output_Address{vendorAddr.String()}, Amount: uint64(amt)}
	}
	if _, ok := outMap["moderator"]; ok {
		outputShareOfFee := (float64(modValue) / float64(totalOut)) * float64(txFee)
		amt := int64(modValue) - int64(outputShareOfFee)
		if amt < 0 {
			amt = 0
		}
		payout.ModeratorOutput = &pb.DisputeResolution_Payout_Output{ScriptOrAddress: &pb.DisputeResolution_Payout_Output_Address{modAddr.String()}, Amount: uint64(amt)}
	}

	d.Payout = payout

	rc := new(pb.RicardianContract)
	rc.DisputeResolution = d
	rc, err = n.SignDisputeResolution(rc)
	if err != nil {
		return err
	}

	err = n.SendDisputeClose(buyerID, &buyerKey, rc)
	if err != nil {
		return err
	}
	err = n.SendDisputeClose(vendorID, &vendorKey, rc)
	if err != nil {
		return err
	}

	err = n.Datastore.Cases().MarkAsClosed(orderID, d)
	if err != nil {
		return err
	}
	return nil
}

// SignDisputeResolution - add signature to DisputeResolution
func (n *OpenBazaarNode) SignDisputeResolution(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedDR, err := proto.Marshal(contract.DisputeResolution)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_DISPUTE_RESOLUTION
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedDR)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

// ValidateCaseContract - validate contract details
func (n *OpenBazaarNode) ValidateCaseContract(contract *pb.RicardianContract) []string {
	var validationErrors []string

	if !HasListings(contract) {
		validationErrors = append(validationErrors, "Contract contains no listings")
	}

	if !ContractHasBuyerOrder(contract) {
		validationErrors = append(validationErrors, "Contract is missing the buyer's order")
	}

	if ContractIsMissingVendorID(contract) {
		validationErrors = append(validationErrors, "The listing is missing the vendor ID information. Unable to validate any signatures.")
		return validationErrors
	}

	if ContractIsMissingBuyerID(contract) {
		validationErrors = append(validationErrors, "The listing is missing the buyer ID information. Unable to validate any signatures.")
		return validationErrors
	}

	if ContractIsMissingPayment(contract) {
		validationErrors = append(validationErrors, "The buyer's order is missing the payment section")
	}

	// There needs to be one listing for each unique item in the order
	unmatchedListingHashes := GetUnmatchedListingHashes(contract)
	if len(unmatchedListingHashes) > 0 {
		validationErrors = append(validationErrors, "Not all items in the order have a matching vendor listing")
	}

	// There needs to be one listing signature for each listing
	invalidSignatureErrors := CheckListingSignatures(contract)
	if len(invalidSignatureErrors) > 0 {
		validationErrors = append(validationErrors, invalidSignatureErrors...)
	}

	// Verify the order signature
	if !ValidBuyerOrderSignature(contract) {
		validationErrors = append(validationErrors, "Invalid buyer signature on order")
	}

	// Verify the order confirmation signature
	if !ValidOrderConfirmationSignature(contract) {
		validationErrors = append(validationErrors, "Invalid vendor signature on order confirmation")
	}

	// There should be one fulfillment signature for each vendorOrderFulfilment object
	fulfillmentSigs := GetFulfillmentSignatures(contract)
	fulfillments := GetFulfillments(contract)

	if len(fulfillmentSigs) != len(fulfillments) {
		validationErrors = append(validationErrors, "Not all order fulfillments are signed by the vendor")
	}

	// Verify the signature of the order fulfillments
	fulfillmentSignatureErrors := CheckFulfillmentSignatures(contract)
	if len(fulfillmentSignatureErrors) > 0 {
		validationErrors = append(validationErrors, fulfillmentSignatureErrors...)
	}

	// Verify the buyer's bitcoin signature on his guid
	if err := verifyBitcoinSignature(
		contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin,
		contract.BuyerOrder.BuyerID.BitcoinSig,
		contract.BuyerOrder.BuyerID.PeerID,
	); err != nil {
		validationErrors = append(validationErrors, "The buyer's bitcoin signature which covers his guid is invalid. This could be an attempt to forge the buyer's identity.")
	}

	// Verify the vendor's bitcoin signature on his guid
	if err := verifyBitcoinSignature(
		contract.VendorListings[0].VendorID.Pubkeys.Bitcoin,
		contract.VendorListings[0].VendorID.BitcoinSig,
		contract.VendorListings[0].VendorID.PeerID,
	); err != nil {
		validationErrors = append(validationErrors, "The vendor's bitcoin signature which covers his guid is invalid. This could be an attempt to forge the vendor's identity.")
	}

	// Verify the redeem script matches all the bitcoin keys
	if contract.BuyerOrder.Payment != nil {
		wal, err := n.Multiwallet.WalletForCurrencyCode(contract.BuyerOrder.Payment.Coin)
		if err != nil {
			validationErrors = append(validationErrors, "Contract uses a coin not found in wallet")
			return validationErrors
		}
		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		mECKey, err := n.MasterPrivateKey.ECPubKey()
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		moderatorKey, err := wal.ChildKey(mECKey.SerializeCompressed(), chaincode, false)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		buyerKey, err := wal.ChildKey(contract.BuyerOrder.BuyerID.Pubkeys.Bitcoin, chaincode, false)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		vendorKey, err := wal.ChildKey(contract.VendorListings[0].VendorID.Pubkeys.Bitcoin, chaincode, false)
		if err != nil {
			validationErrors = append(validationErrors, "Error validating bitcoin address and redeem script")
			return validationErrors
		}
		timeout, _ := time.ParseDuration(strconv.Itoa(int(contract.VendorListings[0].Metadata.EscrowTimeoutHours)) + "h")
		addr, redeemScript, err := wal.GenerateMultisigScript([]hd.ExtendedKey{*buyerKey, *vendorKey, *moderatorKey}, 2, timeout, vendorKey)
		if err != nil {
			validationErrors = append(validationErrors, "Error generating multisig script")
			return validationErrors
		}

		if contract.BuyerOrder.Payment.Address != addr.EncodeAddress() {
			validationErrors = append(validationErrors, "The calculated bitcoin address doesn't match the address in the order")
		}

		if hex.EncodeToString(redeemScript) != contract.BuyerOrder.Payment.RedeemScript {
			validationErrors = append(validationErrors, "The calculated redeem script doesn't match the redeem script in the order")
		}
	}

	return validationErrors
}

// ValidateDisputeResolution - validate dispute resolution
func (n *OpenBazaarNode) ValidateDisputeResolution(contract *pb.RicardianContract) error {
	err := n.verifySignatureOnDisputeResolution(contract)
	if err != nil {
		return err
	}
	if contract.DisputeResolution.Payout == nil || len(contract.DisputeResolution.Payout.Sigs) == 0 {
		return errors.New("DisputeResolution contains invalid payout")
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(contract.BuyerOrder.Payment.Coin)
	if err != nil {
		return err
	}

	if contract.VendorListings[0].VendorID.PeerID == n.IpfsNode.Identity.Pretty() && contract.DisputeResolution.Payout.VendorOutput != nil {
		return n.verifyPaymentDestinationIsInWallet(contract.DisputeResolution.Payout.VendorOutput, wal)
	} else if contract.BuyerOrder.BuyerID.PeerID == n.IpfsNode.Identity.Pretty() && contract.DisputeResolution.Payout.BuyerOutput != nil {
		return n.verifyPaymentDestinationIsInWallet(contract.DisputeResolution.Payout.BuyerOutput, wal)
	}
	return nil
}

func (n *OpenBazaarNode) verifyPaymentDestinationIsInWallet(output *pb.DisputeResolution_Payout_Output, wal wallet.Wallet) error {
	addr, err := pb.DisputeResolutionPayoutOutputToAddress(wal, output)
	if err != nil {
		return err
	}

	if !wal.HasKey(addr) {
		return errors.New("moderator dispute resolution payout address is not defined in your wallet to recieve funds")
	}
	return nil
}

func (n *OpenBazaarNode) verifySignatureOnDisputeResolution(contract *pb.RicardianContract) error {

	moderatorID, err := peer.IDB58Decode(contract.BuyerOrder.Payment.Moderator)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pubkey, err := n.IpfsNode.Routing.(*dht.IpfsDHT).GetPublicKey(ctx, moderatorID)
	if err != nil {
		log.Errorf("Failed to find public key for %s", moderatorID.Pretty())
		return err
	}
	pubKeyBytes, err := pubkey.Bytes()
	if err != nil {
		return err
	}

	if err := verifyMessageSignature(
		contract.DisputeResolution,
		pubKeyBytes,
		contract.Signatures,
		pb.Signature_DISPUTE_RESOLUTION,
		moderatorID.Pretty(),
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("contract does not contain a signature for the dispute resolution")
		case invalidSigError:
			return errors.New("guid signature on contact failed to verify")
		case matchKeyError:
			return errors.New("public key in dispute does not match reported ID")
		default:
			return err
		}
	}
	return nil
}

// ReleaseFunds - release funds
func (n *OpenBazaarNode) ReleaseFunds(contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {
	// Create inputs
	var inputs []wallet.TransactionInput
	for _, o := range contract.DisputeResolution.Payout.Inputs {
		decodedHash, err := hex.DecodeString(o.Hash)
		if err != nil {
			return err
		}
		input := wallet.TransactionInput{
			OutpointHash:  decodedHash,
			OutpointIndex: o.Index,
			Value:         int64(o.Value),
		}
		inputs = append(inputs, input)
	}

	if len(inputs) == 0 {
		return errors.New("transaction has no inputs")
	}
	wal, err := n.Multiwallet.WalletForCurrencyCode(contract.BuyerOrder.Payment.Coin)
	if err != nil {
		return err
	}

	// Create outputs
	var outputs []wallet.TransactionOutput
	if contract.DisputeResolution.Payout.BuyerOutput != nil {
		addr, err := pb.DisputeResolutionPayoutOutputToAddress(wal, contract.DisputeResolution.Payout.BuyerOutput)
		if err != nil {
			return err
		}
		output := wallet.TransactionOutput{
			Address: addr,
			Value:   int64(contract.DisputeResolution.Payout.BuyerOutput.Amount),
		}
		outputs = append(outputs, output)
	}
	if contract.DisputeResolution.Payout.VendorOutput != nil {
		addr, err := pb.DisputeResolutionPayoutOutputToAddress(wal, contract.DisputeResolution.Payout.VendorOutput)
		if err != nil {
			return err
		}
		output := wallet.TransactionOutput{
			Address: addr,
			Value:   int64(contract.DisputeResolution.Payout.VendorOutput.Amount),
		}
		outputs = append(outputs, output)
	}
	if contract.DisputeResolution.Payout.ModeratorOutput != nil {
		addr, err := pb.DisputeResolutionPayoutOutputToAddress(wal, contract.DisputeResolution.Payout.ModeratorOutput)
		if err != nil {
			return err
		}
		output := wallet.TransactionOutput{
			Address: addr,
			Value:   int64(contract.DisputeResolution.Payout.ModeratorOutput.Amount),
		}
		outputs = append(outputs, output)
	}

	// Create signing key
	chaincodeBytes, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
	if err != nil {
		return err
	}
	mPrivKey := n.MasterPrivateKey
	if err != nil {
		return err
	}
	mECKey, err := mPrivKey.ECPrivKey()
	if err != nil {
		return err
	}
	signingKey, err := wal.ChildKey(mECKey.Serialize(), chaincodeBytes, true)
	if err != nil {
		return err
	}

	// Create signatures
	redeemScriptBytes, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
	if err != nil {
		return err
	}
	mySigs, err := wal.CreateMultisigSignature(inputs, outputs, signingKey, redeemScriptBytes, 0)
	if err != nil {
		return err
	}

	var moderatorSigs []wallet.Signature
	for _, sig := range contract.DisputeResolution.Payout.Sigs {
		s := wallet.Signature{
			Signature:  sig.Signature,
			InputIndex: sig.InputIndex,
		}
		moderatorSigs = append(moderatorSigs, s)
	}

	accept := new(pb.DisputeAcceptance)
	// Create timestamp
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	accept.Timestamp = ts
	accept.ClosedBy = n.IpfsNode.Identity.Pretty()
	contract.DisputeAcceptance = accept

	orderID, err := n.CalcOrderID(contract.BuyerOrder)
	if err != nil {
		return err
	}

	// Update database
	if n.IpfsNode.Identity.Pretty() == contract.BuyerOrder.BuyerID.PeerID {
		err = n.Datastore.Purchases().Put(orderID, *contract, pb.OrderState_DECIDED, true)
	} else {
		err = n.Datastore.Sales().Put(orderID, *contract, pb.OrderState_DECIDED, true)
	}
	if err != nil {
		log.Errorf("ReleaseFunds error updating database: %s", err.Error())
	}

	// Build, sign, and broadcast transaction
	_, err = wal.Multisign(inputs, outputs, mySigs, moderatorSigs, redeemScriptBytes, 0, true)
	if err != nil {
		return err
	}

	return nil
}
