package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/btcsuite/btcd/btcec"
	"github.com/golang/protobuf/proto"
	crypto "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
)

func ValidateRating(rating *pb.Rating) (bool, error) {
	if rating.RatingData == nil || rating.RatingData.VendorID == nil || rating.RatingData.VendorID.Pubkeys == nil || rating.RatingData.VendorSig == nil || rating.RatingData.VendorSig.Metadata == nil {
		return false, errors.New("missing rating data")
	}

	// Validate rating signature
	ser, err := proto.Marshal(rating.RatingData)
	if err != nil {
		return false, err
	}
	ratingKey, err := btcec.ParsePubKey(rating.RatingData.RatingKey, btcec.S256())
	if err != nil {
		return false, err
	}
	sig, err := btcec.ParseSignature(rating.Signature, btcec.S256())
	if err != nil {
		return false, err
	}
	hashed := sha256.Sum256(ser)
	valid := sig.Verify(hashed[:], ratingKey)
	if !valid {
		return false, errors.New("invalid rating signature")
	}

	// Validate the vendor's signature on the metadata object
	vendorKey, err := crypto.UnmarshalPublicKey(rating.RatingData.VendorID.Pubkeys.Identity)
	if err != nil {
		return false, err
	}
	ser, err = proto.Marshal(rating.RatingData.VendorSig.Metadata)
	if err != nil {
		return false, err
	}
	valid, err = vendorKey.Verify(ser, rating.RatingData.VendorSig.Signature)
	if !valid || err != nil {
		return false, errors.New("invaid vendor signature")
	}

	// Validate vendor peerID matches pubkey
	id, err := peer.IDB58Decode(rating.RatingData.VendorID.PeerID)
	if err != nil {
		return false, err
	}
	if !id.MatchesPublicKey(vendorKey) {
		return false, errors.New("vendor ID does not match public key")
	}

	// If not moderated
	if rating.RatingData.ModeratorID == nil {
		// Validate that the rating key the vendor signed matches the rating key in the review
		if !bytes.Equal(rating.RatingData.RatingKey, rating.RatingData.VendorSig.Metadata.RatingKey) {
			return false, errors.New("rating key does match key signed by vendor")
		}

	} else { // If moderated
		if rating.RatingData.ModeratorID.Pubkeys == nil {
			return false, errors.New("moderator key is nil")
		}
		// Validate the moderator's signature on the rating key
		moderatorKey, err := crypto.UnmarshalPublicKey(rating.RatingData.ModeratorID.Pubkeys.Identity)
		if err != nil {
			return false, err
		}
		valid, err = moderatorKey.Verify(rating.RatingData.RatingKey, rating.RatingData.ModeratorSig)
		if !valid || err != nil {
			return false, errors.New("invalid moderator signature")
		}

		// Validate the moderator key the vendor signed matches the key in the rating
		if !bytes.Equal(rating.RatingData.ModeratorID.Pubkeys.Identity, rating.RatingData.VendorSig.Metadata.ModeratorKey) {
			return false, errors.New("moderator key does not match key signed by vendor")
		}

		// Validate moderator peerID matches pubkey
		id, err := peer.IDB58Decode(rating.RatingData.ModeratorID.PeerID)
		if err != nil {
			return false, err
		}
		if !id.MatchesPublicKey(moderatorKey) {
			return false, errors.New("moderator ID does not match public key")
		}

	}

	// Validate buyer signature if not anonymous
	if rating.RatingData.BuyerID != nil {
		if rating.RatingData.BuyerID.Pubkeys == nil {
			return false, errors.New("buyer public key is nil")
		}
		buyerKey, err := crypto.UnmarshalPublicKey(rating.RatingData.BuyerID.Pubkeys.Identity)
		if err != nil {
			return false, err
		}
		valid, err = buyerKey.Verify(rating.RatingData.RatingKey, rating.RatingData.BuyerSig)
		if !valid || err != nil {
			return false, errors.New("invalid buyer signature")
		}

		// Validate buyer peerID matches pubkey
		id, err := peer.IDB58Decode(rating.RatingData.BuyerID.PeerID)
		if err != nil {
			return false, err
		}
		if !id.MatchesPublicKey(buyerKey) {
			return false, errors.New("buyer ID does not match public key")
		}
	}

	return true, nil
}
