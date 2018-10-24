package core

import (
	"crypto/sha256"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/OpenBazaar/jsonpb"
	"golang.org/x/net/context"

	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	multihash "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

// ModeratorPointerID  moderator ipfs multihash
var ModeratorPointerID multihash.Multihash

// ErrNoListings - no listing error
// FIXME : This is not used anywhere
var ErrNoListings = errors.New("no listings to set moderators on")

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

// IsModerator - Am I a moderator?
func (n *OpenBazaarNode) IsModerator() bool {
	profile, err := n.GetProfile()
	if err != nil {
		return false
	}
	return profile.Moderator
}

// SetSelfAsModerator - set self as a moderator
func (n *OpenBazaarNode) SetSelfAsModerator(moderator *pb.Moderator) error {
	if moderator != nil {
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

		var currencies []string
		settingsData, _ := n.Datastore.Settings().Get()
		if settingsData.PreferredCurrencies != nil {
			currencies = append(currencies, *settingsData.PreferredCurrencies...)
		} else {
			for ct := range n.Multiwallet {
				currencies = append(currencies, ct.CurrencyCode())
			}
		}
		for _, cc := range currencies {
			moderator.AcceptedCurrencies = append(moderator.AcceptedCurrencies, NormalizeCurrencyCode(cc))
		}

		profile.Moderator = true
		profile.ModeratorInfo = moderator
		err = n.UpdateProfile(&profile)
		if err != nil {
			return err
		}
	}

	// Publish pointer
	pointers, err := n.Datastore.Pointers().GetByPurpose(ipfs.MODERATOR)
	ctx := context.Background()
	if err != nil || len(pointers) == 0 {
		addr, err := ma.NewMultiaddr("/ipfs/" + n.IpfsNode.Identity.Pretty())
		if err != nil {
			return err
		}
		pointer, err := ipfs.NewPointer(ModeratorPointerID, 64, addr, []byte(n.IpfsNode.Identity.Pretty()))
		if err != nil {
			return err
		}
		go ipfs.PublishPointer(n.IpfsNode, ctx, pointer)
		pointer.Purpose = ipfs.MODERATOR
		err = n.Datastore.Pointers().Put(pointer)
		if err != nil {
			return err
		}
	} else {
		go ipfs.PublishPointer(n.IpfsNode, ctx, pointers[0])
	}
	return nil
}

// RemoveSelfAsModerator - relinquish moderatorship
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

// GetModeratorFee - fetch moderator fee
func (n *OpenBazaarNode) GetModeratorFee(transactionTotal uint64, paymentCoin, currencyCode string) (uint64, error) {
	file, err := ioutil.ReadFile(path.Join(n.RepoPath, "root", "profile.json"))
	if err != nil {
		return 0, err
	}
	profile := new(pb.Profile)
	err = jsonpb.UnmarshalString(string(file), profile)
	if err != nil {
		return 0, err
	}

	switch profile.ModeratorInfo.Fee.FeeType {
	case pb.Moderator_Fee_PERCENTAGE:
		return uint64(float64(transactionTotal) * (float64(profile.ModeratorInfo.Fee.Percentage) / 100)), nil
	case pb.Moderator_Fee_FIXED:

		if NormalizeCurrencyCode(profile.ModeratorInfo.Fee.FixedFee.CurrencyCode) == NormalizeCurrencyCode(currencyCode) {
			if profile.ModeratorInfo.Fee.FixedFee.Amount >= transactionTotal {
				return 0, errors.New("Fixed moderator fee exceeds transaction amount")
			}
			return profile.ModeratorInfo.Fee.FixedFee.Amount, nil
		}
		fee, err := n.getPriceInSatoshi(paymentCoin, profile.ModeratorInfo.Fee.FixedFee.CurrencyCode, profile.ModeratorInfo.Fee.FixedFee.Amount)
		if err != nil {
			return 0, err
		} else if fee >= transactionTotal {
			return 0, errors.New("Fixed moderator fee exceeds transaction amount")
		}
		return fee, err

	case pb.Moderator_Fee_FIXED_PLUS_PERCENTAGE:
		var fixed uint64
		if NormalizeCurrencyCode(profile.ModeratorInfo.Fee.FixedFee.CurrencyCode) == NormalizeCurrencyCode(currencyCode) {
			fixed = profile.ModeratorInfo.Fee.FixedFee.Amount
		} else {
			fixed, err = n.getPriceInSatoshi(paymentCoin, profile.ModeratorInfo.Fee.FixedFee.CurrencyCode, profile.ModeratorInfo.Fee.FixedFee.Amount)
			if err != nil {
				return 0, err
			}
		}
		percentage := uint64(float64(transactionTotal) * (float64(profile.ModeratorInfo.Fee.Percentage) / 100))
		if fixed+percentage >= transactionTotal {
			return 0, errors.New("Fixed moderator fee exceeds transaction amount")
		}
		return fixed + percentage, nil
	default:
		return 0, errors.New("Unrecognized fee type")
	}
}

// SetModeratorsOnListings - set moderators for a listing
func (n *OpenBazaarNode) SetModeratorsOnListings(moderators []string) error {
	absPath, err := filepath.Abs(path.Join(n.RepoPath, "root", "listings"))
	if err != nil {
		return err
	}
	hashes := make(map[string]string)
	walkpath := func(p string, f os.FileInfo, err error) error {
		if !f.IsDir() {
			file, err := ioutil.ReadFile(p)
			if err != nil {
				return err
			}
			sl := new(pb.SignedListing)
			err = jsonpb.UnmarshalString(string(file), sl)
			if err != nil {
				return err
			}
			coupons, err := n.Datastore.Coupons().Get(sl.Listing.Slug)
			if err != nil {
				return err
			}
			couponMap := make(map[string]string)
			for _, c := range coupons {
				couponMap[c.Hash] = c.Code
			}
			for _, coupon := range sl.Listing.Coupons {
				code, ok := couponMap[coupon.GetHash()]
				if ok {
					coupon.Code = &pb.Listing_Coupon_DiscountCode{DiscountCode: code}
				}
			}

			sl.Listing.Moderators = moderators
			sl, err = n.SignListing(sl.Listing)
			if err != nil {
				return err
			}
			m := jsonpb.Marshaler{
				EnumsAsInts:  false,
				EmitDefaults: false,
				Indent:       "    ",
				OrigName:     false,
			}
			fi, err := os.Create(p)
			if err != nil {
				return err
			}
			out, err := m.MarshalToString(sl)
			if err != nil {
				return err
			}
			if _, err := fi.WriteString(out); err != nil {
				return err
			}
			hash, err := ipfs.GetHashOfFile(n.IpfsNode, p)
			if err != nil {
				return err
			}
			hashes[sl.Listing.Slug] = hash

			return nil
		}
		return nil
	}

	err = filepath.Walk(absPath, walkpath)
	if err != nil {
		return err
	}

	// Update moderators and hashes on index
	updater := func(listing *ListingData) error {
		listing.ModeratorIDs = moderators
		if hash, ok := hashes[listing.Slug]; ok {
			listing.Hash = hash
		}
		return nil
	}
	return n.UpdateEachListingOnIndex(updater)
}

// NotifyModerators - notify moderators(peers)
func (n *OpenBazaarNode) NotifyModerators(moderators []string) error {
	settings, err := n.Datastore.Settings().Get()
	if err != nil {
		return err
	}
	currentMods := make(map[string]bool)
	if settings.StoreModerators != nil {
		for _, mod := range *settings.StoreModerators {
			currentMods[mod] = true
		}
	}
	var addedMods []string
	for _, mod := range moderators {
		if !currentMods[mod] {
			addedMods = append(addedMods, mod)
		} else {
			delete(currentMods, mod)
		}
	}

	removedMods := currentMods

	for _, mod := range addedMods {
		go n.SendModeratorAdd(mod)
	}
	for mod := range removedMods {
		go n.SendModeratorRemove(mod)
	}
	return nil
}
