package repo

import (
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"

	notif "github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	btc "github.com/btcsuite/btcutil"
	"time"
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
	Chat() Chat
	Notifications() Notifications
	Coupons() Coupons
	TxMetadata() TxMetadata
	ModeratedStores() ModeratedStores
	Ping() error
	Close()
}

type Config interface {
	/* Initialize the database with the node's mnemonic seed and
	   identity key. This will be called during repo init. */
	Init(mnemonic string, identityKey []byte, password string, creationDate time.Time) error

	// Return the mnemonic string
	GetMnemonic() (string, error)

	// Return the identity key
	GetIdentityKey() ([]byte, error)

	// Returns the date the seed was created
	GetCreationDate() (time.Time, error)

	// Returns true if the database has failed to decrypt properly ex) wrong pw
	IsEncrypted() bool
}

type Followers interface {
	// Put a B58 encoded follower ID and proof to the database
	Put(follower string, proof []byte) error

	/* Get followers from the database.
	   The offset and limit arguments can be used to for lazy loading. */
	Get(offsetId string, limit int) ([]Follower, error)

	// Delete a follower from the database
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

	// Save a message with the url
	SetMessage(url string, message []byte) error

	// Get all entries with a message
	GetMessages() (map[string][]byte, error)

	// Delete the given message
	DeleteMessage(url string) error
}

type Pointers interface {
	// Put a pointer to the database
	Put(p ipfs.Pointer) error

	// Delete a pointer from the database
	Delete(id peer.ID) error

	// Delete all pointers of a given purpose
	DeleteAll(purpose ipfs.Purpose) error

	// Fetch a specific pointer
	Get(id peer.ID) (ipfs.Pointer, error)

	// Fetch all pointers of the given type
	GetByPurpose(purpose ipfs.Purpose) ([]ipfs.Pointer, error)

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

	// Delete all settings data
	Delete() error
}

type Inventory interface {
	/* Put an inventory count for a listing
	   Override the existing count if it exists */
	Put(slug string, variantIndex int, count int) error

	// Return the count for a specific listing including variants
	GetSpecific(slug string, variantIndex int) (int, error)

	// Get the count for all variants of a given listing
	Get(slug string) (map[int]int, error)

	// Fetch all inventory maps for each slug
	GetAll() (map[string]map[int]int, error)

	// Delete a listing and related count
	Delete(slug string, variant int) error

	// Delete all variants of a given slug
	DeleteAll(slug string) error
}

type Purchases interface {
	// Save or update an order
	Put(orderID string, contract pb.RicardianContract, state pb.OrderState, read bool) error

	// Mark an order as read in the database
	MarkAsRead(orderID string) error

	// Mark an order as unread in the database
	MarkAsUnread(orderID string) error

	// Update the funding level for the contract
	UpdateFunding(orderId string, funded bool, records []*wallet.TransactionRecord) error

	// Delete an order
	Delete(orderID string) error

	// Return a purchase given the payment address
	GetByPaymentAddress(addr btc.Address) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord, err error)

	// Return a purchase given the order ID
	GetByOrderId(orderId string) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord, read bool, err error)

	// Return the metadata for all purchases. Also returns the original size of the query.
	GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]Purchase, int, error)

	// Return the number of purchases in the database
	Count() int
}

type Sales interface {
	// Save or update a sale
	Put(orderID string, contract pb.RicardianContract, state pb.OrderState, read bool) error

	// Mark an order as read in the database
	MarkAsRead(orderID string) error

	// Mark an order as unread in the database
	MarkAsUnread(orderID string) error

	// Update the funding level for the contract
	UpdateFunding(orderId string, funded bool, records []*wallet.TransactionRecord) error

	// Delete an order
	Delete(orderID string) error

	// Return a sale given the payment address
	GetByPaymentAddress(addr btc.Address) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord, err error)

	// Return a sale given the order ID
	GetByOrderId(orderId string) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord, read bool, err error)

	// Return the metadata for all sales. Also returns the original size of the query.
	GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]Sale, int, error)

	// Return the number of sales in the database
	Count() int
}

