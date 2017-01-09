package core

import (
	"crypto/sha256"
	"errors"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"golang.org/x/net/context"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	"io/ioutil"
	"path"
	"strings"
)

var ModeratorPointerID multihash.Multihash

func init() {
	modHash := sha256.Sum256([]byte("moderators"))
	encoded, err := multihash.Encode(modHash[:], multihash.SHA2_256)
	if err != nil {
		log.Fatal("Error creating moderator pointer ID (multihash encode)")
	}
	mh, err := multihash.Cast(encoded)
	if err != nil {
		log.Fatal("Error creating moderator pointer ID (multihash cast)")
	}
	ModeratorPointerID = mh
}

func (n *OpenBazaarNode) SetSelfAsModerator(moderator *pb.Moderator) error {
	if moderator.Fee == nil {
		return errors.New("Moderator must have a fee set")
	}
	if (int(moderator.Fee.FeeType) == 0 || int(moderator.Fee.FeeType) == 2) && moderator.Fee.FixedFee == nil {
		return errors.New("Fixed fee must be set when using a fixed fee type")
	}

	// Update profile
	profile, err := n.GetProfile()
	if err != nil {
		return err
	}
	profile.Moderator = true
	profile.ModInfo = moderator
	err = n.UpdateProfile(&profile)
	if err != nil {
		return err
	}

	// Publish pointer
	ctx := context.Background()

	b, err := multihash.Encode([]byte(n.IpfsNode.Identity.Pretty()), multihash.SHA1)
	if err != nil {
		return err
	}
	mhc, err := multihash.Cast(b)
	if err != nil {
		return err
	}
	addr, err := ma.NewMultiaddr("/ipfs/" + mhc.B58String())
	if err != nil {
		return err
	}
	pointer, err := ipfs.PublishPointer(n.IpfsNode, ctx, ModeratorPointerID, 64, addr)
	if err != nil {
		return err
	}
	pointer.Purpose = ipfs.MODERATOR
	err = n.Datastore.Pointers().DeleteAll(pointer.Purpose)
	if err != nil {
		return err
	}
	err = n.Datastore.Pointers().Put(pointer)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) RemoveSelfAsModerator() error {
	// Update profile
	profile, err := n.GetProfile()
	if err != nil {
		return err
	}
	profile.Moderator = false
	err = n.UpdateProfile(&profile)
	if err != nil {
		return err
	}

	// Delete pointer from database
	err = n.Datastore.Pointers().DeleteAll(ipfs.MODERATOR)
	if err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) GetModeratorFee(transactionTotal uint64) (uint64, error) {
	file, err := ioutil.ReadFile(path.Join(n.RepoPath, "root", "profile"))
	if err != nil {
		return 0, err
	}
	profile := new(pb.Profile)
	err = jsonpb.UnmarshalString(string(file), profile)
	if err != nil {
		return 0, err
	}

	switch profile.ModInfo.Fee.FeeType {
	case pb.Moderator_Fee_PERCENTAGE:
		return uint64(float64(transactionTotal) * (float64(profile.ModInfo.Fee.Percentage) / 100)), nil
	case pb.Moderator_Fee_FIXED:
		if strings.ToLower(profile.ModInfo.Fee.FixedFee.CurrencyCode) == "btc" {
			if profile.ModInfo.Fee.FixedFee.Amount >= transactionTotal {
				return 0, errors.New("Fixed moderator fee exceeds transaction amount")
			}
			return profile.ModInfo.Fee.FixedFee.Amount, nil
		} else {
			fee, err := n.getPriceInSatoshi(profile.ModInfo.Fee.FixedFee.CurrencyCode, profile.ModInfo.Fee.FixedFee.Amount)
			if err != nil {
				return 0, err
			} else if fee >= transactionTotal {
				return 0, errors.New("Fixed moderator fee exceeds transaction amount")
			}
			return fee, err
		}
	case pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE:
		var fixed uint64
		if strings.ToLower(profile.ModInfo.Fee.FixedFee.CurrencyCode) == "btc" {
			fixed = profile.ModInfo.Fee.FixedFee.Amount
		} else {
			fixed, err = n.getPriceInSatoshi(profile.ModInfo.Fee.FixedFee.CurrencyCode, profile.ModInfo.Fee.FixedFee.Amount)
			if err != nil {
				return 0, err
			}
		}
		percentage := uint64(float64(transactionTotal) * (float64(profile.ModInfo.Fee.Percentage) / 100))
		if fixed+percentage >= transactionTotal {
			return 0, errors.New("Fixed moderator fee exceeds transaction amount")
		}
		return fixed + percentage, nil
	default:
		return 0, errors.New("Unrecognized fee type")
	}
}
