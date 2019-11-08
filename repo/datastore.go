package repo

import (
	"database/sql"
	"math/big"

	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/wallet-interface"
	btc "github.com/btcsuite/btcutil"
)

type Datastore interface {
	Config() Config
	Followers() FollowerStore
	Following() FollowingStore
	OfflineMessages() OfflineMessageStore
	Pointers() PointerStore
	Settings() ConfigurationStore
	Inventory() InventoryStore
	Purchases() PurchaseStore
	Sales() SaleStore
	Cases() CaseStore
	Chat() ChatStore
	Notifications() NotificationStore
	Coupons() CouponStore
	TxMetadata() TransactionMetadataStore
	ModeratedStores() ModeratedStore
	Messages() MessageStore
	Ping() error
	Close()
}

type Queryable interface {
	Lock()
	Unlock()
	BeginTransaction() (*sql.Tx, error)
	PrepareQuery(string) (*sql.Stmt, error)
	PrepareAndExecuteQuery(string, ...interface{}) (*sql.Rows, error)
	ExecuteQuery(string, ...interface{}) (sql.Result, error)
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

type FollowerStore interface {
	Queryable

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

type FollowingStore interface {
	Queryable

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

type OfflineMessageStore interface {
	Queryable

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

type PointerStore interface {
	Queryable

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

type ConfigurationStore interface {
	Queryable

	// Put settings to the database, overriding all fields
	Put(settings SettingsData) error

	// Update all non-nil fields
	Update(settings SettingsData) error

	// Return the settings object
	Get() (SettingsData, error)

	// Delete all settings data
	Delete() error
}

type InventoryStore interface {
	Queryable

	/* Put an inventory count for a listing
	   Override the existing count if it exists */
	Put(slug string, variantIndex int, count *big.Int) error

	// Return the count for a specific listing including variants
	GetSpecific(slug string, variantIndex int) (*big.Int, error)

	// Get the count for all variants of a given listing
	Get(slug string) (map[int]*big.Int, error)

	// Fetch all inventory maps for each slug
	GetAll() (map[string]map[int]*big.Int, error)

	// Delete a listing and related count
	Delete(slug string, variant int) error

	// Delete all variants of a given slug
	DeleteAll(slug string) error
}

type PurchaseStore interface {
	Queryable

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
	GetByOrderId(orderId string) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord, read bool, currencyCode *CurrencyCode, err error)

	// Return the metadata for all purchases. Also returns the original size of the query.
	GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]Purchase, int, error)

	// Return unfunded orders.
	GetUnfunded() ([]UnfundedOrder, error)

	// Return the number of purchases in the database
	Count() int

	// GetPurchasesForDisputeTimeoutNotification returns []*PurchaseRecord including
	// each record which needs buyerDisputeTimeout Notifications to be generated.
	GetPurchasesForDisputeTimeoutNotification() ([]*PurchaseRecord, error)

	// GetPurchasesForDisputeExpiryNotification returns []*PurchaseRecord including
	// each record which needs buyerDisputeExpiry Notifications to be generated.
	GetPurchasesForDisputeExpiryNotification() ([]*PurchaseRecord, error)

	// UpdatePurchasesLastDisputeTimeoutNotifiedAt  accepts []*PurchaseRecord and updates each records lastDisputeTimeoutNotifiedAt by its OrderID
	UpdatePurchasesLastDisputeTimeoutNotifiedAt([]*PurchaseRecord) error

	// UpdatePurchasesLastDisputeExpiryNotifiedAt  accepts []*PurchaseRecord and updates each records lastDisputeExpiryNotifiedAt by its OrderID
	UpdatePurchasesLastDisputeExpiryNotifiedAt([]*PurchaseRecord) error
}

type SaleStore interface {
	Queryable

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
	GetByOrderId(orderId string) (contract *pb.RicardianContract, state pb.OrderState, funded bool, records []*wallet.TransactionRecord, read bool, currencyCode *CurrencyCode, err error)

	// Return the metadata for all sales. Also returns the original size of the query.
	GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]Sale, int, error)

	// Return unfunded orders.
	GetUnfunded() ([]UnfundedOrder, error)

	// Return the number of sales in the database
	Count() int

	// GetSalesForDisputeTimeoutNotification returns []*SaleRecord including
	// each record which needs Notifications to be generated.
	GetSalesForDisputeTimeoutNotification() ([]*SaleRecord, error)

	// UpdateSalesLastDisputeTimeoutNotifiedAt  accepts []*SaleRecord and updates each records lastDisputeTimeoutNotifiedAt by its CaseID
	UpdateSalesLastDisputeTimeoutNotifiedAt([]*SaleRecord) error
}

