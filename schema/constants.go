package schema

import "errors"

const (
	// SQL Statements
	PragmaUserVersionSQL                    = "pragma user_version = 0;"
	CreateTableConfigSQL                    = "create table config (key text primary key not null, value blob);"
	CreateTableFollowersSQL                 = "create table followers (peerID text primary key not null, proof blob);"
	CreateTableFollowingSQL                 = "create table following (peerID text primary key not null);"
	CreateTableOfflineMessagesSQL           = "create table offlinemessages (url text primary key not null, timestamp integer, message blob);"
	CreateTablePointersSQL                  = "create table pointers (pointerID text primary key not null, key text, address text, cancelID text, purpose integer, timestamp integer);"
	CreateTableKeysSQL                      = "create table keys (scriptAddress text primary key not null, purpose integer, keyIndex integer, used integer, key text, coin text);"
	CreateIndexKeysSQL                      = "create index index_keys on keys (coin);"
	CreateTableUnspentTransactionOutputsSQL = "create table utxos (outpoint text primary key not null, value text, height integer, scriptPubKey text, watchOnly integer, coin text);"
	CreateIndexUnspentTransactionOutputsSQL = "create index index_utxos on utxos (coin);"
	CreateTableSpentTransactionOutputsSQL   = "create table stxos (outpoint text primary key not null, value text, height integer, scriptPubKey text, watchOnly integer, spendHeight integer, spendTxid text, coin text);"
	CreateIndexSpentTransactionOutputsSQL   = "create index index_stxos on stxos (coin);"
	CreateTableTransactionsSQL              = "create table txns (txid text primary key not null, value text, height integer, timestamp integer, watchOnly integer, tx blob, coin text);"
	CreateIndexTransactionsSQL              = "create index index_txns on txns (coin);"
	CreateTableTransactionMetadataSQL       = "create table txmetadata (txid text primary key not null, address text, memo text, orderID text, thumbnail text, canBumpFee integer);"
	CreateTableInventorySQL                 = "create table inventory (invID text primary key not null, slug text, variantIndex integer, count integer);"
	CreateIndexInventorySQL                 = "create index index_inventory on inventory (slug);"
	CreateTablePurchasesSQL                 = "create table purchases (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, vendorID text, vendorHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, lastDisputeExpiryNotifiedAt integer not null default 0, disputedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	CreateIndexPurchasesSQL                 = "create index index_purchases on purchases (paymentAddr, timestamp);"
	CreateTableSalesSQL                     = "create table sales (orderID text primary key not null, contract blob, state integer, read integer, timestamp integer, total integer, thumbnail text, buyerID text, buyerHandle text, title text, shippingName text, shippingAddress text, paymentAddr text, funded integer, transactions blob, lastDisputeTimeoutNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	CreateIndexSalesSQL                     = "create index index_sales on sales (paymentAddr, timestamp);"
	CreatedTableWatchedScriptsSQL           = "create table watchedscripts (scriptPubKey text primary key not null, coin text);"
	CreateIndexWatchedScriptsSQL            = "create index index_watchscripts on watchedscripts (coin);"
	CreateTableDisputedCasesSQL             = "create table cases (caseID text primary key not null, buyerContract blob, vendorContract blob, buyerValidationErrors blob, vendorValidationErrors blob, buyerPayoutAddress text, vendorPayoutAddress text, buyerOutpoints blob, vendorOutpoints blob, state integer, read integer, timestamp integer, buyerOpened integer, claim text, disputeResolution blob, lastDisputeExpiryNotifiedAt integer not null default 0, coinType not null default '', paymentCoin not null default '');"
	CreateIndexDisputedCasesSQL             = "create index index_cases on cases (timestamp);"
	CreateTableChatSQL                      = "create table chat (messageID text primary key not null, peerID text, subject text, message text, read integer, timestamp integer, outgoing integer);"
	CreateIndexChatSQL                      = "create index index_chat on chat (peerID, subject, read, timestamp);"
	CreateTableNotificationsSQL             = "create table notifications (notifID text primary key not null, serializedNotification blob, type text, timestamp integer, read integer);"
	CreateIndexNotificationsSQL             = "create index index_notifications on notifications (read, type, timestamp);"
	CreateTableCouponsSQL                   = "create table coupons (slug text, code text, hash text);"
	CreateIndexCouponsSQL                   = "create index index_coupons on coupons (slug);"
	CreateTableModeratedStoresSQL           = "create table moderatedstores (peerID text primary key not null);"
	CreateMessagesSQL                       = "create table messages (messageID text primary key not null, orderID text, message_type integer, message blob, peerID text, url text, acknowledged bool, tries integer, created_at integer, updated_at integer, err string, received_at integer, pubkey blob);"
	CreateIndexMessagesSQLMessageID         = "create index index_messages_messageID on messages (messageID);"
	CreateIndexMessagesSQLOrderIDMType      = "create index index_messages_orderIDmType on messages (orderID, message_type);"
	CreateIndexMessagesSQLPeerIDMType       = "create index index_messages_peerIDmType on messages (peerID, message_type);"
	// End SQL Statements

	// Configuration defaults
	EthereumRegistryAddressMainnet = "0x5c69ccf91eab4ef80d9929b3c1b4d5bc03eb0981"
	EthereumRegistryAddressRinkeby = "0x5cEF053c7b383f430FC4F4e1ea2F7D31d8e2D16C"
	EthereumRegistryAddressRopsten = "0x403d907982474cdd51687b09a8968346159378f3"

	DataPushNodeOne   = "QmbwN82MVyBukT7WTdaQDppaACo62oUfma8dUa5R9nBFHm"
	DataPushNodeTwo   = "QmPPg2qeF3n2KvTRXRZLaTwHCw8JxzF4uZK93RfMoDvf2o"
	DataPushNodeThree = "QmY8puEnVx66uEet64gAf4VZRo7oUyMCwG6KdB9KM92EGQ"

	BootstrapNodeTestnet_BrooklynFlea     = "/ip4/165.227.117.91/tcp/4001/ipfs/Qmaa6De5QYNqShzPb9SGSo8vLmoUte8mnWgzn4GYwzuUYA"
	BootstrapNodeTestnet_Shipshewana      = "/ip4/46.101.221.165/tcp/4001/ipfs/QmVAQYg7ygAWTWegs8HSV2kdW1MqW8WMrmpqKG1PQtkgTC"
	BootstrapNodeDefault_LeMarcheSerpette = "/ip4/107.170.133.32/tcp/4001/ipfs/QmUZRGLhcKXF1JyuaHgKm23LvqcoMYwtb9jmh8CkP4og3K"
	BootstrapNodeDefault_BrixtonVillage   = "/ip4/139.59.174.197/tcp/4001/ipfs/QmZfTbnpvPwxCjpCG3CXJ7pfexgkBZ2kgChAiRJrTK1HsM"
	BootstrapNodeDefault_Johari           = "/ip4/139.59.6.222/tcp/4001/ipfs/QmRDcEDK9gSViAevCHiE6ghkaBCU7rTuQj4BDpmCzRvRYg"

	IPFSCachingRouterDefaultURI = "https://routing.api.openbazaar.org"
	// End Configuration defaults
)

