package core

import (
	"path"
	"os"
	"encoding/json"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"io/ioutil"
	"time"
	"github.com/golang/protobuf/jsonpb"
)

func (n *OpenBazaarNode) UpdateFollow() error {
	followPath := path.Join(n.RepoPath, "root", "followers")
	followingPath := path.Join(n.RepoPath, "root", "following")
	profilePath := path.Join(n.RepoPath, "root", "profile")

	// Update followers file
	followers, err := n.Datastore.Followers().Get(0, -1)
	if err != nil {
		return err
	}
	f1, err := os.Create(followPath)
	defer f1.Close()
	if err != nil {
		return err
	}

	j, jerr := json.MarshalIndent(followers, "", "    ")
	if jerr != nil {
		return jerr
	}
	if string(j) == "null" {
		j = []byte("[]")
	}
	_, werr := f1.Write(j)
	if werr != nil {
		return werr
	}

	// Update following file
	following, err := n.Datastore.Following().Get(0, -1)
	if err != nil {
		return err
	}
	f2, err := os.Create(followingPath)
	defer f2.Close()
	if err != nil {
		return err
	}

	j, jerr = json.MarshalIndent(following, "", "    ")
	if jerr != nil {
		return jerr
	}
	if string(j) == "null" {
		j = []byte("[]")
	}
	_, werr = f2.Write(j)
	if werr != nil {
		return werr
	}

	// Update profile counts
	profile := new(pb.Profile)

	_, ferr := os.Stat(profilePath)
	if !os.IsNotExist(ferr) {
		// read existing file
		file, err := ioutil.ReadFile(profilePath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(file, profile)
		if err != nil {
			return err
		}
	}
	profile.FollowerCount = uint32(n.Datastore.Followers().Count())
	profile.FollowingCount = uint32(n.Datastore.Following().Count())
	profile.LastModified = uint64(time.Now().Unix())

	f3, err := os.Create(profilePath)
	defer f3.Close()
	if err != nil {
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

	if _, err := f3.WriteString(out); err != nil {
		return err
	}

	return nil
}
