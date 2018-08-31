package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/golang/protobuf/ptypes"
	"github.com/imdario/mergo"
	ipnspath "github.com/ipfs/go-ipfs/path"

	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

// KeyCachePrefix - cache prefix for public key
const KeyCachePrefix = "/pubkey/"

// ErrorProfileNotFound - profile not found error
var ErrorProfileNotFound = errors.New("profile not found")

// GetProfile - fetch user profile
func (n *OpenBazaarNode) GetProfile() (pb.Profile, error) {
	var profile pb.Profile
	f, err := os.Open(path.Join(n.RepoPath, "root", "profile.json"))
	if err != nil {
		return profile, ErrorProfileNotFound
	}
	defer f.Close()
	err = jsonpb.Unmarshal(f, &profile)
	if err != nil {
		return profile, err
	}
	return profile, nil
}

// FetchProfile - fetch peer's profile
func (n *OpenBazaarNode) FetchProfile(peerID string, useCache bool) (pb.Profile, error) {
	var pro pb.Profile
	b, err := n.IPNSResolveThenCat(ipnspath.FromString(path.Join(peerID, "profile.json")), time.Minute, useCache)
	if err != nil || len(b) == 0 {
		return pro, err
	}
	err = jsonpb.UnmarshalString(string(b), &pro)
	if err != nil {
		return pro, err
	}
	return pro, nil
}

// UpdateProfile - update user profile
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

	if profile.Currencies == nil {
		profile.Currencies = []string{NormalizeCurrencyCode(n.Wallet.CurrencyCode())}
	}

	if profile.ModeratorInfo != nil {
		profile.ModeratorInfo.AcceptedCurrencies = []string{NormalizeCurrencyCode(n.Wallet.CurrencyCode())}
	}
	profile.PeerID = n.IpfsNode.Identity.Pretty()
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	profile.LastModified = ts
	out, err := m.MarshalToString(profile)
	if err != nil {
		return err
	}

	profilePath := path.Join(n.RepoPath, "root", "profile.json")
	f, err := os.Create(profilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(out); err != nil {
		return err
	}
	return nil
}

// PatchProfile - patch user profile
func (n *OpenBazaarNode) PatchProfile(patch map[string]interface{}) error {
	profilePath := path.Join(n.RepoPath, "root", "profile.json")

	// Read stored profile data
	file, err := os.Open(profilePath)
	if err != nil {
		return err
	}
	d := json.NewDecoder(file)
	d.UseNumber()

	var i interface{}
	err = d.Decode(&i)
	if err != nil {
		return err
	}
	profile := i.(map[string]interface{})

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
	if err != nil {
		return err
	}
	p := new(pb.Profile)
	if err := jsonpb.Unmarshal(bytes.NewReader(newProfile), p); err != nil {
		return err
	}
	return n.UpdateProfile(p)
}

func (n *OpenBazaarNode) appendCountsToProfile(profile *pb.Profile) (*pb.Profile, bool) {
	if profile.Stats == nil {
		profile.Stats = new(pb.Profile_Stats)
	}
	var changed bool
	listingCount := uint32(n.GetListingCount())
	if listingCount != profile.Stats.ListingCount {
		profile.Stats.ListingCount = listingCount
		changed = true
	}
	postCount := uint32(n.GetPostCount())
	if postCount != profile.Stats.PostCount {
		profile.Stats.PostCount = postCount
		changed = true
	}
	followerCount := uint32(n.Datastore.Followers().Count())
	if followerCount != profile.Stats.FollowerCount {
		profile.Stats.FollowerCount = followerCount
		changed = true
	}
	followingCount := uint32(n.Datastore.Following().Count())
	if followingCount != profile.Stats.FollowingCount {
		profile.Stats.FollowingCount = followingCount
		changed = true
	}
	ratingCount, averageRating, err := n.GetRatingCounts()
	if err == nil && (ratingCount != profile.Stats.RatingCount || averageRating != profile.Stats.AverageRating) {
		profile.Stats.RatingCount = ratingCount
		profile.Stats.AverageRating = averageRating
		changed = true
	}
	return profile, changed
}

func (n *OpenBazaarNode) updateProfileCounts() error {
	profilePath := path.Join(n.RepoPath, "root", "profile.json")
	profile := new(pb.Profile)
	_, ferr := os.Stat(profilePath)
	if !os.IsNotExist(ferr) {
		// Read existing file
		file, err := os.Open(profilePath)
		if err != nil {
			return err
		}
		defer file.Close()
		err = jsonpb.Unmarshal(file, profile)
		if err != nil {
			return err
		}
	} else {
		return nil
	}
	newPro, changed := n.appendCountsToProfile(profile)
	if changed {
		return n.UpdateProfile(newPro)
	}
	return nil
}

func (n *OpenBazaarNode) updateProfileRatings(newRating *pb.Rating) error {
	profilePath := path.Join(n.RepoPath, "root", "profile.json")
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
	if profile.Stats != nil && newRating.RatingData != nil {
		total := profile.Stats.AverageRating * float32(profile.Stats.RatingCount)
		total += float32(newRating.RatingData.Overall)
		profile.Stats.RatingCount++ // += 1
		profile.Stats.AverageRating = total / float32(profile.Stats.RatingCount)
	}
	newPro, _ := n.appendCountsToProfile(profile)

	return n.UpdateProfile(newPro)
}

