package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	libp2p "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"io/ioutil"
	"os"
	"path"
	"time"

	"fmt"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
)

const (
	RatingMin           = 1
	RatingMax           = 5
	ReviewMaxCharacters = 3000
)

type OrderRatings struct {
	OrderId string       `json:"orderId"`
	Ratings []RatingData `json:"ratings"`
}

type RatingData struct {
	Slug            string `json:"slug"`
	Overall         int    `json:"overall"`
	Quality         int    `json:"quality"`
	Description     int    `json:"description"`
	DeliverySpeed   int    `json:"deliverySpeed"`
	CustomerService int    `json:"customerService"`
	Review          string `json:"review"`
	Anonymous       bool   `json:"anonymous"`
}

type SavedRating struct {
	Slug    string   `json:"slug"`
	Count   int      `json:"count"`
	Average float32  `json:"average"`
	Ratings []string `json:"ratings"`
}

func (n *OpenBazaarNode) CompleteOrder(orderRatings *OrderRatings, contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {

	orderId, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}

	oc := new(pb.OrderCompletion)
	oc.OrderId = orderId
	oc.Ratings = []*pb.Rating{}

	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	oc.Timestamp = ts

	for z, r := range orderRatings.Ratings {
		rating := new(pb.Rating)
		rd := new(pb.Rating_RatingData)

		var rs *pb.RatingSignature
		var rk []byte
		if contract.DisputeResolution != nil {
			if contract.VendorOrderConfirmation == nil {
				return errors.New("Cannot leave review because the vendor never accepted this order")
			}
			for _, sig := range contract.VendorOrderConfirmation.RatingSignatures {
				if sig.Metadata.ListingSlug == r.Slug {
					rs = sig
					break
				}
			}
			if len(contract.BuyerOrder.RatingKeys) < len(orderRatings.Ratings) {
				return errors.New("Invalid number of rating keys in buyer order")
			}
			rk = contract.BuyerOrder.RatingKeys[z]

			for i, l := range contract.VendorListings {
				if l.Slug == r.Slug && i <= len(contract.DisputeResolution.ModeratorRatingSigs)-1 {
					rd.ModeratorSig = contract.DisputeResolution.ModeratorRatingSigs[i]
					break
				}
			}
		} else {
			for _, fulfillment := range contract.VendorOrderFulfillment {
				if fulfillment.RatingSignature.Metadata.ListingSlug == r.Slug {
					rs = fulfillment.RatingSignature
					break
				}
			}
			rk = rs.Metadata.RatingKey
		}

		rd.RatingKey = rk
		if !r.Anonymous {
			profile, _ := n.GetProfile()
			rd.BuyerID = contract.BuyerOrder.BuyerID
			rd.BuyerName = profile.Name
			sig, err := n.IpfsNode.PrivateKey.Sign(rd.RatingKey)
			if err != nil {
				return err
			}
			rd.BuyerSig = sig
		}
		rd.VendorID = contract.VendorListings[0].VendorID
		rd.VendorSig = rs

		rd.Overall = uint32(r.Overall)
		rd.Quality = uint32(r.Quality)
		rd.Description = uint32(r.Description)
		rd.CustomerService = uint32(r.CustomerService)
		rd.DeliverySpeed = uint32(r.DeliverySpeed)
		rd.Review = r.Review

		ts, err := ptypes.TimestampProto(time.Now())
		if err != nil {
			return err
		}
		rd.Timestamp = ts
		rating.RatingData = rd

		ser, err := proto.Marshal(rating.RatingData)
		if err != nil {
			return err
		}

		ratingKey, err := n.Wallet.MasterPrivateKey().Child(uint32(contract.BuyerOrder.Timestamp.Seconds))
		if err != nil {
			return err
		}
		ecRatingKey, err := ratingKey.ECPrivKey()
		if err != nil {
			return err
		}
		hashed := sha256.Sum256(ser)
		sig, err := ecRatingKey.Sign(hashed[:])
		if err != nil {
			return err
		}
		rating.Signature = sig.Serialize()
		oc.Ratings = append(oc.Ratings, rating)
	}

	// Payout order if moderated and not disputed
	if contract.BuyerOrder.Payment.Method == pb.Order_Payment_MODERATED && contract.DisputeResolution == nil {
		var ins []wallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return err
				}
				outValue += r.Value
				in := wallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash, Value: r.Value}
				ins = append(ins, in)
			}
		}

		payoutAddress, err := n.Wallet.DecodeAddress(contract.VendorOrderFulfillment[0].Payout.PayoutAddress)
		if err != nil {
			return err
		}
		var output wallet.TransactionOutput
		outputScript, err := n.Wallet.AddressToScript(payoutAddress)
		if err != nil {
			return err
		}
		output.ScriptPubKey = outputScript
		output.Value = outValue

		chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
		if err != nil {
			return err
		}
		parentFP := []byte{0x00, 0x00, 0x00, 0x00}
		mPrivKey := n.Wallet.MasterPrivateKey()
		if err != nil {
			return err
		}
		mECKey, err := mPrivKey.ECPrivKey()
		if err != nil {
			return err
		}
		hdKey := hd.NewExtendedKey(
			n.Wallet.Params().HDPrivateKeyID[:],
			mECKey.Serialize(),
			chaincode,
			parentFP,
			0,
			0,
			true)

		buyerKey, err := hdKey.Child(0)
		if err != nil {
			return err
		}
		redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
		if err != nil {
			return err
		}

		buyerSignatures, err := n.Wallet.CreateMultisigSignature(ins, []wallet.TransactionOutput{output}, buyerKey, redeemScript, contract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte)
		if err != nil {
			return err
		}
		var pbSigs []*pb.BitcoinSignature
		for _, s := range buyerSignatures {
			sig := new(pb.BitcoinSignature)
			sig.InputIndex = s.InputIndex
			sig.Signature = s.Signature
			pbSigs = append(pbSigs, sig)
		}
		oc.PayoutSigs = pbSigs
		var vendorSignatures []wallet.Signature
		for _, s := range contract.VendorOrderFulfillment[0].Payout.Sigs {
			sig := wallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		_, err = n.Wallet.Multisign(ins, []wallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte, true)
		if err != nil {
			return err
		}
	}

	rc := new(pb.RicardianContract)
	rc.BuyerOrderCompletion = oc

	rc, err = n.SignOrderCompletion(rc)
	if err != nil {
		return err
	}
	vendorkey, err := libp2p.UnmarshalPublicKey(contract.VendorListings[0].VendorID.Pubkeys.Identity)
	if err != nil {
		return err
	}
	err = n.SendOrderCompletion(contract.VendorListings[0].VendorID.PeerID, &vendorkey, rc)
	if err != nil {
		return err
	}
	contract.BuyerOrderCompletion = oc
	for _, sig := range rc.Signatures {
		if sig.Section == pb.Signature_ORDER_COMPLETION {
			contract.Signatures = append(contract.Signatures, sig)
		}
	}
	err = n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_COMPLETED, true)
	if err != nil {
		return err
	}

	return nil
}

