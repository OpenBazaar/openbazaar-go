package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	peerpkg "github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	"net"
	"time"
)

const (
	maxRequestedTxns  = wire.MaxInvPerMsg
	maxFalsePositives = 7
)

// newPeerMsg signifies a newly connected peer to the block handler.
type newPeerMsg struct {
	peer *peerpkg.Peer
}

// donePeerMsg signifies a newly disconnected peer to the block handler.
type donePeerMsg struct {
	peer *peerpkg.Peer
}

// headersMsg packages a bitcoin headers message and the peer it came from
// together so the handler has access to that information.
type headersMsg struct {
	headers *wire.MsgHeaders
	peer    *peerpkg.Peer
}

// merkleBlockMsg packages a merkle block message and the peer it came from
// together so the handler has access to that information.
type merkleBlockMsg struct {
	merkleBlock *wire.MsgMerkleBlock
	peer        *peerpkg.Peer
}

// invMsg packages a bitcoin inv message and the peer it came from together
// so the handler has access to that information.
type invMsg struct {
	inv  *wire.MsgInv
	peer *peerpkg.Peer
}

type heightAndTime struct {
	height    uint32
	timestamp time.Time
}

// txMsg packages a bitcoin tx message and the peer it came from together
// so the handler has access to that information.
type txMsg struct {
	tx    *wire.MsgTx
	peer  *peerpkg.Peer
	reply chan struct{}
}

type updateFiltersMsg struct{}

type WireServiceConfig struct {
	params             *chaincfg.Params
	chain              *Blockchain
	txStore            *TxStore
	walletCreationDate time.Time
	minPeersForSync    int
}

// peerSyncState stores additional information that the WireService tracks
// about a peer.
type peerSyncState struct {
	syncCandidate   bool
	requestQueue    []*wire.InvVect
	requestedTxns   map[chainhash.Hash]heightAndTime
	requestedBlocks map[chainhash.Hash]struct{}
	falsePositives  uint32
	blockScore      int32
}

type WireService struct {
	params             *chaincfg.Params
	chain              *Blockchain
	txStore            *TxStore
	walletCreationDate time.Time
	syncPeer           *peerpkg.Peer
	peerStates         map[*peerpkg.Peer]*peerSyncState
	requestedTxns      map[chainhash.Hash]heightAndTime
	requestedBlocks    map[chainhash.Hash]struct{}
	mempool            map[chainhash.Hash]struct{}
	msgChan            chan interface{}
	quit               chan struct{}
	minPeersForSync    int
	zeroHash           chainhash.Hash
}

func NewWireService(config *WireServiceConfig) *WireService {
	return &WireService{
		params:             config.params,
		chain:              config.chain,
		walletCreationDate: config.walletCreationDate,
		minPeersForSync:    config.minPeersForSync,
		txStore:            config.txStore,
		peerStates:         make(map[*peerpkg.Peer]*peerSyncState),
		requestedTxns:      make(map[chainhash.Hash]heightAndTime),
		requestedBlocks:    make(map[chainhash.Hash]struct{}),
		mempool:            make(map[chainhash.Hash]struct{}),
		msgChan:            make(chan interface{}),
		quit:               make(chan struct{}),
	}
}

func (ws *WireService) MsgChan() chan interface{} {
	return ws.msgChan
}

// The start function must be run in its own goroutine. The entire WireService is single
// threaded which means all messages are processed sequentially removing the need for complex
// locking.
func (ws *WireService) Start() {
	best, err := ws.chain.BestBlock()
	if err != nil {
		log.Error(err)
	}
	log.Infof("Starting wire service at height %d", int(best.height))
out:
	for {
		select {
		case m := <-ws.msgChan:
			switch msg := m.(type) {
			case newPeerMsg:
				ws.handleNewPeerMsg(msg.peer)
			case donePeerMsg:
				ws.handleDonePeerMsg(msg.peer)
			case headersMsg:
				ws.handleHeadersMsg(&msg)
			case merkleBlockMsg:
				ws.handleMerkleBlockMsg(&msg)
			case invMsg:
				ws.handleInvMsg(&msg)
			case txMsg:
				ws.handleTxMsg(&msg)
			case updateFiltersMsg:
				ws.handleUpdateFiltersMsg()
			default:
				log.Warningf("Unknown message type sent to WireService message chan: %T", msg)
			}
		case <-ws.quit:
			break out
		}
	}
}

