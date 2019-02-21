package core_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"

	"github.com/OpenBazaar/openbazaar-go/core"

	"github.com/OpenBazaar/openbazaar-go/test"
)

func TestOpenBazaarFollow_UpdateFollow(t *testing.T) {
	node, err := test.NewNode()
	if err != nil {
		t.Error(err)
	}

	err = node.UpdateFollow()
	if err != nil {
		t.Error(err)
	}

	filepath_followers, _ := filepath.Abs("../test/files/followers_1.json")
	filepath_following, _ := filepath.Abs("../test/files/following_1.json")

	node.Datastore.Followers().Put("QmTestFollower", []byte("TEST"))
	node.Datastore.Following().Put("QmTestFollowing")

	followers, _ := node.Datastore.Followers().Get("", -1)
	following, _ := node.Datastore.Following().Get("", -1)

	// Check no filepath specified
	err = core.UpdateConnectionsInFile("", nil)
	if err == nil {
		t.Fail()
	}

	err = core.UpdateConnectionsInFile(filepath_followers, followers)
	if err != nil {
		t.Error(err)
	}
	err = core.UpdateConnectionsInFile(filepath_following, following)
	if err != nil {
		t.Error(err)
	}

	followers = []repo.Follower{
		{"QmTEST", []byte("bytes")},
	}
	followerBytes, err := core.ConnectionsToBytes(followers)
	if err != nil {
		t.Error(err)
	}

	test_bytes := []uint8{91, 10, 32, 32, 32, 32, 123, 10, 32, 32, 32, 32, 32, 32, 32, 32, 34, 112, 101, 101, 114, 73, 100, 34, 58, 32, 34, 81, 109, 84, 69, 83, 84, 34, 44, 10, 32, 32, 32, 32, 32, 32, 32, 32, 34, 112, 114, 111, 111, 102, 34, 58, 32, 34, 89, 110, 108, 48, 90, 88, 77, 61, 34, 10, 32, 32, 32, 32, 125, 10, 93}
	if string(followerBytes) != string(test_bytes) {
		t.Fail()
	}

	following = []string{
		"QmTEST",
	}
	followingBytes, err := core.ConnectionsToBytes(following)
	if err != nil {
		t.Error(err)
	}

	test_bytes = []uint8{91, 10, 32, 32, 32, 32, 34, 81, 109, 84, 69, 83, 84, 34, 10, 93}
	if string(followingBytes) != string(test_bytes) {
		fmt.Println(followingBytes)
		t.Fail()
	}
}
