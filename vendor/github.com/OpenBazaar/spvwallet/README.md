# spvwallet

Lightweight p2p SPV wallet in Go. It connects directly to the bitcoin p2p network to fetch headers, merkle blocks, and transactions.
It mostly utilizes utilities from btcd and is partially based on https://github.com/LightningNetwork/lnd/tree/master/uspv.

## Interfaces
These are the used interfaces:

#### Database

A sqlite implementation is included for testing but you should probably implement your own custom solution. Note there is no encryption in the example db.

* Keys

The wallet manages an hd keychain (m/0'/change/index/) and stores they keys in this database. Used keys are marked in the db and a lookahead window is maintained to ensure all transactions can be recovered when restoring from seed.

* Utxos

Stores all the utxos.  The goal of bitcoin is to get lots of utxos, earning a high score.

* Stxos

For record keeping. Stores what used to be utxos, but are no longer "u"txos, and are spent outpoints.  It references the spending txid.

* Txns

This bucket stores full serialized transactions which are refenced in the Stxos bucket.  These can be used to re-play transactions in the case of re-orgs.

* State

This has describes some miscellaneous global state variables of the database, such as what height it has synchronized up to.

#### Header file (currently headers.bin)

This is currently a bolt db which stores the chain of headers as well as orphans. A separate bucks tracks the tip of the chain. Cumulative work is calculated for each header and enables us to smoothly handle reorgs.

## Synchronization overview

At startup addresses are gathered from the DNS seeds and it will maintain one or more connections (determined by the MAX_PEERS constant).  It first asks for headers, providing the last known header, then loops through asking for headers until it receives an empty header message, which signals that headers are fully synchronized.

After header synchronization is complete, it requests merkle blocks starting at the last db height recorded in state. If the height is zero it requests from the last checkpoiint. Bloom filters are generated for the addresses and utxos known to the wallet.  If too many false positives are received, a new filter is generated and sent. Once the merkle blocks have been received up to the header height, the wallet is considered synchronized and it will listen for new inv messages from the remote node.

## TODO

* Evict confirmed transactions from the db when a reorg is triggered.
* Cache peer addresses and request more using getaddr. Only use the DNS seeds if we absolutely need to.
* General refactoring, comments, and clean up.
* Unit tests.