func (ws *WireService) Stop() {
	close(ws.quit)
}

func (ws *WireService) handleNewPeerMsg(peer *peerpkg.Peer) {
	// Initialize the peer state
	ws.peerStates[peer] = &peerSyncState{
		syncCandidate:   ws.isSyncCandidate(peer),
		requestedTxns:   make(map[chainhash.Hash]heightAndTime),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
	}

	ws.updateFilterAndSend(peer)

	// If we don't have a sync peer and we are not current we should start a sync
	if ws.syncPeer == nil && !ws.Current() {
		ws.startSync(nil)
	}
}

// isSyncCandidate returns whether or not the peer is a candidate to consider
// syncing from.
func (ws *WireService) isSyncCandidate(peer *peerpkg.Peer) bool {
	// Typically a peer is not a candidate for sync if it's not a full node,
	// however regression test is special in that the regression tool is
	// not a full node and still needs to be considered a sync candidate.
	if ws.params == &chaincfg.RegressionNetParams {
		// The peer is not a candidate if it's not coming from localhost
		// or the hostname can't be determined for some reason.
		host, _, err := net.SplitHostPort(peer.Addr())
		if err != nil {
			return false
		}

		if host != "127.0.0.1" && host != "localhost" {
			return false
		}
	} else {
		// The peer is not a candidate for sync if it's not a full node
		nodeServices := peer.Services()
		if nodeServices&wire.SFNodeNetwork != wire.SFNodeNetwork {
			return false
		}
	}

	// Candidate if all checks passed.
	return true
}

func (ws *WireService) startSync(syncPeer *peerpkg.Peer) {
	// Wait for a minimum number of peers to connect. This makes sure we have a good
	// selection to choose from before starting the sync.
	if len(ws.peerStates) < ws.minPeersForSync {
		return
	}
	ws.Rebroadcast()
	bestBlock, err := ws.chain.BestBlock()
	if err != nil {
		log.Error(err)
		return
	}
	var bestPeer *peerpkg.Peer
	if syncPeer == nil {
		var bestPeerHeight int32
		for peer, state := range ws.peerStates {
			if !state.syncCandidate {
				continue
			}

			// Remove sync candidate peers that are no longer candidates due
			// to passing their latest known block.  NOTE: The < is
			// intentional as opposed to <=.  While technically the peer
			// doesn't have a later block when it's equal, it will likely
			// have one soon so it is a reasonable choice.  It also allows
			// the case where both are at 0 such as during regression test.
			if peer.LastBlock() < int32(bestBlock.height) {
				state.syncCandidate = false
				continue
			}

			// Select peer which is reporting the greatest height
			if peer.LastBlock() > bestPeerHeight {
				bestPeer = peer
				bestPeerHeight = peer.LastBlock()
			}
		}
	} else {
		bestPeer = syncPeer
	}

	// Start syncing this bitch
	if bestPeer != nil {
		// TODO: use checkpoints here
		ws.syncPeer = bestPeer

		// Clear the requestedBlocks if the sync peer changes, otherwise
		// we may ignore blocks we need that the last sync peer failed
		// to send.
		ws.requestedBlocks = make(map[chainhash.Hash]struct{})

		locator := ws.chain.GetBlockLocator()

		// If the best header we have was created before this wallet then we can sync just headers
		// up to the wallet creation date since we know there wont be any transactions in those
		// blocks we're interested in. However, if we're past the wallet creation date we need to
		// start downloading merkle blocks so we learn of the wallet's transactions. We'll use a
		// buffer of one week to make sure we don't miss anything.
		log.Infof("Starting chain download from %s", bestPeer)
		if bestBlock.header.Timestamp.Before(ws.walletCreationDate.Add(-time.Hour * 24 * 7)) {
			bestPeer.PushGetHeadersMsg(locator, &ws.zeroHash)
		} else {
			bestPeer.PushGetBlocksMsg(locator, &ws.zeroHash)
		}
	} else {
		log.Warning("No sync candidates available")
	}
}