var (
	// Errors
	ErrorEmptyMnemonic = errors.New("mnemonic string must not be empty")
	// End Errors
)

var (
	DataPushNodes = []string{DataPushNodeOne, DataPushNodeTwo, DataPushNodeThree}

	BootstrapAddressesDefault = []string{
		BootstrapNodeDefault_LeMarcheSerpette,
		BootstrapNodeDefault_BrixtonVillage,
		BootstrapNodeDefault_Johari,
	}
	BootstrapAddressesTestnet = []string{
		BootstrapNodeTestnet_BrooklynFlea,
		BootstrapNodeTestnet_Shipshewana,
	}
)

func EthereumDefaultOptions() map[string]interface{} {
	return map[string]interface{}{
		"RegistryAddress":        EthereumRegistryAddressMainnet,
		"RinkebyRegistryAddress": EthereumRegistryAddressRinkeby,
		"RopstenRegistryAddress": EthereumRegistryAddressRopsten,
	}
}

const (
	WalletTypeAPI = "API"
	WalletTypeSPV = "SPV"
)

const (
	CoinAPIOpenBazaarBTC = "https://btc.api.openbazaar.org/api"
	CoinAPIOpenBazaarBCH = "https://bch.api.openbazaar.org/api"
	CoinAPIOpenBazaarLTC = "https://ltc.api.openbazaar.org/api"
	CoinAPIOpenBazaarZEC = "https://zec.api.openbazaar.org/api"
	CoinAPIOpenBazaarETH = "https://mainnet.infura.io"
	CoinAPIOpenBazaarFIL = "http://localhost:1234"

	CoinAPIOpenBazaarTBTC = "https://tbtc.api.openbazaar.org/api"
	CoinAPIOpenBazaarTBCH = "https://tbch.api.openbazaar.org/api"
	CoinAPIOpenBazaarTLTC = "https://tltc.api.openbazaar.org/api"
	CoinAPIOpenBazaarTZEC = "https://tzec.api.openbazaar.org/api"
	CoinAPIOpenBazaarTETH = "https://rinkeby.infura.io"
	CoinAPIOpenBazaarTFIL = "http://localhost:1234"
)

var (
	CoinPoolBTC = []string{CoinAPIOpenBazaarBTC}
	CoinPoolBCH = []string{CoinAPIOpenBazaarBCH}
	CoinPoolLTC = []string{CoinAPIOpenBazaarLTC}
	CoinPoolZEC = []string{CoinAPIOpenBazaarZEC}
	CoinPoolETH = []string{CoinAPIOpenBazaarETH}
	CoinPoolFIL = []string{CoinAPIOpenBazaarFIL}

	CoinPoolTBTC = []string{CoinAPIOpenBazaarTBTC}
	CoinPoolTBCH = []string{CoinAPIOpenBazaarTBCH}
	CoinPoolTLTC = []string{CoinAPIOpenBazaarTLTC}
	CoinPoolTZEC = []string{CoinAPIOpenBazaarTZEC}
	CoinPoolTETH = []string{CoinAPIOpenBazaarTETH}
	CoinPoolTFIL = []string{CoinAPIOpenBazaarTFIL}
)
