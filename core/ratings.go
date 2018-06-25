package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/btcsuite/btcd/btcec"
	"github.com/golang/protobuf/proto"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	crypto "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"io/ioutil"
	"os"
	"path"
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
	if rating.RatingData.ModeratorSig == nil {
		// Validate that the rating key the vendor signed matches the rating key in the review
		if !bytes.Equal(rating.RatingData.RatingKey, rating.RatingData.VendorSig.Metadata.RatingKey) {
			return false, errors.New("rating key does match key signed by vendor")
		}

	} else { // If moderated
		// Validate the moderator's signature on the rating key
		moderatorKey, err := btcec.ParsePubKey(rating.RatingData.VendorSig.Metadata.ModeratorKey, btcec.S256())
		if err != nil {
			return false, err
		}
		sig, err := btcec.ParseSignature(rating.RatingData.ModeratorSig, btcec.S256())
		if err != nil {
			return false, err
		}
		hashed := sha256.Sum256(rating.RatingData.RatingKey)
		valid := sig.Verify(hashed[:], moderatorKey)
		if !valid {
			return false, errors.New("invalid moderator signature")
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

func (n *OpenBazaarNode) GetRatingCounts() (uint32, float32, error) {
	indexPath := path.Join(n.RepoPath, "root", "ratings.json")

	var index []SavedRating

	_, ferr := os.Stat(indexPath)
	if !os.IsNotExist(ferr) {
		// Read existing file
		file, err := ioutil.ReadFile(indexPath)
		if err != nil {
			return 0, 0, err
		}
		err = json.Unmarshal(file, &index)
		if err != nil {
			return 0, 0, err
		}
	} else {
		return 0, 0, nil
	}
	var ratingCount uint32
	var totalRating float32
	for _, i := range index {
		ratingCount += uint32(i.Count)
		totalRating += (float32(i.Count) * i.Average)
	}
	averageRating := (totalRating / float32(ratingCount))
	return ratingCount, averageRating, nil
}
