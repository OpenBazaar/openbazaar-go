package repo

import (
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/spvwallet"
	btc "github.com/btcsuite/btcutil"
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
	Purchases() Purchases
	Sales() Sales
	Cases() Cases
	Close()
}

type Config interface {
	/* Initialize the database with the node's mnemonic seed and
	   identity key. This will be called during repo init. */
	Init(mnemonic string, identityKey []byte, password string) error

	// Return the mnemonic string
	GetMnemonic() (string, error)

	// Return the identity key
	GetIdentityKey() ([]byte, error)

	// Returns true if the database has failed to decrypt properly ex) wrong pw
	IsEncrypted() bool
}

type Followers interface {
	// Put a B58 encoded follower ID to the database
	Put(follower string) error

	/* Get followers from the database.
	   The offset and limit arguments can be used to for lazy loading. */
	Get(offsetId string, limit int) ([]string, error)

	// Delete a follower from the databse
	Delete(follower string) error

	// Return the number of followers in the database
	Count() int

	// Are we followed by this peer?
	FollowsMe(peerId string) bool
}

type Following interface {
	// Put a B58 encoded peer ID to the database
	Put(peer string) error

	/* Get a list of following peers from the database.
	   The offset and limit arguments can be used to for lazy loading. */
	Get(offsetId string, limit int) ([]string, error)

	// Delete a peer from the database
	Delete(peer string) error

	// Return the number of peers in the database
	Count() int

	// Am I following this peer?
	IsFollowing(peerId string) bool
}

type OfflineMessages interface {
	// Put a URL from a retrieved message
	Put(url string) error

	// Does the given URL exist in the database?
	Has(url string) bool
}

type Pointers interface {
	// Put a pointer to the database
	Put(p ipfs.Pointer) error

	// Delete a pointer from the database
	Delete(id peer.ID) error

	// Delete all pointers of a given purpose
	DeleteAll(purpose ipfs.Purpose) error

	// Fetch the entire list of pointers
	GetAll() ([]ipfs.Pointer, error)
}

type Settings interface {
	// Put settings to the database, overriding all fields
	Put(settings SettingsData) error

	// Update all non-nil fields
	Update(settings SettingsData) error

	// Return the settings object
	Get() (SettingsData, error)
}

type Inventory interface {
	/* Put an inventory count for a listing
	   Override the existing count if it exists */
	Put(slug string, count int) error

	// Return the count for a specific listing including variants
	GetSpecific(path string) (int, error)

	// Get the count for all variants of a given listing
	Get(slug string) (map[string]int, error)

	// Fetch all inventory countes
	GetAll() (map[string]int, error)

	// Delete a listing and related count
	Delete(path string) error

	// Delete all variants of a given slug
	DeleteAll(slug string) error
}

type Purchases interface {
	// Save or update an order
	Put(orderID string, contract pb.RicardianContract, state pb.OrderState, read bool) error

	// Mark an order as read in the database
	MarkAsRead(orderID string) error

	// Update the funding level for the contract
	UpdateFunding(orderId string, funded bool, records []*spvwallet.TransactionRecord) error

	// Delete an order
	Delete(orderID string) error

	// Return a purchase given the payment address
	GetByPaymentAddress(addr btc.Address) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*spvwallet.TransactionRecord, err error)

	// Return a purchase given the order ID
	GetByOrderId(orderId string) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*spvwallet.TransactionRecord, read bool, err error)

	// Return the IDs for all orders
	GetAll() ([]string, error)
}

type Sales interface {
	// Save or update a sale
	Put(orderID string, contract pb.RicardianContract, state pb.OrderState, read bool) error

	// Mark an order as read in the database
	MarkAsRead(orderID string) error

	// Update the funding level for the contract
	UpdateFunding(orderId string, funded bool, records []*spvwallet.TransactionRecord) error

	// Delete an order
	Delete(orderID string) error

	// Return a sale given the payment address
	GetByPaymentAddress(addr btc.Address) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*spvwallet.TransactionRecord, err error)

	// Return a sale given the order ID
	GetByOrderId(orderId string) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*spvwallet.TransactionRecord, read bool, err error)

	// Return the IDs for all orders
	GetAll() ([]string, error)
}

type Cases interface {
	// Save or update a sale
	Put(orderID string, buyerContract, vendorContract pb.RicardianContract, state pb.OrderState, read bool) error

	// Mark a case as read in the database
	MarkAsRead(orderID string) error

	// Delete a case
	Delete(orderID string) error

	// Return a sale given the order ID
	GetByOrderId(orderId string) (buyerContract, vendorContract *pb.RicardianContract, state pb.OrderState, read bool, err error)

	// Return the IDs for all cases
	GetAll() ([]string, error)
}
