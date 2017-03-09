package core

import (
	"crypto/sha256"
	"errors"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"golang.org/x/net/context"
	ma "gx/ipfs/QmUAQaWbKxGCUTuoQVvvicbQNZ9APF5pDGWyAZSe93AtKH/go-multiaddr"
	multihash "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ModeratorPointerID multihash.Multihash

var NoListingsError error = errors.New("No listings to set moderators on")

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

func (n *OpenBazaarNode) IsModerator() bool {
	profile, err := n.GetProfile()
	if err != nil {
		return false
	}
	return profile.Moderator
}

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
		profile.Moderator = true
		profile.ModInfo = moderator
		err = n.UpdateProfile(&profile)
		if err != nil {
			return err
		}
	}

	// Publish pointer
	pointers, err := n.Datastore.Pointers().Get(ipfs.MODERATOR)
	ctx := context.Background()
	if err != nil || len(pointers) == 0 {
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
		err = n.Datastore.Pointers().Put(pointer)
		if err != nil {
			return err
		}
	} else {
		err := ipfs.RePublishPointer(n.IpfsNode, ctx, pointers[0])
		if err != nil {
			return err
		}
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
			rc := new(pb.RicardianContract)
			err = jsonpb.UnmarshalString(string(file), rc)
			if err != nil {
				return err
			}
			rc.VendorListings[0].Moderators = moderators
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
			out, err := m.MarshalToString(rc)
			if err != nil {
				return err
			}
			if _, err := fi.WriteString(out); err != nil {
				return err
			}
			hash, err := ipfs.GetHash(n.Context, p)
			if err != nil {
				return err
			}
			hashes[rc.VendorListings[0].Slug] = hash

			return n.UpdateListingIndex(rc)
		}
		return nil
	}

	err = filepath.Walk(absPath, walkpath)
	if err != nil {
		return err
	}
	return n.UpdateIndexHashes(hashes)
}

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