func (ws *WireService) Current() bool {
	best, err := ws.chain.BestBlock()
	if err != nil {
		return false
	}

	// If our best header's timestamp was more than 24 hours ago, we're probably not current
	if best.header.Timestamp.Before(time.Now().Add(-24 * time.Hour)) {
		return false
	}

	// Check our other peers to see if any are reporting a greater height than we have
	for peer := range ws.peerStates {
		if int32(best.height) < peer.LastBlock() {
			return false
		}
	}
	return true
}

func (ws *WireService) handleDonePeerMsg(peer *peerpkg.Peer) {
	state, exists := ws.peerStates[peer]
	if !exists {
		return
	}

	// Remove the peer from the list of candidate peers.
	delete(ws.peerStates, peer)

	// Remove requested transactions from the global map so that they will
	// be fetched from elsewhere next time we get an inv.
	for txHash := range state.requestedTxns {
		delete(ws.requestedTxns, txHash)
	}

	// Remove requested blocks from the global map so that they will be
	// fetched from elsewhere next time we get an inv.
	// TODO: we could possibly here check which peers have these blocks
	// and request them now to speed things up a little.
	for blockHash := range state.requestedBlocks {
		delete(ws.requestedBlocks, blockHash)
	}

	// Attempt to find a new peer to sync from if the quitting peer is the
	// sync peer.
	if ws.syncPeer == peer && !ws.Current() {
		log.Info("Sync peer disconnected")
		ws.syncPeer = nil
		ws.startSync(nil)
	}
}

// handleHeadersMsg handles block header messages from all peers.  Headers are
// requested when performing a headers-first sync.
func (ws *WireService) handleHeadersMsg(hmsg *headersMsg) {
	peer := hmsg.peer
	if peer != ws.syncPeer {
		log.Warning("Received header message from a peer that isn't our sync peer")
		peer.Disconnect()
		return
	}
	_, exists := ws.peerStates[peer]
	if !exists {
		log.Warningf("Received headers message from unknown peer %s", peer)
		peer.Disconnect()
		return
	}

	msg := hmsg.headers
	numHeaders := len(msg.Headers)

	// Nothing to do for an empty headers message
	if numHeaders == 0 {
		return
	}

	// Process each header we received. Make sure when check that each one is before our
	// wallet creation date (minus the buffer). If we pass the creation date we will exit
	// request merkle blocks from this point forward and exit the function.
	badHeaders := 0
	for _, blockHeader := range msg.Headers {
		if blockHeader.Timestamp.Before(ws.walletCreationDate.Add(-time.Hour * 24 * 7)) {
			_, _, height, err := ws.chain.CommitHeader(*blockHeader)
			if err != nil {
				badHeaders++
				log.Errorf("Commit header error: %s", err.Error())
			}
			log.Infof("Received header %s at height %d", blockHeader.BlockHash().String(), height)
		} else {
			log.Info("Switching to downloading merkle blocks")
			locator := ws.chain.GetBlockLocator()
			peer.PushGetBlocksMsg(locator, &ws.zeroHash)
			return
		}
	}
	// Usually the peer will send the header at the tip of the chain in each batch. This will trigger
	// one commit error so we'll consider that acceptable, but anything more than that suggests misbehavior
	// so we'll dump this peer.
	if badHeaders > 1 {
		log.Warningf("Disconnecting from peer %s because he sent us too many bad headers", peer)
		peer.Disconnect()
		return
	}

	// Request the next batch of headers
	locator := ws.chain.GetBlockLocator()
	err := peer.PushGetHeadersMsg(locator, &ws.zeroHash)
	if err != nil {
		log.Warningf("Failed to send getheaders message to peer %s: %v", peer.Addr(), err)
		return
	}
}