type Cases interface {
	// Save a new case
	Put(caseID string, state pb.OrderState, buyerOpened bool, claim string) error

	// Update a case with the buyer info
	UpdateBuyerInfo(caseID string, buyerContract *pb.RicardianContract, buyerValidationErrors []string, buyerPayoutAddress string, buyerOutpoints []*pb.Outpoint) error

	// Update a case with the vendor info
	UpdateVendorInfo(caseID string, vendorContract *pb.RicardianContract, vendorValidationErrors []string, vendorPayoutAddress string, vendorOutpoints []*pb.Outpoint) error

	// Mark a case as read in the database
	MarkAsRead(caseID string) error

	// Mark a case as unread in the database
	MarkAsUnread(caseID string) error

	// Mark a case as closed in the database
	MarkAsClosed(caseID string, resolution *pb.DisputeResolution) error

	// Delete a case
	Delete(caseID string) error

	// Return the case metadata given a case ID
	GetCaseMetadata(caseID string) (buyerContract, vendorContract *pb.RicardianContract, buyerValidationErrors, vendorValidationErrors []string, state pb.OrderState, read bool, timestamp time.Time, buyerOpened bool, claim string, resolution *pb.DisputeResolution, err error)

	// Return the dispute payout data for a case
	GetPayoutDetails(caseID string) (buyerContract, vendorContract *pb.RicardianContract, buyerPayoutAddress, vendorPayoutAddress string, buyerOutpoints, vendorOutpoints []*pb.Outpoint, state pb.OrderState, err error)

	// Return the metadata for all cases given the search terms. Also returns the original size of the query.
	GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]Case, int, error)

	// Return the number of cases in the database
	Count() int
}

type Chat interface {

	// Put a new chat message to the database
	Put(messageId string, peerId string, subject string, message string, timestamp time.Time, read bool, outgoing bool) error

	// Returns a list of open conversations
	GetConversations() []ChatConversation

	// A list of messages given a peer ID and a subject
	GetMessages(peerID string, subject string, offsetID string, limit int) []ChatMessage

	// Mark all chat messages for a peer as read. Returns the Id of the last seen message and
	// whether any messages were updated.
	// If message Id is specified it will only mark that message and earlier as read.
	MarkAsRead(peerID string, subject string, outgoing bool, messageId string) (string, bool, error)

	// Returns the incoming unread count for all messages of a given subject
	GetUnreadCount(subject string) (int, error)

	// Delete a message
	DeleteMessage(msgID string) error

	// Delete all messages from from a peer
	DeleteConversation(peerID string) error
}

type Notifications interface {

	// Put a new notification to the database
	Put(notifID string, notification notif.Data, notifType string, timestamp time.Time) error

	// Mark notification as read
	MarkAsRead(notifID string) error

	// Mark all notifications as read
	MarkAllAsRead() error

	// Fetch notifications from database
	GetAll(offsetID string, limit int, typeFilter []string) ([]notif.Notification, int, error)

	// Returns the unread count for all notifications
	GetUnreadCount() (int, error)

	// Delete a notification
	Delete(notifID string) error
}

type Coupons interface {

	// Put a list of coupons to the db
	Put(coupons []Coupon) error

	// Get a list of coupons given a slug
	Get(slug string) ([]Coupon, error)

	// Delete all coupons for a given slug
	Delete(slug string) error
}

type TxMetadata interface {

	// Put metadata for a transaction to the db
	Put(m Metadata) error

	// Get the metadata given the txid
	Get(txid string) (Metadata, error)

	// Get a map of the txid to each metadata object
	GetAll() (map[string]Metadata, error)

	// Delete a metadata entry
	Delete(txid string) error
}

type ModeratedStores interface {
	// Put a B58 encoded peer ID to the database
	Put(peerId string) error

	/* Get the moderated store list from the database.
	   The offset and limit arguments can be used to for lazy loading. */
	Get(offsetId string, limit int) ([]string, error)

	// Delete a moderated store from the database
	Delete(peerId string) error
}