var EscrowTimeLockedError error

func (n *OpenBazaarNode) ReleaseFundsAfterTimeout(contract *pb.RicardianContract, records []*wallet.TransactionRecord) error {
	minConfirms := contract.VendorListings[0].Metadata.EscrowTimeoutHours * 6
	var utxos []wallet.Utxo
	for _, r := range records {
		if !r.Spent && r.Value > 0 {
			var utxo wallet.Utxo
			utxo.Value = r.Value

			hash, err := chainhash.NewHashFromStr(r.Txid)
			if err != nil {
				return err
			}

			confirms, _, err := n.Wallet.GetConfirmations(*hash)
			if err != nil {
				return err
			}
			if confirms < minConfirms {
				EscrowTimeLockedError = fmt.Errorf("Tx %s needs %d more confirmations before it can be spent", r.Txid, int(minConfirms-confirms))
				return EscrowTimeLockedError
			}
			outpoint := wire.NewOutPoint(hash, r.Index)
			utxo.Op = *outpoint
			utxos = append(utxos, utxo)
		}
	}

	chaincode, err := hex.DecodeString(contract.BuyerOrder.Payment.Chaincode)
	if err != nil {
		return err
	}
	parentFP := []byte{0x00, 0x00, 0x00, 0x00}
	mPrivKey := n.Wallet.MasterPrivateKey()
	if err != nil {
		return err
	}
	mECKey, err := mPrivKey.ECPrivKey()
	if err != nil {
		return err
	}
	hdKey := hd.NewExtendedKey(
		n.Wallet.Params().HDPrivateKeyID[:],
		mECKey.Serialize(),
		chaincode,
		parentFP,
		0,
		0,
		true)

	vendorKey, err := hdKey.Child(0)
	if err != nil {
		return err
	}
	redeemScript, err := hex.DecodeString(contract.BuyerOrder.Payment.RedeemScript)
	if err != nil {
		return err
	}
	_, err = n.Wallet.SweepAddress(utxos, nil, vendorKey, &redeemScript, wallet.NORMAL)
	if err != nil {
		return err
	}

	orderId, err := n.CalcOrderId(contract.BuyerOrder)
	if err != nil {
		return err
	}

	err = n.Datastore.Purchases().Put(orderId, *contract, pb.OrderState_PAYMENT_FINALIZED, true)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) SignOrderCompletion(contract *pb.RicardianContract) (*pb.RicardianContract, error) {
	serializedOrderFulfil, err := proto.Marshal(contract.BuyerOrderCompletion)
	if err != nil {
		return contract, err
	}
	s := new(pb.Signature)
	s.Section = pb.Signature_ORDER_COMPLETION
	if err != nil {
		return contract, err
	}
	guidSig, err := n.IpfsNode.PrivateKey.Sign(serializedOrderFulfil)
	if err != nil {
		return contract, err
	}
	s.SignatureBytes = guidSig
	contract.Signatures = append(contract.Signatures, s)
	return contract, nil
}