// handleMerkleBlockMsg handles merkle block messages from all peers.  Merkle blocks are
// requested in response to inv packets both during initial sync and after.
func (ws *WireService) handleMerkleBlockMsg(bmsg *merkleBlockMsg) {
	peer := bmsg.peer

	// We don't need to process blocks when we're syncing. They wont connect anyway
	if peer != ws.syncPeer && !ws.Current() {
		log.Warningf("Received block from %s when we aren't current", peer)
		return
	}
	state, exists := ws.peerStates[peer]
	if !exists {
		log.Warningf("Received merkle block message from unknown peer %s", peer)
		peer.Disconnect()
		return
	}

	// If we didn't ask for this block then the peer is misbehaving.
	merkleBlock := bmsg.merkleBlock
	header := merkleBlock.Header
	blockHash := header.BlockHash()
	if _, exists = state.requestedBlocks[blockHash]; !exists {
		// The regression test intentionally sends some blocks twice
		// to test duplicate block insertion fails.  Don't disconnect
		// the peer or ignore the block when we're in regression test
		// mode in this case so the chain code is actually fed the
		// duplicate blocks.
		if ws.params != &chaincfg.RegressionNetParams {
			log.Warningf("Got unrequested block %v from %s -- "+
				"disconnecting", blockHash, peer.Addr())
			peer.Disconnect()
			return
		}
	}

	// Remove block from request maps. Either chain will know about it and
	// so we shouldn't have any more instances of trying to fetch it, or we
	// will fail the insert and thus we'll retry next time we get an inv.
	delete(state.requestedBlocks, blockHash)
	delete(ws.requestedBlocks, blockHash)

	txids, err := checkMBlock(merkleBlock)
	if err != nil {
		log.Warningf("Peer %s sent an invalid MerkleBlock", peer)
		peer.Disconnect()
		return
	}

	newBlock, reorg, newHeight, err := ws.chain.CommitHeader(header)
	// If this is an orphan block which doesn't connect to the chain, it's possible
	// that we might be synced on the longest chain, but not the most-work chain like
	// we should be. To make sure this isn't the case, let's sync from the peer who
	// sent us this orphan block.
	if err == OrphanHeaderError && ws.Current() {
		log.Notice("Received orphan header, checking peer for more blocks")
		state.requestQueue = []*wire.InvVect{}
		state.requestedBlocks = make(map[chainhash.Hash]struct{})
		ws.requestedBlocks = make(map[chainhash.Hash]struct{})
		ws.startSync(peer)
		return
	} else if err == OrphanHeaderError && !ws.Current() {
		// The sync peer sent us an orphan header in the middle of a sync. This could
		// just be the last block in the batch which represents the tip of the chain.
		// In either case let's adjust the score for this peer downwards. If it goes
		// negative it means he's slamming us with blocks that don't fit in our chain
		// so disconnect.
		state.blockScore--
		if state.blockScore < 0 {
			log.Warningf("Disconnecting from peer %s because he sent us too many bad blocks", peer)
			peer.Disconnect()
			return
		}

	} else if err != nil {
		log.Error(err)
		return
	}
	state.blockScore++

	peer.UpdateLastBlockHeight(int32(newHeight))

	// Request the transactions in this block
	for _, txid := range txids {
		ws.requestedTxns[*txid] = heightAndTime{newHeight, header.Timestamp}
		limitMap(ws.requestedTxns, maxRequestedTxns)
		state.requestedTxns[*txid] = heightAndTime{newHeight, header.Timestamp}
	}

	// We can exit here if the block is already known
	if !newBlock {
		return
	}

	log.Infof("Received merkle block %s at height %d", blockHash.String(), newHeight)

	// Check reorg
	if reorg != nil && ws.Current() {
		// Rollback the appropriate transactions in our database
		err := ws.txStore.processReorg(reorg.height)
		if err != nil {
			log.Error(err)
		}
		// Set the reorg block as current best block in the header db
		// This will cause a new chain sync from the reorg point
		err = ws.chain.db.Put(*reorg, true)
		if err != nil {
			log.Error(err)
		}

		// Clear request state for new sync
		state.requestQueue = []*wire.InvVect{}
		state.requestedBlocks = make(map[chainhash.Hash]struct{})
		ws.requestedBlocks = make(map[chainhash.Hash]struct{})
	}

	// Clear mempool
	ws.mempool = make(map[chainhash.Hash]struct{})

	// If we're not current and we've downloaded everything we've requested send another getblocks message.
	// Otherwise we'll request the next block in the queue.
	if !ws.Current() && len(state.requestQueue) == 0 {
		locator := ws.chain.GetBlockLocator()
		peer.PushGetBlocksMsg(locator, &ws.zeroHash)
	} else if !ws.Current() && len(state.requestQueue) > 0 {
		iv := state.requestQueue[0]
		iv.Type = wire.InvTypeFilteredBlock
		state.requestQueue = state.requestQueue[1:]
		state.requestedBlocks[iv.Hash] = struct{}{}
		gdmsg2 := wire.NewMsgGetData()
		gdmsg2.AddInvVect(iv)
		peer.QueueMessage(gdmsg2, nil)
	}
}

