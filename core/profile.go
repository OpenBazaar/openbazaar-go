package core

import (
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"os"
	"path"
	"time"
)

func (n *OpenBazaarNode) GetProfile() (pb.Profile, error) {
	var profile pb.Profile
	f, err := os.Open(path.Join(n.RepoPath, "root", "profile"))
	defer f.Close()
	if err != nil {
		return profile, err
	}
	err = jsonpb.Unmarshal(f, &profile)
	if err != nil {
		return profile, err
	}
	return profile, nil
}

func (n *OpenBazaarNode) UpdateProfile(profile *pb.Profile) error {
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

func (n *OpenBazaarNode) appendCountsToProfile(profile *pb.Profile) (*pb.Profile, error) {
	profile.ListingCount = uint32(n.GetListingCount())
	profile.FollowerCount = uint32(n.Datastore.Followers().Count())
	profile.FollowingCount = uint32(n.Datastore.Following().Count())
	profile.LastModified = uint32(time.Now().Unix())
	return profile, nil
}