// ValidateProfile - validate fetched profile
func ValidateProfile(profile *pb.Profile) error {
	if strings.Contains(profile.Handle, "@") {
		return errors.New("Handle should not contain @")
	}
	if len(profile.Handle) > WordMaxCharacters {
		return fmt.Errorf("Handle character length is greater than the max of %d", WordMaxCharacters)
	}
	if len(profile.Name) == 0 {
		return errors.New("Profile name not set")
	}
	if len(profile.Name) > WordMaxCharacters {
		return fmt.Errorf("Name character length is greater than the max of %d", WordMaxCharacters)
	}
	if len(profile.Location) > WordMaxCharacters {
		return fmt.Errorf("Location character length is greater than the max of %d", WordMaxCharacters)
	}
	if len(profile.About) > AboutMaxCharacters {
		return fmt.Errorf("About character length is greater than the max of %d", AboutMaxCharacters)
	}
	if len(profile.ShortDescription) > ShortDescriptionLength {
		return fmt.Errorf("Short description character length is greater than the max of %d", ShortDescriptionLength)
	}
	if profile.ContactInfo != nil {
		if len(profile.ContactInfo.Website) > URLMaxCharacters {
			return fmt.Errorf("Website character length is greater than the max of %d", URLMaxCharacters)
		}
		if len(profile.ContactInfo.Email) > SentenceMaxCharacters {
			return fmt.Errorf("Email character length is greater than the max of %d", SentenceMaxCharacters)
		}
		if len(profile.ContactInfo.PhoneNumber) > WordMaxCharacters {
			return fmt.Errorf("Phone number character length is greater than the max of %d", WordMaxCharacters)
		}
		if len(profile.ContactInfo.Social) > MaxListItems {
			return fmt.Errorf("Number of social accounts is greater than the max of %d", MaxListItems)
		}
		for _, s := range profile.ContactInfo.Social {
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
	}
	if profile.ModeratorInfo != nil {
		if len(profile.ModeratorInfo.Description) > AboutMaxCharacters {
			return fmt.Errorf("Moderator description character length is greater than the max of %d", AboutMaxCharacters)
		}
		if len(profile.ModeratorInfo.TermsAndConditions) > PolicyMaxCharacters {
			return fmt.Errorf("Moderator terms and conditions character length is greater than the max of %d", PolicyMaxCharacters)
		}
		if len(profile.ModeratorInfo.Languages) > MaxListItems {
			return fmt.Errorf("Moderator number of languages greater than the max of %d", MaxListItems)
		}
		for _, l := range profile.ModeratorInfo.Languages {
			if len(l) > WordMaxCharacters {
				return fmt.Errorf("Moderator language character length is greater than the max of %d", WordMaxCharacters)
			}
		}
		if profile.ModeratorInfo.Fee != nil {
			if profile.ModeratorInfo.Fee.FixedFee != nil {
				if len(profile.ModeratorInfo.Fee.FixedFee.CurrencyCode) > WordMaxCharacters {
					return fmt.Errorf("Moderator fee currency code character length is greater than the max of %d", WordMaxCharacters)
				}
			}
		}
	}
	if profile.AvatarHashes != nil && (profile.AvatarHashes.Large != "" || profile.AvatarHashes.Medium != "" ||
		profile.AvatarHashes.Small != "" || profile.AvatarHashes.Tiny != "" || profile.AvatarHashes.Original != "") {
		_, err := cid.Decode(profile.AvatarHashes.Tiny)
		if err != nil {
			return errors.New("Tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Small)
		if err != nil {
			return errors.New("Small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Medium)
		if err != nil {
			return errors.New("Medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Large)
		if err != nil {
			return errors.New("Large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Original)
		if err != nil {
			return errors.New("Original image hashes must be properly formatted CID")
		}
	}
	if profile.HeaderHashes != nil && (profile.HeaderHashes.Large != "" || profile.HeaderHashes.Medium != "" ||
		profile.HeaderHashes.Small != "" || profile.HeaderHashes.Tiny != "" || profile.HeaderHashes.Original != "") {
		_, err := cid.Decode(profile.HeaderHashes.Tiny)
		if err != nil {
			return errors.New("Tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Small)
		if err != nil {
			return errors.New("Small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Medium)
		if err != nil {
			return errors.New("Medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Large)
		if err != nil {
			return errors.New("Large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Original)
		if err != nil {
			return errors.New("Original image hashes must be properly formatted CID")
		}
	}
	if len(profile.BitcoinPubkey) > 66 {
		return fmt.Errorf("Bitcoin public key character length is greater than the max of %d", 66)
	}
	if profile.Stats != nil {
		if profile.Stats.AverageRating > 5 {
			return fmt.Errorf("Average rating cannot be greater than %d", 5)
		}
	}
	return nil
}
