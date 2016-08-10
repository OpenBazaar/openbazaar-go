package core

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/gogo/protobuf/jsonpb"
	"os"
	"path"
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
