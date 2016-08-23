package spvwallet

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"net"
	"sync"
)

const (
	VERSION = 70012
)

type Peer struct {
	con net.Conn // the (probably tcp) connection to the node

	downloadPeer bool

	blockchain *Blockchain

	remoteAddress string

	disconnectChan chan string

	//[doesn't work without fancy mutexes, nevermind, just use header file]
	// localHeight   int32  // block height we're on
	remoteHeight  int32  // block height they're on
	localVersion  uint32 // version we report
	remoteVersion uint32 // version remote node

	// what's the point of the input queue? remove? leave for now...
	inMsgQueue  chan wire.Message // Messages coming in from remote node
	outMsgQueue chan wire.Message // Messages going out to remote node

	WBytes uint64 // total bytes written
	RBytes uint64 // total bytes read

	TS *TxStore // transaction store to write to

	// mBlockQueue is for keeping track of what height we've requested.
	blockQueue chan HashAndHeight

	// fPositives is a channel to keep track of bloom filter false positives.
	fPositives chan int32

	// State of the connection with this peer
	connectionState ConnectionState

	// The user agent our peer sees
	userAgent string

	// known good txids and their heights
	OKTxids map[chainhash.Hash]int32
	OKMutex sync.Mutex
}

// AskForTx requests a tx we heard about from an inv message.
// It's one at a time but should be fast enough.
// I don't like this function because SPV shouldn't even ask...
func (p *Peer) AskForTx(txid chainhash.Hash) {
	gdata := wire.NewMsgGetData()
	inv := wire.NewInvVect(wire.InvTypeTx, &txid)
	gdata.AddInvVect(inv)
	p.outMsgQueue <- gdata
}

// HashAndHeight is needed instead of just height in case a fullnode
// responds abnormally (?) by sending out of order merkleblocks.
// we cache a merkleroot:height pair in the queue so we don't have to
// look them up from the disk.
// Also used when inv messages indicate blocks so we can add the header
// and parse the txs in one request instead of requesting headers first.
type HashAndHeight struct {
	blockhash chainhash.Hash
	height    int32
	final     bool // indicates this is the last merkleblock requested
}

// NewRootAndHeight saves like 2 lines.
func NewRootAndHeight(b chainhash.Hash, h int32) (hah HashAndHeight) {
	hah.blockhash = b
	hah.height = h
	return
}

func (p *Peer) AskForMerkleBlock(hash chainhash.Hash) {
	m := wire.NewMsgGetData()
	m.AddInvVect(wire.NewInvVect(wire.InvTypeFilteredBlock, &hash))
	p.outMsgQueue <- m
}

func (p *Peer) IngestBlockAndHeader(m *wire.MsgMerkleBlock) {
	txids, err := checkMBlock(m) // check self-consistency
	if err != nil {
		log.Errorf("Merkle block error: %s\n", err.Error())
		return
	}

	success, err := p.blockchain.CommitHeader(m.Header)
	if err != nil {
		log.Error(err)
		return
	}
	var height uint32
	if success {
		h, err := p.blockchain.db.Height()
		height = h
		if err != nil {
			log.Error(err)
			return
		}
		p.TS.SetDBSyncHeight(int32(h))
	} else {
		bestSH, err := p.blockchain.db.GetBestHeader()
		height = bestSH.height
		if err != nil {
			log.Error(err)
			return
		}
		headerHash := m.Header.BlockHash()
		tipHash := bestSH.header.BlockHash()
		if !tipHash.IsEqual(&headerHash) {
			return
		}
	}
	p.OKMutex.Lock()
	for _, txid := range txids {
		p.OKTxids[*txid] = int32(height)
	}
	p.OKMutex.Unlock()
	log.Debugf("Received Merkle Block %s from %s", m.Header.BlockHash().String(), p.con.RemoteAddr().String())
}

