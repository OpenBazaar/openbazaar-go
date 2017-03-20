package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"time"

	"fmt"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/imdario/mergo"
	ipnspath "github.com/ipfs/go-ipfs/path"
	mh "gx/ipfs/QmbZ6Cee2uHjG7hf19qLHppgKDRtaG4CVtMzdmK9VCVqLu/go-multihash"
)

func (n *OpenBazaarNode) GetProfile() (pb.Profile, error) {
	var profile pb.Profile
	f, err := os.Open(path.Join(n.RepoPath, "root", "profile"))
	if err != nil {
		return profile, err
	}
	defer f.Close()
	err = jsonpb.Unmarshal(f, &profile)
	if err != nil {
		return profile, err
	}
	return profile, nil
}

func (n *OpenBazaarNode) FetchProfile(peerId string) (pb.Profile, error) {
	profile, err := ipfs.ResolveThenCat(n.Context, ipnspath.FromString(path.Join(peerId, "profile")))
	if err != nil || len(profile) == 0 {
		return pb.Profile{}, err
	}
	var pro pb.Profile
	err = jsonpb.UnmarshalString(string(profile), &pro)
	if err != nil {
		return pb.Profile{}, err
	}
	if err := ValidateProfile(&pro); err != nil {
		return pb.Profile{}, err
	}
	return pro, nil
}

func (n *OpenBazaarNode) UpdateProfile(profile *pb.Profile) error {
	mPubkey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
		return err
	}
	if err := ValidateProfile(profile); err != nil {
		return err
	}

	profile.BitcoinPubkey = hex.EncodeToString(mPubkey.SerializeCompressed())
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		Indent:       "    ",
		OrigName:     false,
	}
	out, err := m.MarshalToString(profile)
	if err != nil {
		return err
	}
	profilePath := path.Join(n.RepoPath, "root", "profile")
	f, err := os.Create(profilePath)
	defer f.Close()
	if err != nil {
		return err
	}
	if _, err := f.WriteString(out); err != nil {
		return err
	}
	return nil
}

func (n *OpenBazaarNode) PatchProfile(patch map[string]interface{}) error {
	profilePath := path.Join(n.RepoPath, "root", "profile")

	// Read stored profile data
	profile := make(map[string]interface{})
	profileBytes, err := ioutil.ReadFile(profilePath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		return err
	}

	formatModeratorAmount := func(modInfo interface{}) {
		fee, ok := modInfo.(map[string]interface{})["fee"]
		if ok {
			fixedFee, ok := fee.(map[string]interface{})["fixedFee"]
			if ok {
				amt := fixedFee.(map[string]interface{})["amount"].(float64)
				fixedFee.(map[string]interface{})["amount"] = uint64(amt)
			}
		}
	}
	modInfo, ok := patch["modInfo"]
	if ok {
		formatModeratorAmount(modInfo)
	}
	modInfo, ok = profile["modInfo"]
	if ok {
		formatModeratorAmount(modInfo)
	}

	patchMod, pok := patch["moderator"]
	storedMod, sok := profile["moderator"]
	if pok && sok {
		patchBool, ok := patchMod.(bool)
		if !ok {
			return errors.New("Invalid moderator type")
		}
		storedBool, ok := storedMod.(bool)
		if !ok {
			return errors.New("Invalid moderator type")
		}
		if patchBool && patchBool != storedBool {
			if err := n.SetSelfAsModerator(nil); err != nil {
				return err
			}
		} else if !patchBool && patchBool != storedBool {
			if err := n.RemoveSelfAsModerator(); err != nil {
				return err
			}
		}
	}

	// Assuming that `profile` map contains complete data, as it is read
	// from storage, and `patch` map is possibly incomplete, merge first
	// into second recursively, preserving new fields and adding missing
	// old ones
	if err := mergo.Map(&patch, &profile); err != nil {
		return err
	}

	// Execute UpdateProfile with new profile
	newProfile, err := json.Marshal(patch)
	p := new(pb.Profile)
	if err := jsonpb.UnmarshalString(string(newProfile), p); err != nil {
		return err
	}
	return n.UpdateProfile(p)
}

func (n *OpenBazaarNode) appendCountsToProfile(profile *pb.Profile) (*pb.Profile, error) {
	profile.ListingCount = uint32(n.GetListingCount())
	profile.FollowerCount = uint32(n.Datastore.Followers().Count())
	profile.FollowingCount = uint32(n.Datastore.Following().Count())

	ts := new(timestamp.Timestamp)
	ts.Seconds = time.Now().Unix()
	ts.Nanos = 0
	profile.LastModified = ts
	return profile, nil
}

func (n *OpenBazaarNode) updateProfileCounts() error {
	profilePath := path.Join(n.RepoPath, "root", "profile")
	profile := new(pb.Profile)
	_, ferr := os.Stat(profilePath)
	if !os.IsNotExist(ferr) {
		// Read existing file
		file, err := ioutil.ReadFile(profilePath)
		if err != nil {
			return err
		}
		err = jsonpb.UnmarshalString(string(file), profile)
		if err != nil {
			return err
		}
	} else {
		return nil
	}
	profile, err := n.appendCountsToProfile(profile)
	if err != nil {
		return err
	}
	return n.UpdateProfile(profile)
}