type CaseStore interface {
	Queryable

	// Save a new case
	Put(caseID string, state pb.OrderState, buyerOpened bool, claim string, paymentCoin string, coinType string) error

	// Save a new case
	PutRecord(*DisputeCaseRecord) error

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

	// GetByCaseID returns the dispute payout data for a case
	GetByCaseID(caseID string) (*DisputeCaseRecord, error)

	// Return the metadata for all cases given the search terms. Also returns the original size of the query.
	GetAll(stateFilter []pb.OrderState, searchTerm string, sortByAscending bool, sortByRead bool, limit int, exclude []string) ([]Case, int, error)

	// Return the number of cases in the database
	Count() int

	// GetDisputesForDisputeExpiryNotification returns []*DisputeCaseRecord including
	// each record which needs Notifications to be generated.
	GetDisputesForDisputeExpiryNotification() ([]*DisputeCaseRecord, error)

	// UpdateDisputesLastDisputeExpiryNotifiedAt accepts []*DisputeCaseRecord and updates each records lastDisputeExpiryNotifiedAt by its CaseID
	UpdateDisputesLastDisputeExpiryNotifiedAt([]*DisputeCaseRecord) error
}

type ChatStore interface {
	Queryable

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

type NotificationStore interface {
	Queryable

	// PutRecord persists a Notification to the database
	PutRecord(*Notification) error

	// Mark notification as read
	MarkAsRead(notifID string) error

	// Mark all notifications as read
	MarkAllAsRead() error

	// Fetch notifications from database
	GetAll(offsetID string, limit int, typeFilter []string) ([]*Notification, int, error)

	// Returns the unread count for all notifications
	GetUnreadCount() (int, error)

	// Delete a notification
	Delete(notifID string) error
}

type CouponStore interface {
	Queryable

	// Put a list of coupons to the db
	Put(coupons []Coupon) error

	// Get a list of coupons given a slug
	Get(slug string) ([]Coupon, error)

	// Delete all coupons for a given slug
	Delete(slug string) error
}

type TransactionMetadataStore interface {
	Queryable

	// Put metadata for a transaction to the db
	Put(m Metadata) error

	// Get the metadata given the txid
	Get(txid string) (Metadata, error)

	// Get a map of the txid to each metadata object
	GetAll() (map[string]Metadata, error)

	// Delete a metadata entry
	Delete(txid string) error
}

type ModeratedStore interface {
	Queryable

	// Put a B58 encoded peer ID to the database
	Put(peerId string) error

	/* Get the moderated store list from the database.
	   The offset and limit arguments can be used to for lazy loading. */
	Get(offsetId string, limit int) ([]string, error)

	// Delete a moderated store from the database
	Delete(peerId string) error
}

type KeyStore interface {
	Queryable
	wallet.Keys
}

type SpentTransactionOutputStore interface {
	Queryable
	wallet.Stxos
}

type TransactionStore interface {
	Queryable
	wallet.Txns
}

type UnspentTransactionOutputStore interface {
	Queryable
	wallet.Utxos
}

type WatchedScriptStore interface {
	Queryable
	wallet.WatchedScripts
}

// MessageStore is the messages table interface
type MessageStore interface {
	Queryable

	// Save a new message
	Put(messageID, orderID string, mType pb.Message_MessageType, peerID string, msg Message, err string, receivedAt int64, pubkey []byte) error

	// GetByOrderIDType returns the message for specified order and type
	GetByOrderIDType(orderID string, mType pb.Message_MessageType) (*Message, string, error)

	// GetAllErrored returns the all messages with error
	GetAllErrored() ([]OrderMessage, error)
}
