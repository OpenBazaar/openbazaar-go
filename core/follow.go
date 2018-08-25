package core

import (
	"encoding/json"
	"os"
	"path"
)

/*UpdateFollow This function updates the follow and following lists in the node's root directory
  as well as adds the current follow, following, and listing counts to the profile.
  We only do this when a user updates his node to avoid needing to make network calls
  each time a new follower or unfollow request comes in. */
func (n *OpenBazaarNode) UpdateFollow() error {
	followPath := path.Join(n.RepoPath, "root", "followers.json")
	followingPath := path.Join(n.RepoPath, "root", "following.json")

	// Update followers file
	followers, err := n.Datastore.Followers().Get("", -1)
	if err != nil {
		return err
	}
	f1, err := os.Create(followPath)
	if err != nil {
		return err
	}
	defer f1.Close()

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
	if err != nil {
		return err
	}
	defer f2.Close()

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

	return n.updateProfileCounts()
}
