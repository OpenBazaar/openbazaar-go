package core

import (
	"bytes"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/btcsuite/btcd/btcec"
	"github.com/golang/protobuf/proto"
	crypto "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
)

func ValidateRating(rating *pb.Rating) bool {
	if rating.RatingData == nil || rating.RatingData.VendorID == nil || rating.RatingData.VendorID.Pubkeys == nil || rating.RatingData.VendorSig == nil || rating.RatingData.VendorSig.Metadata == nil {
		return false
	}

	// Validate rating signature
	ser, err := proto.Marshal(rating.RatingData)
	if err != nil {
		return false
	}
	ratingKey, err := btcec.ParsePubKey(rating.RatingData.RatingKey, btcec.S256())
	if err != nil {
		return false
	}
	sig, err := btcec.ParseSignature(rating.Signature, btcec.S256())
	if err != nil {
		return false
	}
	valid := sig.Verify(ser, ratingKey)
	if !valid {
		return false
	}

	// Validate the vendor's signature on the metadata object
	vendorKey, err := crypto.UnmarshalPublicKey(rating.RatingData.VendorID.Pubkeys.Identity)
	if err != nil {
		return false
	}
	ser, err = proto.Marshal(rating.RatingData.VendorSig.Metadata)
	if err != nil {
		return false
	}
	valid, err = vendorKey.Verify(ser, rating.RatingData.VendorSig.Signature)
	if !valid || err != nil {
		return false
	}

	// Validate vendor peerID matches pubkey
	id, err := peer.IDB58Decode(rating.RatingData.VendorID.PeerID)
	if err != nil {
		return false
	}
	if !id.MatchesPublicKey(vendorKey) {
		return false
	}

	// If not moderated
	if rating.RatingData.ModeratorID == nil {
		// Validate that the rating key the vendor signed matches the rating key in the review
		if !bytes.Equal(rating.RatingData.RatingKey, rating.RatingData.VendorSig.Metadata.RatingKey) {
			return false
		}

	} else { // If moderated
		if rating.RatingData.ModeratorID.Pubkeys == nil {
			return false
		}
		// Validate the moderator's signature on the rating key
		moderatorKey, err := crypto.UnmarshalPublicKey(rating.RatingData.ModeratorID.Pubkeys.Identity)
		if err != nil {
			return false
		}
		valid, err = moderatorKey.Verify(rating.RatingData.RatingKey, rating.RatingData.ModeratorSig)
		if !valid || err != nil {
			return false
		}

		// Validate the moderator key the vendor signed matches the key in the rating
		if !bytes.Equal(rating.RatingData.ModeratorID.Pubkeys.Identity, rating.RatingData.VendorSig.Metadata.ModeratorKey) {
			return false
		}

		// Validate moderator peerID matches pubkey
		id, err := peer.IDB58Decode(rating.RatingData.ModeratorID.PeerID)
		if err != nil {
			return false
		}
		if !id.MatchesPublicKey(moderatorKey) {
			return false
		}

	}

	// Validate buyer signature if not anonymous
	if rating.RatingData.BuyerID != nil {
		if rating.RatingData.BuyerID.Pubkeys == nil {
			return false
		}
		buyerKey, err := crypto.UnmarshalPublicKey(rating.RatingData.BuyerID.Pubkeys.Identity)
		if err != nil {
			return false
		}
		valid, err = buyerKey.Verify(rating.RatingData.RatingKey, rating.RatingData.BuyerSig)
		if !valid || err != nil {
			return false
		}

		// Validate buyer peerID matches pubkey
		id, err := peer.IDB58Decode(rating.RatingData.BuyerID.PeerID)
		if err != nil {
			return false
		}
		if !id.MatchesPublicKey(buyerKey) {
			return false
		}
	}

	return true
}
