package repo

import (
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
)

type Datastore interface {
	Config() Config
	Followers() Followers
	Following() Following
	OfflineMessages() OfflineMessages
	Pointers() Pointers
	Settings() Settings
	Inventory() Inventory
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
	Has(url string) bool
}

type Pointers interface {
	// Put a pointer to the database.
	Put(p ipfs.Pointer) error

	// Delete a pointer from the db.
	Delete(id peer.ID) error

	// Fetch the entire list of pointers
	GetAll() ([]ipfs.Pointer, error)
}

type Settings interface {
	// Put settings to the database
	// Override all fields
	Put(settings SettingsData) error

	// Update all non-nil fields
	Update(settings SettingsData) error

	// Return the settings object
	Get() (SettingsData, error)
}

type Inventory interface {
	// Put an inventory count for a listing
	// Override the existing count if it exists
	Put(slug string, count int) error

	// Return the count for a listing
	Get(slug string) (int, error)

	// Fetch all inventory countes
	GetAll() (map[string]int, error)

	// Delete a listing and related count
	Delete(slug string) error
}
