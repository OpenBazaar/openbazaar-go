package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	libp2p "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	"gx/ipfs/QmbZ6Cee2uHjG7hf19qLHppgKDRtaG4CVtMzdmK9VCVqLu/go-multihash"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/btcec"
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

func (n *OpenBazaarNode) CompleteOrder(orderRatings *OrderRatings, contract *pb.RicardianContract, records []*spvwallet.TransactionRecord) error {

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

	for _, r := range orderRatings.Ratings {
		rating := new(pb.Rating)
		rd := new(pb.Rating_RatingData)

		var rs *pb.RatingSignature
		if contract.DisputeResolution != nil {
			for _, sig := range contract.VendorOrderConfirmation.RatingSignatures {
				if sig.Metadata.ListingSlug == r.Slug {
					rs = sig
					break
				}
			}
			for i, l := range contract.VendorListings {
				if l.Slug == r.Slug && i <= len(contract.DisputeResolution.ModeratorRatingSigs)-1 {
					rd.ModeratorSig = contract.DisputeResolution.ModeratorRatingSigs[i]
					break
				}
			}
			moderatorID := &pb.ID{
				PeerID: contract.BuyerOrder.Payment.Moderator,
				Pubkeys: &pb.ID_Pubkeys{
					Identity: rs.Metadata.ModeratorKey,
				},
			}
			rd.ModeratorID = moderatorID
		} else {
			for _, fulfillment := range contract.VendorOrderFulfillment {
				if fulfillment.RatingSignature.Metadata.ListingSlug == r.Slug {
					rs = fulfillment.RatingSignature
					break
				}
			}
		}

		rd.RatingKey = rs.Metadata.RatingKey
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
		var ins []spvwallet.TransactionInput
		var outValue int64
		for _, r := range records {
			if !r.Spent && r.Value > 0 {
				outpointHash, err := hex.DecodeString(r.Txid)
				if err != nil {
					return err
				}
				outValue += r.Value
				in := spvwallet.TransactionInput{OutpointIndex: r.Index, OutpointHash: outpointHash}
				ins = append(ins, in)
			}
		}

		payoutAddress, err := n.Wallet.DecodeAddress(contract.VendorOrderFulfillment[0].Payout.PayoutAddress)
		if err != nil {
			return err
		}
		var output spvwallet.TransactionOutput
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

		buyerSignatures, err := n.Wallet.CreateMultisigSignature(ins, []spvwallet.TransactionOutput{output}, buyerKey, redeemScript, contract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte)
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
		var vendorSignatures []spvwallet.Signature
		for _, s := range contract.VendorOrderFulfillment[0].Payout.Sigs {
			sig := spvwallet.Signature{InputIndex: s.InputIndex, Signature: s.Signature}
			vendorSignatures = append(vendorSignatures, sig)
		}
		_, err = n.Wallet.Multisign(ins, []spvwallet.TransactionOutput{output}, buyerSignatures, vendorSignatures, redeemScript, contract.VendorOrderFulfillment[0].Payout.PayoutFeePerByte, true)
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

func (n *OpenBazaarNode) ValidateAndSaveRating(contract *pb.RicardianContract) error {
	for _, rating := range contract.BuyerOrderCompletion.Ratings {
		pubkey, err := btcec.ParsePubKey(rating.RatingData.RatingKey, btcec.S256())
		if err != nil {
			return err
		}

		signature, err := btcec.ParseSignature(rating.Signature, btcec.S256())
		if err != nil {
			return err
		}

		ser, err := proto.Marshal(rating.RatingData)
		if err != nil {
			return err
		}
		hashed := sha256.Sum256(ser)
		verified := signature.Verify(hashed[:], pubkey)

		if !verified {
			return errors.New("Invalid rating signature on rating")
		}

		ratingSigData, err := proto.Marshal(rating.RatingData.VendorSig.Metadata)
		if err != nil {
			return err
		}
		valid, err := n.IpfsNode.PrivateKey.GetPublic().Verify(ratingSigData, rating.RatingData.VendorSig.Signature)
		if !valid {
			return errors.New("Invalid vendor signature on rating")
		}

		if rating.RatingData.Overall < RatingMin || rating.RatingData.Overall > RatingMax {
			return errors.New("Rating not within valid range")
		}
		if rating.RatingData.Quality < RatingMin || rating.RatingData.Quality > RatingMax {
			return errors.New("Rating not within valid range")
		}
		if rating.RatingData.Description < RatingMin || rating.RatingData.Description > RatingMax {
			return errors.New("Rating not within valid range")
		}
		if rating.RatingData.DeliverySpeed < RatingMin || rating.RatingData.DeliverySpeed > RatingMax {
			return errors.New("Rating not within valid range")
		}
		if rating.RatingData.CustomerService < RatingMin || rating.RatingData.CustomerService > RatingMax {
			return errors.New("Rating not within valid range")
		}

		m := jsonpb.Marshaler{
			EnumsAsInts:  false,
			EmitDefaults: false,
			Indent:       "    ",
			OrigName:     false,
		}
		ratingJson, err := m.MarshalToString(rating)
		if err != nil {
			return err
		}

		sha := sha256.Sum256([]byte(ratingJson))
		h, err := multihash.Encode(sha[:], multihash.SHA2_256)
		if err != nil {
			return err
		}

		mh, err := multihash.Cast(h)
		if err != nil {
			return err
		}

		ratingPath := path.Join(n.RepoPath, "root", "ratings", "rating_"+mh.B58String()[:12])
		f, err := os.Create(ratingPath)
		if err != nil {
			return err
		}
		defer f.Close()

		_, werr := f.Write([]byte(ratingJson))
		if werr != nil {
			return werr
		}

		profile, err := n.GetProfile()
		if err == nil {
			totalRatingVal := profile.Stats.AverageRating * float32(profile.Stats.RatingCount)
			totalRatingVal += float32(rating.RatingData.Overall)
			newAvg := totalRatingVal / float32(profile.Stats.RatingCount+1)
			profile.Stats.AverageRating = newAvg
			profile.Stats.RatingCount = profile.Stats.RatingCount + 1
			err = n.UpdateProfile(&profile)
			if err != nil {
				return err
			}
		}
		if err := n.updateRatingIndex(rating, ratingPath); err != nil {
			return err
		}

		if err := n.updateRatingInListingIndex(rating); err != nil {
			return err
		}
	}
	return nil
}

func (n *OpenBazaarNode) updateRatingIndex(rating *pb.Rating, ratingPath string) error {
	indexPath := path.Join(n.RepoPath, "root", "ratings", "index.json")

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
	for _, d := range index {
		if rating.RatingData.VendorSig.Metadata.ListingSlug == d.Slug {
			d.Ratings = append(d.Ratings, ratingHash)
			total := d.Average * float32(d.Count)
			total += float32(rating.RatingData.Overall)
			d.Count += 1
			d.Average = total / float32(d.Count)
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