// handleInvMsg handles inv messages from all peers.
// We examine the inventory advertised by the remote peer and act accordingly.
func (ws *WireService) handleInvMsg(imsg *invMsg) {
	peer := imsg.peer
	state, exists := ws.peerStates[peer]
	if !exists {
		log.Warningf("Received inv message from unknown peer %s", peer)
		return
	}

	// Attempt to find the final block in the inventory list.  There may
	// not be one.
	lastBlock := -1
	invVects := imsg.inv.InvList
	for i := len(invVects) - 1; i >= 0; i-- {
		if invVects[i].Type == wire.InvTypeBlock {
			lastBlock = i
			break
		}
	}

	// If this inv contains a block announcement, and this isn't coming from
	// our current sync peer or we're current, then update the last
	// announced block for this peer. We'll use this information later to
	// update the heights of peers based on blocks we've accepted that they
	// previously announced.
	if lastBlock != -1 && (peer != ws.syncPeer || ws.Current()) {
		peer.UpdateLastAnnouncedBlock(&invVects[lastBlock].Hash)
	}

	// Ignore invs from peers that aren't the sync if we are not current.
	// Helps prevent fetching a mass of orphans.
	if peer != ws.syncPeer && !ws.Current() {
		return
	}

	// If our chain is current and a peer announces a block we already
	// know of, then update their current block height.
	if lastBlock != -1 && ws.Current() {
		sh, err := ws.chain.GetHeader(&invVects[lastBlock].Hash)
		if err == nil {
			peer.UpdateLastBlockHeight(int32(sh.height))
		}
	}

	// Request the advertised inventory if we don't already have it
	gdmsg := wire.NewMsgGetData()
	numRequested := 0
	for _, iv := range invVects {

		// Add the inventory to the cache of known inventory
		// for the peer.
		peer.AddKnownInventory(iv)

		// Request the inventory if we don't already have it.
		haveInv, err := ws.haveInventory(iv)
		if err != nil {
			log.Warningf("Unexpected failure when checking for "+
				"existing inventory during inv message "+
				"processing: %v", err)
			continue
		}

		switch iv.Type {
		case wire.InvTypeFilteredBlock:
			fallthrough
		case wire.InvTypeBlock:
			// Block inventory goes into a request queue to be downloaded
			// one at a time. Sadly we can't batch these because the remote
			// peer  will not update the bloom filter until he's done processing
			// the batch which means we will have a super high false positive rate.
			if _, exists := ws.requestedBlocks[iv.Hash]; (!ws.Current() && !exists && !haveInv) || ws.Current() {
				iv.Type = wire.InvTypeFilteredBlock
				state.requestQueue = append(state.requestQueue, iv)
			}
		case wire.InvTypeTx:
			// Transaction inventory can be requested in batches
			if _, exists := ws.requestedTxns[iv.Hash]; !exists && numRequested < wire.MaxInvPerMsg && !haveInv {
				ws.requestedTxns[iv.Hash] = heightAndTime{0, time.Now()} // unconfirmed tx
				limitMap(ws.requestedTxns, maxRequestedTxns)
				state.requestedTxns[iv.Hash] = heightAndTime{0, time.Now()}

				gdmsg.AddInvVect(iv)
				numRequested++
			}
		default:
			continue
		}
	}

	// Pop the first block off the queue and request it
	if len(state.requestQueue) > 0 {
		iv := state.requestQueue[0]
		gdmsg.AddInvVect(iv)
		if len(state.requestQueue) > 1 {
			state.requestQueue = state.requestQueue[1:]
		} else {
			state.requestQueue = []*wire.InvVect{}
		}
		state.requestedBlocks[iv.Hash] = struct{}{}
	}
	if len(gdmsg.InvList) > 0 {
		peer.QueueMessage(gdmsg, nil)
	}
}