func ValidateProfile(profile *pb.Profile) error {
	if len(profile.Handle) > WordMaxCharacters {
		return fmt.Errorf("Handle character length is greater than the max of %d", WordMaxCharacters)
	}
	if len(profile.Name) > WordMaxCharacters {
		return fmt.Errorf("Name character length is greater than the max of %d", WordMaxCharacters)
	}
	if len(profile.Location) > WordMaxCharacters {
		return fmt.Errorf("Location character length is greater than the max of %d", WordMaxCharacters)
	}
	if len(profile.About) > AboutMaxCharacteres {
		return fmt.Errorf("About character length is greater than the max of %d", AboutMaxCharacteres)
	}
	if len(profile.ShortDescription) > ShortDescriptionLength {
		return fmt.Errorf("Short description character length is greater than the max of %d", ShortDescriptionLength)
	}
	if len(profile.Website) > URLMaxCharacters {
		return fmt.Errorf("Website character length is greater than the max of %d", URLMaxCharacters)
	}
	if len(profile.Email) > SentenceMaxCharacters {
		return fmt.Errorf("Email character length is greater than the max of %d", SentenceMaxCharacters)
	}
	if len(profile.PhoneNumber) > WordMaxCharacters {
		return fmt.Errorf("Phone number character length is greater than the max of %d", WordMaxCharacters)
	}
	if len(profile.Social) > MaxListItems {
		return fmt.Errorf("Number of social accounts is greater than the max of %d", MaxListItems)
	}
	for _, s := range profile.Social {
		if len(s.Username) > WordMaxCharacters {
			return fmt.Errorf("Social username character length is greater than the max of %d", WordMaxCharacters)
		}
		if len(s.Type) > WordMaxCharacters {
			return fmt.Errorf("Social account type character length is greater than the max of %d", WordMaxCharacters)
		}
		if len(s.Proof) > URLMaxCharacters {
			return fmt.Errorf("Social proof character length is greater than the max of %d", WordMaxCharacters)
		}
	}
	if profile.ModInfo != nil {
		if len(profile.ModInfo.Description) > AboutMaxCharacteres {
			return fmt.Errorf("Moderator description character length is greater than the max of %d", AboutMaxCharacteres)
		}
		if len(profile.ModInfo.TermsAndConditions) > PolicyMaxCharacters {
			return fmt.Errorf("Moderator terms and conditions character length is greater than the max of %d", PolicyMaxCharacters)
		}
		if len(profile.ModInfo.Languages) > MaxListItems {
			return fmt.Errorf("Moderator number of languages greater than the max of %d", MaxListItems)
		}
		for _, l := range profile.ModInfo.Languages {
			if len(l) > WordMaxCharacters {
				return fmt.Errorf("Moderator language character length is greater than the max of %d", WordMaxCharacters)
			}
		}
		if profile.ModInfo.Fee != nil {
			if profile.ModInfo.Fee.FixedFee != nil {
				if len(profile.ModInfo.Fee.FixedFee.CurrencyCode) > WordMaxCharacters {
					return fmt.Errorf("Moderator fee currency code character length is greater than the max of %d", WordMaxCharacters)
				}
			}
		}
	}
	if profile.AvatarHashes != nil {
		_, err := mh.FromB58String(profile.AvatarHashes.Tiny)
		if err != nil {
			return errors.New("Tiny image hashes must be multihashes")
		}
		_, err = mh.FromB58String(profile.AvatarHashes.Small)
		if err != nil {
			return errors.New("Small image hashes must be multihashes")
		}
		_, err = mh.FromB58String(profile.AvatarHashes.Medium)
		if err != nil {
			return errors.New("Medium image hashes must be multihashes")
		}
		_, err = mh.FromB58String(profile.AvatarHashes.Large)
		if err != nil {
			return errors.New("Large image hashes must be multihashes")
		}
		_, err = mh.FromB58String(profile.AvatarHashes.Original)
		if err != nil {
			return errors.New("Original image hashes must be multihashes")
		}
	}
	if profile.HeaderHashes != nil {
		_, err := mh.FromB58String(profile.HeaderHashes.Tiny)
		if err != nil {
			return errors.New("Tiny image hashes must be multihashes")
		}
		_, err = mh.FromB58String(profile.HeaderHashes.Small)
		if err != nil {
			return errors.New("Small image hashes must be multihashes")
		}
		_, err = mh.FromB58String(profile.HeaderHashes.Medium)
		if err != nil {
			return errors.New("Medium image hashes must be multihashes")
		}
		_, err = mh.FromB58String(profile.HeaderHashes.Large)
		if err != nil {
			return errors.New("Large image hashes must be multihashes")
		}
		_, err = mh.FromB58String(profile.HeaderHashes.Original)
		if err != nil {
			return errors.New("Original image hashes must be multihashes")
		}
	}
	if len(profile.BitcoinPubkey) > 66 {
		return fmt.Errorf("Bitcoin public key character length is greater than the max of %d", 66)
	}
	if profile.AvgRating > 5 {
		return fmt.Errorf("Average rating cannot be greater than %d", 5)
	}
	return nil
}