func (p *Peer) IngestMerkleBlock(m *wire.MsgMerkleBlock) {
	p.OKMutex.Lock()
	defer p.OKMutex.Unlock()
	txids, err := checkMBlock(m) // check self-consistency
	if err != nil {
		log.Debugf("Merkle block error: %s\n", err.Error())
		return
	}
	var hah HashAndHeight
	select { // select here so we don't block on an unrequested mblock
	case hah = <-p.blockQueue: // pop height off mblock queue
		break
	default:
		log.Warning("Unrequested merkle block")
		return
	}

	// this verifies order, and also that the returned header fits
	// into our SPV header file
	newMerkBlockSha := m.Header.BlockHash()
	if !hah.blockhash.IsEqual(&newMerkBlockSha) {
		// This implies we may miss transactions in this block.
		log.Errorf("merkle block out of order got %s expect %s",
			m.Header.BlockHash().String(), hah.blockhash.String())
		return
	}
	for _, txid := range txids {
		p.OKTxids[*txid] = hah.height
	}
	// write to db that we've sync'd to the height indicated in the
	// merkle block.  This isn't QUITE true since we haven't actually gotten
	// the txs yet but if there are problems with the txs we should backtrack.
	err = p.TS.SetDBSyncHeight(hah.height)
	if err != nil {
		log.Errorf("Merkle block error: %s\n", err.Error())
		return
	}
	if hah.final {
		// don't set waitstate; instead, ask for headers again!
		// this way the only thing that triggers waitstate is asking for headers,
		// getting 0, calling AskForMerkBlocks(), and seeing you don't need any.
		// that way you are pretty sure you're synced up.
		err = p.AskForHeaders()
		if err != nil {
			log.Errorf("Merkle block error: %s\n", err.Error())
			return
		}
	}
	log.Debugf("Ingested Merkle Block %s at height %d", m.Header.BlockHash().String(), hah.height)
	return
}

// IngestHeaders takes in a bunch of headers and appends them to the
// local header file, checking that they fit.  If there's no headers,
// it assumes we're done and returns false.  If it worked it assumes there's
// more to request and returns true.
func (p *Peer) IngestHeaders(m *wire.MsgHeaders) (bool, error) {
	gotNum := int64(len(m.Headers))
	if gotNum > 0 {
		log.Debugf("Received %d headers from %s, validating...", gotNum, p.con.RemoteAddr().String())
	} else {
		log.Debugf("Received 0 headers from %s, we're probably synced up", p.con.RemoteAddr().String())
		if p.TS.chainState == SYNCING {
			log.Info("Headers fully synced")
		}
		return false, nil
	}
	for _, resphdr := range m.Headers {
		_, err := p.blockchain.CommitHeader(*resphdr)
		if err != nil {
			// probably should disconnect from spv node at this point,
			// since they're giving us invalid headers.
			return true, fmt.Errorf("Returned header didn't fit in chain")
		}
	}
	height, _ := p.blockchain.db.Height()
	log.Debugf("Headers to height %d OK.", height)
	return true, nil
}

func (p *Peer) AskForHeaders() error {
	ghdr := wire.NewMsgGetHeaders()
	ghdr.ProtocolVersion = p.localVersion

	ghdr.BlockLocatorHashes = p.blockchain.GetBlockLocatorHashes()

	log.Debugf("Sending getheaders message to %s\n", p.con.RemoteAddr().String())
	p.outMsgQueue <- ghdr
	return nil
}

// AskForMerkBlocks requests blocks from current to last
// right now this asks for 1 block per getData message.
// Maybe it's faster to ask for many in a each message?
func (p *Peer) AskForBlocks() error {
	headerTip, err := p.blockchain.db.Height()
	if err != nil {
		return err
	}

	dbTip, err := p.TS.GetDBSyncHeight()
	if err != nil {
		return err
	}

	log.Debugf("DatabaseTip %d HeaderTip %d\n", dbTip, headerTip)
	if uint32(dbTip) > headerTip {
		return fmt.Errorf("error- db longer than headers! shouldn't happen.")
	}

	if uint32(dbTip) == headerTip {
		// nothing to ask for; set wait state and return
		log.Debugf("No blocks to request, entering wait state\n")
		if p.TS.chainState != WAITING {
			log.Info("Blockchain fully synced")
		}
		p.TS.chainState = WAITING
		// also advertise any unconfirmed txs here
		p.Rebroadcast()
		return nil
	}

	log.Debugf("Will request blocks %d to %d\n", dbTip+1, headerTip)
	hashes := p.blockchain.GetNPrevBlockHashes(int(headerTip - uint32(dbTip)))

	// loop through all heights where we want merkleblocks.
	for i := len(hashes) - 1; i >= 0; i-- {
		dbTip++
		iv1 := wire.NewInvVect(wire.InvTypeFilteredBlock, hashes[i])
		gdataMsg := wire.NewMsgGetData()
		// add inventory
		err = gdataMsg.AddInvVect(iv1)
		if err != nil {
			return err
		}

		hah := NewRootAndHeight(*hashes[i], dbTip)
		if uint32(dbTip) == headerTip { // if this is the last block, indicate finality
			hah.final = true
		}
		// waits here most of the time for the queue to empty out
		p.blockQueue <- hah // push height and mroot of requested block on queue
		p.outMsgQueue <- gdataMsg
	}
	return nil
}
