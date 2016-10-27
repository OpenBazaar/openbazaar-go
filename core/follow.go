package core

import (
	"encoding/json"
	"github.com/OpenBazaar/jsonpb"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"io/ioutil"
	"os"
	"path"
)

/* This function updates the follow and following lists in the node's root directory
   as well as adds the current follow, following, and listing counts to the profile.
   We only do this when a user updates his node to avoid needing to make network calls
   each time a new follower or unfollow request comes in. */
func (n *OpenBazaarNode) UpdateFollow() error {
	followPath := path.Join(n.RepoPath, "root", "followers")
	followingPath := path.Join(n.RepoPath, "root", "following")
	profilePath := path.Join(n.RepoPath, "root", "profile")

	// Update followers file
	followers, err := n.Datastore.Followers().Get("", -1)
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
	following, err := n.Datastore.Following().Get("", -1)
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
		// Read existing file
		file, err := ioutil.ReadFile(profilePath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(file, profile)
		if err != nil {
			return err
		}
	}

	profile, err = n.appendCountsToProfile(profile)
	if err != nil {
		return err
	}

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
