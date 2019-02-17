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
	UpdateConnectionsInFile(followPath, followers)

	// Update following file
	following, err := n.Datastore.Following().Get("", -1)
	if err != nil {
		return err
	}
	UpdateConnectionsInFile(followingPath, following)

	return n.updateProfileCounts()
}

func UpdateConnectionsInFile(filepath string, connections interface{}) error {

	f2, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f2.Close()

	connections_bytes, err := ConnectionsToBytes(connections)
	if err != nil {
		return err
	}

	_, werr := f2.Write(connections_bytes)
	if werr != nil {
		return werr
	}

	return nil
}

func ConnectionsToBytes(connections interface{}) ([]byte, error) {
	j, jerr := json.MarshalIndent(connections, "", "    ")
	if jerr != nil {
		return nil, jerr
	}
	if string(j) == "null" {
		j = []byte("[]")
	}

	return j, nil
}
