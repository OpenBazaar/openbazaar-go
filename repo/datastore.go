package repo

import (
	b32 "github.com/tyler-smith/go-bip32"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
)

type Datastore interface {
	Config() Config
	Followers() Followers
	Following() Following
	OfflineMessages() OfflineMessages
	Pointers() Pointers
	Keys() Keys
	Close()
}

type Config interface {
	// Initialize the database with the node's mnemonic seed and
	// identity key. This will be called during repo init
	Init(mnemonic string, identityKey []byte, password string) error

	// Return the mnemonic string
	GetMnemonic() (string, error)

	// Return the identity key
	GetIdentityKey() ([]byte, error)

	// Returns true if the db has failed to decrypt properly ex) wrong pw
	IsEncrypted() bool
}

type Followers interface {
	// Put a B58 encoded follower ID to the database
	Put(follower string) error

	// Get followers from the database.
	// The offset and limit arguments can be used to for lazy loading.
	Get(offset int, limit int) ([]string, error)

	// Delete a follower from the databse.
	Delete(follower string) error

	// Return the number of followers in the database.
	Count() int
}

type Following interface {
	// Put a B58 encoded peer ID to the database
	Put(peer string) error

	// Get a list of following peers from the database.
	// The offset and limit arguments can be used to for lazy loading.
	Get(offset int, limit int) ([]string, error)

	// Delete a peer from the databse.
	Delete(peer string) error

	// Return the number of peers in the database.
	Count() int
}

type OfflineMessages interface {

	// Put a url from a retrieved message
	Put(url string) error

	// Does the given url exist in the db?
	Exists(url string) bool
}

type Pointers interface {

	// Put a pointer to the database.
	Put(p ipfs.Pointer) error

	// Delete a pointer from the db.
	Delete(id peer.ID) error

	// Fetch the entire list of pointers
	GetAll() ([]ipfs.Pointer, error)
}

type Keys interface {

	// Put a bip32 key to the database
	Put(key *b32.Key, scriptPubKey []byte, purpose bitcoin.KeyPurpose) error

	// Mark the given key as used
	MarkKeyAsUsed(key *b32.Key) error

	// Fetch the key at the last index for the given purpose
	// The bool should state whether the key has been used or not
	GetLastKey(purpose bitcoin.KeyPurpose) (*b32.Key, bool, error)

	// Given a scriptPubKey return the corresponding bip32 key
	GetKeyForScript(scriptPubKey []byte) (*b32.Key, error)
}