func (ws *WireService) handleTxMsg(tmsg *txMsg) {
	tx := tmsg.tx
	peer := tmsg.peer
	txHash := tx.TxHash()

	state, exists := ws.peerStates[peer]
	if !exists {
		log.Warningf("Received tx message from unknown peer %s", peer)
		return
	}
	ht, ok := state.requestedTxns[tx.TxHash()]
	if !ok {
		log.Warningf("Peer %s is sending us transactions we didn't request", peer)
		peer.Disconnect()
		return
	}
	ws.mempool[txHash] = struct{}{}
	hits, err := ws.txStore.Ingest(tx, int32(ht.height), ht.timestamp)
	if err != nil {
		log.Errorf("Error ingesting tx: %s\n", err.Error())
	}

	// Remove transaction from request maps. Either the mempool/chain
	// already knows about it and as such we shouldn't have any more
	// instances of trying to fetch it, or we failed to insert and thus
	// we'll retry next time we get an inv.
	delete(state.requestedTxns, txHash)
	delete(ws.requestedTxns, txHash)

	// If this transaction had no hits, update the peer's false positive counter
	if hits == 0 {
		log.Debugf("Tx %s from Peer%d had no hits, filter false positive.", txHash.String(), peer.ID())
		state.falsePositives++
	} else {
		log.Noticef("Ingested new tx %s at height %d", txHash.String(), ht.height)
	}

	// Check to see if false positives exceeds the maximum allowed. If so, reset and resend the filter.
	if state.falsePositives > maxFalsePositives {
		state.falsePositives = 0
		ws.updateFilterAndSend(peer)
	}
}

func (ws *WireService) Rebroadcast() {
	// get all unconfirmed txs
	invMsg, err := ws.txStore.GetPendingInv()
	if err != nil {
		log.Errorf("Rebroadcast error: %s", err.Error())
	}
	// Nothing to broadcast, so don't
	if len(invMsg.InvList) == 0 {
		return
	}
	for peer := range ws.peerStates {
		peer.QueueMessage(invMsg, nil)
	}
}

func (ws *WireService) updateFilterAndSend(peer *peerpkg.Peer) {
	if ws.txStore != nil {
		filter, err := ws.txStore.GimmeFilter()
		if err == nil {
			peer.QueueMessage(filter.MsgFilterLoad(), nil)
		} else {
			log.Errorf("Error loading bloom filter: %s", err.Error())
		}
	}
}

// handleUpdateFiltersMsg sends a filter update message to all peers
func (ws *WireService) handleUpdateFiltersMsg() {
	for peer := range ws.peerStates {
		ws.updateFilterAndSend(peer)
	}
}

// haveInventory returns whether or not the inventory represented by the passed
// inventory vector is known.  This includes checking all of the various places
// inventory can be when it is in different states such as blocks that are part
// of the main chain, on a side chain, in the orphan pool, and transactions that
// are in the memory pool (either the main pool or orphan pool).
func (ws *WireService) haveInventory(invVect *wire.InvVect) (bool, error) {
	switch invVect.Type {
	case wire.InvTypeWitnessBlock:
		fallthrough
	case wire.InvTypeBlock:
		// Ask chain if the block is known to it in any form (main
		// chain, side chain, or orphan).
		_, err := ws.chain.GetHeader(&invVect.Hash)
		if err != nil {
			return false, nil
		}
		return true, nil
	case wire.InvTypeTx:
		// Is transaction already in mempool
		if _, ok := ws.mempool[invVect.Hash]; ok {
			return true, nil
		}
		return false, nil
	}
	// The requested inventory is is an unsupported type, so just claim
	// it is known to avoid requesting it.
	return true, nil
}

// limitMap is a helper function for maps that require a maximum limit by
// evicting a random transaction if adding a new value would cause it to
// overflow the maximum allowed.
func limitMap(i interface{}, limit int) {
	m, ok := i.(map[chainhash.Hash]struct{})
	if ok {
		if len(m)+1 > limit {
			// Remove a random entry from the map.  For most compilers, Go's
			// range statement iterates starting at a random item although
			// that is not 100% guaranteed by the spec.  The iteration order
			// is not important here because an adversary would have to be
			// able to pull off preimage attacks on the hashing function in
			// order to target eviction of specific entries anyways.
			for txHash := range m {
				delete(m, txHash)
				return
			}
		}
		return
	}
	n, ok := i.(map[chainhash.Hash]uint32)
	if ok {
		if len(n)+1 > limit {
			for txHash := range n {
				delete(n, txHash)
				return
			}
		}
	}
}
