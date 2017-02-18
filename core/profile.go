package core

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/btcsuite/goleveldb/leveldb/errors"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/imdario/mergo"
	ipnspath "github.com/ipfs/go-ipfs/path"
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
	return pro, nil
}

func (n *OpenBazaarNode) UpdateProfile(profile *pb.Profile) error {
	mPubkey, err := n.Wallet.MasterPublicKey().ECPubKey()
	if err != nil {
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

	modInfo, ok := patch["modInfo"]
	if ok {
		fee, ok := modInfo.(map[string]interface{})["fee"]
		if ok {
			fixedFee, ok := fee.(map[string]interface{})["fixedFee"]
			if ok {
				amt := fixedFee.(map[string]interface{})["amount"].(float64)
				fixedFee.(map[string]interface{})["amount"] = uint64(amt)
			}
		}
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