func (n *OpenBazaarNode) ValidateOrderCompletion(contract *pb.RicardianContract) error {
	if err := verifySignaturesOnOrderCompletion(contract); err != nil {
		return err
	}

	// TODO: validate bitcoin signatures of moderated

	return nil
}

func (n *OpenBazaarNode) ValidateAndSaveRating(contract *pb.RicardianContract) (retErr error) {
	for _, rating := range contract.BuyerOrderCompletion.Ratings {
		valid, err := ValidateRating(rating)
		if !valid || err != nil {
			retErr = err
			continue
		}

		if rating.RatingData.Overall < RatingMin || rating.RatingData.Overall > RatingMax {
			retErr = err
			continue
		}
		if rating.RatingData.Quality < RatingMin || rating.RatingData.Quality > RatingMax {
			retErr = err
			continue
		}
		if rating.RatingData.Description < RatingMin || rating.RatingData.Description > RatingMax {
			retErr = err
			continue
		}
		if rating.RatingData.DeliverySpeed < RatingMin || rating.RatingData.DeliverySpeed > RatingMax {
			retErr = err
			continue
		}
		if rating.RatingData.CustomerService < RatingMin || rating.RatingData.CustomerService > RatingMax {
			retErr = err
			continue
		}

		m := jsonpb.Marshaler{
			EnumsAsInts:  false,
			EmitDefaults: false,
			Indent:       "    ",
			OrigName:     false,
		}
		ratingJson, err := m.MarshalToString(rating)
		if err != nil {
			retErr = err
			continue
		}

		mh, err := EncodeMultihash([]byte(ratingJson))
		if err != nil {
			retErr = err
			continue
		}

		ratingPath := path.Join(n.RepoPath, "root", "ratings", mh.B58String()[:12]+".json")
		f, err := os.Create(ratingPath)
		if err != nil {
			retErr = err
			continue
		}
		defer f.Close()

		go ipfs.AddFile(n.Context, ratingPath)

		_, werr := f.Write([]byte(ratingJson))
		if werr != nil {
			retErr = err
			continue
		}

		if err := n.updateRatingIndex(rating, ratingPath); err != nil {
			retErr = err
			continue
		}

		if err := n.updateRatingInListingIndex(rating); err != nil {
			retErr = err
			continue
		}

		if err := n.updateProfileRatings(rating); err != nil {
			retErr = err
			continue
		}
	}
	return
}

func (n *OpenBazaarNode) updateRatingIndex(rating *pb.Rating, ratingPath string) error {
	indexPath := path.Join(n.RepoPath, "root", "ratings.json")

	var index []SavedRating

	ratingHash, err := ipfs.GetHashOfFile(n.Context, ratingPath)
	if err != nil {
		return err
	}

	_, ferr := os.Stat(indexPath)
	if !os.IsNotExist(ferr) {
		// Read existing file
		file, err := ioutil.ReadFile(indexPath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(file, &index)
		if err != nil {
			return err
		}
	}

	// Check to see if the rating we are adding already exists in the list. If so update it.
	exists := false
	for i, d := range index {
		if rating.RatingData.VendorSig.Metadata.ListingSlug == d.Slug {
			index[i].Ratings = append(index[i].Ratings, ratingHash)
			total := index[i].Average * float32(index[i].Count)
			total += float32(rating.RatingData.Overall)
			index[i].Count += 1
			index[i].Average = total / float32(index[i].Count)
			exists = true
			break
		}
	}

	// If it doesn't exist create a new one
	if !exists {
		rs := SavedRating{
			Slug:    rating.RatingData.VendorSig.Metadata.ListingSlug,
			Average: float32(rating.RatingData.Overall),
			Count:   1,
			Ratings: []string{ratingHash},
		}
		index = append(index, rs)
	}

	// Write it back to file
	f, err := os.Create(indexPath)
	defer f.Close()
	if err != nil {
		return err
	}
	j, jerr := json.MarshalIndent(index, "", "    ")
	if jerr != nil {
		return jerr
	}
	_, werr := f.Write(j)
	if werr != nil {
		return werr
	}
	return nil
}

func verifySignaturesOnOrderCompletion(contract *pb.RicardianContract) error {
	if err := verifyMessageSignature(
		contract.BuyerOrderCompletion,
		contract.BuyerOrder.BuyerID.Pubkeys.Identity,
		contract.Signatures,
		pb.Signature_ORDER_COMPLETION,
		contract.BuyerOrder.BuyerID.PeerID,
	); err != nil {
		switch err.(type) {
		case noSigError:
			return errors.New("Contract does not contain a signature for the order completion")
		case invalidSigError:
			return errors.New("Buyer's guid signature on contact failed to verify")
		case matchKeyError:
			return errors.New("Public key in order does not match reported buyer ID")
		default:
			return err
		}
	}
	return nil
}
