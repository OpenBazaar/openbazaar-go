package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/repo"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"

	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	ipnspath "gx/ipfs/QmQAgv6Gaoe2tQpcabqwKXKChp2MZ7i3UXv9DqTTaxCaTR/go-path"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/ptypes"
	"github.com/imdario/mergo"
)

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
	b, err := ipfs.ResolveThenCat(n.IpfsNode, ipnspath.FromString(path.Join(peerID, "profile.json")), time.Minute, n.IPNSQuorumSize, true)
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
	mPubkey, err := n.MasterPrivateKey.ECPubKey()
	if err != nil {
		return fmt.Errorf("getting public key: %s", err.Error())
	}

	profile.Version = repo.ListingVersion
	profile.BitcoinPubkey = hex.EncodeToString(mPubkey.SerializeCompressed())
	var acceptedCurrencies = profile.GetCurrencies()
	settingsData, err := n.Datastore.Settings().Get()
	if err != nil {
		log.Debug("settings not set, using default preferred currencies")
	}
	if len(acceptedCurrencies) == 0 {
		if settingsData.PreferredCurrencies != nil {
			for _, ct := range *settingsData.PreferredCurrencies {
				def, err := n.LookupCurrency(ct)
				if err != nil {
					return fmt.Errorf("lookup currency (%s): %s", ct, err)
				}
				acceptedCurrencies = append(acceptedCurrencies, def.CurrencyCode().String())
			}
		} else {
			for ct := range n.Multiwallet {
				def, err := n.LookupCurrency(ct.CurrencyCode())
				if err != nil {
					return fmt.Errorf("lookup currency (%s): %s", ct.CurrencyCode(), err)
				}
				acceptedCurrencies = append(acceptedCurrencies, def.CurrencyCode().String())
			}
		}
	}

	profile.Currencies = acceptedCurrencies
	if profile.ModeratorInfo != nil {
		profile.ModeratorInfo.AcceptedCurrencies = acceptedCurrencies

		// Update moderator info fixed fee details
		if profile.ModeratorInfo.Fee != nil {
			if profile.ModeratorInfo.Fee.FixedFee != nil {
				if profile.ModeratorInfo.Fee.FixedFee.AmountCurrency != nil {
					fixedFee, err := repo.NewCurrencyValueFromProtobuf(profile.ModeratorInfo.Fee.FixedFee.BigAmount, profile.ModeratorInfo.Fee.FixedFee.AmountCurrency)
					if err != nil {
						return fmt.Errorf("unable to parse fixed fee currency: %s", err.Error())
					}
					normalizedFee, err := fixedFee.Normalize()
					if err != nil {
						feeDivisibility := uint(profile.ModeratorInfo.Fee.FixedFee.AmountCurrency.Divisibility)
						return fmt.Errorf("converting divisibility for fixed fee (%s) from (%d) to (%d): %s", fixedFee.Currency.String(), fixedFee.Currency.Divisibility, feeDivisibility, err.Error())
					}
					profile.ModeratorInfo.Fee.FixedFee = &pb.Moderator_Price{
						AmountCurrency: &pb.CurrencyDefinition{
							Code:         normalizedFee.Currency.CurrencyCode().String(),
							Divisibility: uint32(normalizedFee.Currency.Divisibility),
						},
						BigAmount: normalizedFee.Amount.String(),
					}
				}
			}
		}
	}

	profile.PeerID = n.IpfsNode.Identity.Pretty()
	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}
	profile.LastModified = ts
	if err := ValidateProfile(profile); err != nil {
		return err
	}

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
			return errors.New("invalid moderator type")
		}
		storedBool, ok := storedMod.(bool)
		if !ok {
			return errors.New("invalid moderator type")
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

	newProfile, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	p := new(pb.Profile)
	if err := jsonpb.Unmarshal(bytes.NewReader(newProfile), p); err != nil {
		return err
	}

	repoProfile, err := repo.ProfileFromProtobuf(p)
	if err != nil {
		return fmt.Errorf("building profile for validation: %s", err.Error())
	}

	if err := repoProfile.Valid(); err != nil {
		return fmt.Errorf("invalid profile: %s", err.Error())
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
		return errors.New("handle should not contain @")
	}
	if len(profile.Handle) > repo.WordMaxCharacters {
		return fmt.Errorf("handle character length is greater than the max of %d", repo.WordMaxCharacters)
	}
	if len(profile.Name) == 0 {
		return errors.New("profile name not set")
	}
	if len(profile.Name) > repo.WordMaxCharacters {
		return fmt.Errorf("name character length is greater than the max of %d", repo.WordMaxCharacters)
	}
	if len(profile.Location) > repo.WordMaxCharacters {
		return fmt.Errorf("location character length is greater than the max of %d", repo.WordMaxCharacters)
	}
	if len(profile.About) > repo.AboutMaxCharacters {
		return fmt.Errorf("about character length is greater than the max of %d", repo.AboutMaxCharacters)
	}
	if len(profile.ShortDescription) > repo.ShortDescriptionLength {
		return fmt.Errorf("short description character length is greater than the max of %d", repo.ShortDescriptionLength)
	}
	if profile.ContactInfo != nil {
		if len(profile.ContactInfo.Website) > repo.URLMaxCharacters {
			return fmt.Errorf("website character length is greater than the max of %d", repo.URLMaxCharacters)
		}
		if len(profile.ContactInfo.Email) > repo.SentenceMaxCharacters {
			return fmt.Errorf("email character length is greater than the max of %d", repo.SentenceMaxCharacters)
		}
		if len(profile.ContactInfo.PhoneNumber) > repo.WordMaxCharacters {
			return fmt.Errorf("phone number character length is greater than the max of %d", repo.WordMaxCharacters)
		}
		if len(profile.ContactInfo.Social) > repo.MaxListItems {
			return fmt.Errorf("number of social accounts is greater than the max of %d", repo.MaxListItems)
		}
		for _, s := range profile.ContactInfo.Social {
			if len(s.Username) > repo.WordMaxCharacters {
				return fmt.Errorf("social username character length is greater than the max of %d", repo.WordMaxCharacters)
			}
			if len(s.Type) > repo.WordMaxCharacters {
				return fmt.Errorf("social account type character length is greater than the max of %d", repo.WordMaxCharacters)
			}
			if len(s.Proof) > repo.URLMaxCharacters {
				return fmt.Errorf("social proof character length is greater than the max of %d", repo.WordMaxCharacters)
			}
		}
	}
	if profile.ModeratorInfo != nil {
		if len(profile.ModeratorInfo.Description) > repo.AboutMaxCharacters {
			return fmt.Errorf("moderator description character length is greater than the max of %d", repo.AboutMaxCharacters)
		}
		if len(profile.ModeratorInfo.TermsAndConditions) > repo.PolicyMaxCharacters {
			return fmt.Errorf("moderator terms and conditions character length is greater than the max of %d", repo.PolicyMaxCharacters)
		}
		if len(profile.ModeratorInfo.Languages) > repo.MaxListItems {
			return fmt.Errorf("moderator number of languages greater than the max of %d", repo.MaxListItems)
		}
		for _, l := range profile.ModeratorInfo.Languages {
			if len(l) > repo.WordMaxCharacters {
				return fmt.Errorf("moderator language character length is greater than the max of %d", repo.WordMaxCharacters)
			}
		}
		if profile.ModeratorInfo.Fee != nil {
			if profile.ModeratorInfo.Fee.FixedFee != nil &&
				len(profile.ModeratorInfo.Fee.FixedFee.AmountCurrency.Code) > repo.WordMaxCharacters {
				return fmt.Errorf("moderator fee currency code character length is greater than the max of %d", repo.WordMaxCharacters)
			}
		}
	}
	if profile.AvatarHashes != nil && (profile.AvatarHashes.Large != "" || profile.AvatarHashes.Medium != "" ||
		profile.AvatarHashes.Small != "" || profile.AvatarHashes.Tiny != "" || profile.AvatarHashes.Original != "") {
		_, err := cid.Decode(profile.AvatarHashes.Tiny)
		if err != nil {
			return errors.New("tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Small)
		if err != nil {
			return errors.New("small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Medium)
		if err != nil {
			return errors.New("medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Large)
		if err != nil {
			return errors.New("large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.AvatarHashes.Original)
		if err != nil {
			return errors.New("original image hashes must be properly formatted CID")
		}
	}
	if profile.HeaderHashes != nil && (profile.HeaderHashes.Large != "" || profile.HeaderHashes.Medium != "" ||
		profile.HeaderHashes.Small != "" || profile.HeaderHashes.Tiny != "" || profile.HeaderHashes.Original != "") {
		_, err := cid.Decode(profile.HeaderHashes.Tiny)
		if err != nil {
			return errors.New("tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Small)
		if err != nil {
			return errors.New("small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Medium)
		if err != nil {
			return errors.New("medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Large)
		if err != nil {
			return errors.New("large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(profile.HeaderHashes.Original)
		if err != nil {
			return errors.New("original image hashes must be properly formatted CID")
		}
	}
	if len(profile.BitcoinPubkey) > 66 {
		return fmt.Errorf("bitcoin public key character length is greater than the max of %d", 66)
	}
	if profile.Stats != nil {
		if profile.Stats.AverageRating > 5 {
			return fmt.Errorf("average rating cannot be greater than %d", 5)
		}
	}
	return nil
}
