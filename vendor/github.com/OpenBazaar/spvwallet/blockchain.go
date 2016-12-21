package spvwallet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"math/big"
	"path"
	"sort"
	"sync"
	"time"
)

type Headers interface {
	// Put a block header to the database
	// Total work and height are required to be calculated prior to insertion
	// If this is the new best header, the chain tip should also be updated
	Put(header StoredHeader, newBestHeader bool) error

	// Delete all headers after the MAX_HEADERS most recent
	Prune() error

	// Returns all information about the previous header
	GetPreviousHeader(header wire.BlockHeader) (StoredHeader, error)

	// Retreive the best header from the database
	GetBestHeader() (StoredHeader, error)

	// Get the height of chain
	Height() (uint32, error)

	// Cleanly close the db
	Close()

	// Print all headers
	Print()
}

type HeaderDB struct {
	lock     *sync.Mutex
	db       *bolt.DB
	filePath string
}

var (
	BKTHeaders  = []byte("Headers")
	BKTChainTip = []byte("ChainTip")
	KEYChainTip = []byte("ChainTip")
)

func NewHeaderDB(filePath string) *HeaderDB {
	h := new(HeaderDB)
	db, _ := bolt.Open(path.Join(filePath, "headers.bin"), 0644, &bolt.Options{InitialMmapSize: 5000000})
	h.db = db
	h.lock = new(sync.Mutex)
	h.filePath = filePath

	db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists(BKTHeaders)
		if err != nil {
			return err
		}
		_, err = btx.CreateBucketIfNotExists(BKTChainTip)
		if err != nil {
			return err
		}
		return nil
	})
	return h
}

func (h *HeaderDB) Put(sh StoredHeader, newBestHeader bool) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	return h.db.Update(func(btx *bolt.Tx) error {
		hdrs := btx.Bucket(BKTHeaders)
		ser, err := serializeHeader(sh)
		if err != nil {
			return err
		}
		hash := sh.header.BlockHash()
		err = hdrs.Put(hash.CloneBytes(), ser)
		if err != nil {
			return err
		}
		if newBestHeader {
			tip := btx.Bucket(BKTChainTip)
			if err != nil {
				return err
			}
			err = tip.Put(KEYChainTip, ser)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (h *HeaderDB) Prune() error {
	h.lock.Lock()
	defer h.lock.Unlock()
	return h.db.Update(func(btx *bolt.Tx) error {
		hdrs := btx.Bucket(BKTHeaders)
		numHeaders := hdrs.Stats().KeyN
		if numHeaders > MAX_HEADERS {
			for i := 0; i < numHeaders-MAX_HEADERS; i++ {
				k, _ := hdrs.Cursor().First()
				err := hdrs.Delete(k)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (h *HeaderDB) GetPreviousHeader(header wire.BlockHeader) (sh StoredHeader, err error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	err = h.db.View(func(btx *bolt.Tx) error {
		hdrs := btx.Bucket(BKTHeaders)
		hash := header.PrevBlock
		b := hdrs.Get(hash.CloneBytes())
		if b == nil {
			return errors.New("Header does not exist in database")
		}
		sh, err = deserializeHeader(b)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return sh, err
	}
	return sh, nil
}

func (h *HeaderDB) GetBestHeader() (sh StoredHeader, err error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	err = h.db.View(func(btx *bolt.Tx) error {
		tip := btx.Bucket(BKTChainTip)
		b := tip.Get(KEYChainTip)
		if b == nil {
			return errors.New("ChainTip not set")
		}
		sh, err = deserializeHeader(b)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return sh, err
	}
	return sh, nil
}

func (h *HeaderDB) Height() (uint32, error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	var height uint32
	err := h.db.View(func(btx *bolt.Tx) error {
		tip := btx.Bucket(BKTChainTip)
		sh, err := deserializeHeader(tip.Get(KEYChainTip))
		if err != nil {
			return err
		}
		height = sh.height
		return nil
	})
	if err != nil {
		return height, err
	}
	return height, nil
}

func (h *HeaderDB) Print() {
	h.lock.Lock()
	defer h.lock.Unlock()
	m := make(map[uint32]string)
	h.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		bkt := tx.Bucket(BKTHeaders)
		bkt.ForEach(func(k, v []byte) error {
			sh, _ := deserializeHeader(v)
			m[sh.height] = sh.header.BlockHash().String()
			return nil
		})

		return nil
	})
	var keys []int
	for k := range m {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Println(k, m[uint32(k)])
	}
}

func (h *HeaderDB) Close() {
	h.lock.Lock()
}

const (
	MAX_HEADERS                = 2000
	MAINNET_CHECKPOINT_HEIGHT  = 407232
	TESTNET3_CHECKPOINT_HEIGHT = 1058400
	REGTEST_CHECKPOINT_HEIGHT  = 0
)

var MainnetCheckpoint wire.BlockHeader
var Testnet3Checkpoint wire.BlockHeader
var RegtestCheckpoint wire.BlockHeader

type Blockchain struct {
	lock   *sync.Mutex
	params *chaincfg.Params
	db     Headers
}

type StoredHeader struct {
	header    wire.BlockHeader
	height    uint32
	totalWork *big.Int
}

func NewBlockchain(filePath string, params *chaincfg.Params) *Blockchain {
	b := &Blockchain{
		lock:   new(sync.Mutex),
		params: params,
		db:     NewHeaderDB(filePath),
	}

	h, err := b.db.Height()
	if h == 0 || err != nil {
		log.Info("Initializing headers db with checkpoints")
		createCheckpoints()
		if b.params.Name == chaincfg.MainNetParams.Name {
			// Put the checkpoint to the db
			sh := StoredHeader{
				header:    MainnetCheckpoint,
				height:    MAINNET_CHECKPOINT_HEIGHT,
				totalWork: big.NewInt(0),
			}
			b.db.Put(sh, true)
		} else if b.params.Name == chaincfg.TestNet3Params.Name {
			// Put the checkpoint to the db
			sh := StoredHeader{
				header:    Testnet3Checkpoint,
				height:    TESTNET3_CHECKPOINT_HEIGHT,
				totalWork: big.NewInt(0),
			}
			// Put to db
			b.db.Put(sh, true)
		} else if b.params.Name == chaincfg.RegressionNetParams.Name {
			// Put the checkpoint to the db
			sh := StoredHeader{
				header:    RegtestCheckpoint,
				height:    REGTEST_CHECKPOINT_HEIGHT,
				totalWork: big.NewInt(0),
			}
			// Put to db
			b.db.Put(sh, true)
		}
	}
	return b
}

func (b *Blockchain) CommitHeader(header wire.BlockHeader) (bool, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	newTip := false
	// Fetch our current best header from the db
	bestHeader, err := b.db.GetBestHeader()
	if err != nil {
		return false, err
	}
	tipHash := bestHeader.header.BlockHash()
	var parentHeader StoredHeader

	// If the tip is also the parent of this header, then we can save a database read by skipping
	// the lookup of the parent header. Otherwise (ophan?) we need to fetch the parent.
	if header.PrevBlock.IsEqual(&tipHash) {
		parentHeader = bestHeader
	} else {
		parentHeader, err = b.db.GetPreviousHeader(header)
		if err != nil {
			log.Error(header.PrevBlock.String())
			return false, errors.New("Header does not extend any known headers")
		}
	}
	valid := b.CheckHeader(header, parentHeader)
	if !valid {
		return false, nil
	}
	// If this block is already the tip, return
	headerHash := header.BlockHash()
	if tipHash.IsEqual(&headerHash) {
		return newTip, nil
	}
	// Add the work of this header to the total work stored at the previous header
	cumulativeWork := new(big.Int).Add(parentHeader.totalWork, blockchain.CalcWork(header.Bits))

	// If the cumulative work is greater than the total work of our best header
	// then we have a new best header. Update the chain tip and check for a reorg.
	if cumulativeWork.Cmp(bestHeader.totalWork) == 1 {
		newTip = true
		prevHash := parentHeader.header.BlockHash()
		// If this header is not extending the previous best header then we have a reorg.
		if !tipHash.IsEqual(&prevHash) {
			log.Warning("REORG!!! REORG!!! REORG!!!")
		}
	}
	// Put the header to the database
	err = b.db.Put(StoredHeader{
		header:    header,
		height:    parentHeader.height + 1,
		totalWork: cumulativeWork,
	}, newTip)
	if err != nil {
		return newTip, err
	}
	// Prune any excess headers
	/*err = b.Prune()
	if err != nil {
		return newTip, err
	}*/
	return newTip, nil
}

func (b *Blockchain) CheckHeader(header wire.BlockHeader, prevHeader StoredHeader) bool {

	// get hash of n-1 header
	prevHash := prevHeader.header.BlockHash()
	height := prevHeader.height

	// check if headers link together.  That whole 'blockchain' thing.
	if prevHash.IsEqual(&header.PrevBlock) == false {
		log.Errorf("Headers %d and %d don't link.\n", height, height+1)
		return false
	}

	// check the header meets the difficulty requirement
	diffTarget, err := b.calcRequiredWork(header, int32(height+1), prevHeader)
	if err != nil {
		log.Errorf("Error calclating difficulty", err)
		return false
	}
	if header.Bits != diffTarget {
		log.Warningf("Block %d %s incorrect difficuly.  Read %d, expect %d\n",
			height+1, header.BlockHash().String(), header.Bits, diffTarget)
		return false
	}

	// check if there's a valid proof of work.  That whole "Bitcoin" thing.
	if !checkProofOfWork(header, b.params) {
		log.Debugf("Block %d Bad proof of work.\n", height)
		return false
	}

	// TODO: Check header timestamps: from Core
	/*
		 // Check timestamp against prev
		 if (block.GetBlockTime() <= pindexPrev->GetMedianTimePast())
	        	return state.Invalid(false, REJECT_INVALID, "time-too-old", "block's timestamp is too early");

		 // Check timestamp
		 if (block.GetBlockTime() > nAdjustedTime + 2 * 60 * 60)
	        	return state.Invalid(false, REJECT_INVALID, "time-too-new", "block timestamp too far in the future");
	*/

	return true // it must have worked if there's no errors and got to the end.
}

// Get the PoW target this block should meet. We may need to handle a difficlty adjustment
// or testnet difficulty rules.
func (b *Blockchain) calcRequiredWork(header wire.BlockHeader, height int32, prevHeader StoredHeader) (uint32, error) {
	// If this is not a difficulty adjustment period
	if height%epochLength != 0 {
		// If we are on testnet
		if b.params.ReduceMinDifficulty {
			// If it's been more than 20 minutes since the last header return the minimum difficulty
			if header.Timestamp.After(prevHeader.header.Timestamp.Add(targetSpacing * 2)) {
				return b.params.PowLimitBits, nil
			} else { // Otherwise return the difficulty of the last block not using special difficulty rules
				for {
					var err error = nil
					for err == nil && int32(prevHeader.height)%epochLength != 0 && prevHeader.header.Bits == b.params.PowLimitBits {
						var sh StoredHeader
						sh, err = b.db.GetPreviousHeader(prevHeader.header)
						// Error should only be non-nil if prevHeader is the checkpoint.
						// In that case we should just return checkpoint bits
						if err == nil {
							prevHeader = sh
						}

					}
					return prevHeader.header.Bits, nil
				}
			}
		}
		// Just retrn the bits from the last header
		return prevHeader.header.Bits, nil
	}
	// We are on a difficulty adjustment period so we need to correctly calculate the new difficulty.
	epoch, err := b.GetEpoch()
	if err != nil {
		log.Error(err)
		return 0, err
	}
	return calcDiffAdjust(*epoch, prevHeader.header, b.params), nil
}

func (b *Blockchain) GetEpoch() (*wire.BlockHeader, error) {
	sh, err := b.db.GetBestHeader()
	if err != nil {
		return &sh.header, err
	}
	for i := 0; i < 2015; i++ {
		sh, err = b.db.GetPreviousHeader(sh.header)
		if err != nil {
			return &sh.header, err
		}
	}
	log.Debug("Epoch", sh.header.BlockHash().String())
	return &sh.header, nil
}

func (b *Blockchain) GetNPrevBlockHashes(n int) []*chainhash.Hash {
	var ret []*chainhash.Hash
	hdr, err := b.db.GetBestHeader()
	if err != nil {
		return ret
	}
	tipSha := hdr.header.BlockHash()
	ret = append(ret, &tipSha)
	for i := 0; i < n-1; i++ {
		hdr, err = b.db.GetPreviousHeader(hdr.header)
		if err != nil {
			return ret
		}
		shaHash := hdr.header.BlockHash()
		ret = append(ret, &shaHash)
	}
	return ret
}

func (b *Blockchain) GetBlockLocatorHashes() []*chainhash.Hash {
	var ret []*chainhash.Hash
	parent, err := b.db.GetBestHeader()
	if err != nil {
		return ret
	}

	rollback := func(parent StoredHeader, n int) (StoredHeader, error) {
		for i := 0; i < n; i++ {
			parent, err = b.db.GetPreviousHeader(parent.header)
			if err != nil {
				return parent, err
			}
		}
		return parent, nil
	}

	step := 1
	start := 0
	for {
		if start >= 10 {
			step *= 2
			start = 0
		}
		hash := parent.header.BlockHash()
		ret = append(ret, &hash)
		if len(ret) == 500 {
			break
		}
		parent, err = rollback(parent, step)
		if err != nil {
			break
		}
		start += 1
	}
	return ret
}

func (b *Blockchain) Close() {
	b.lock.Lock()
	b.db.Close()
}

/*----- header serialization ------- */
/* byte length   desc            at offset
   80	 header	             0
    4	 height             80
   32	 total work         84
*/
func serializeHeader(sh StoredHeader) ([]byte, error) {
	var buf bytes.Buffer
	err := sh.header.Serialize(&buf)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&buf, binary.BigEndian, sh.height)
	if err != nil {
		return nil, err
	}
	biBytes := sh.totalWork.Bytes()
	pad := make([]byte, 32-len(biBytes))
	serializedBI := append(pad, biBytes...)
	buf.Write(serializedBI)
	return buf.Bytes(), nil
}

func deserializeHeader(b []byte) (sh StoredHeader, err error) {
	r := bytes.NewReader(b)
	hdr := new(wire.BlockHeader)
	err = hdr.Deserialize(r)
	if err != nil {
		return sh, err
	}
	var height uint32
	err = binary.Read(r, binary.BigEndian, &height)
	if err != nil {
		return sh, err
	}
	biBytes := make([]byte, 32)
	_, err = r.Read(biBytes)
	if err != nil {
		return sh, err
	}
	bi := new(big.Int)
	bi.SetBytes(biBytes)
	sh = StoredHeader{
		header:    *hdr,
		height:    height,
		totalWork: bi,
	}
	return sh, nil
}

func createCheckpoints() {
	mainnetPrev, _ := chainhash.NewHashFromStr("0000000000000000045645e2acd740a88d2b3a09369e9f0f80d5376e4b6c5189")
	mainnetMerk, _ := chainhash.NewHashFromStr("e4b259941d8a8d5f1c5f18a68366ef570c0a7876c1f22a54a4f143215e3f4d9b")
	MainnetCheckpoint = wire.BlockHeader{
		Version:    4,
		PrevBlock:  *mainnetPrev,
		MerkleRoot: *mainnetMerk,
		Timestamp:  time.Unix(1460622341, 0),
		Bits:       403056459,
		Nonce:      3800536668,
	}

	testnet3Prev, _ := chainhash.NewHashFromStr("00000000000008471ccf356a18dd48aa12506ef0b6162cb8f98a8d8bb0465902")
	testnet3Merk, _ := chainhash.NewHashFromStr("a2bd975d9ac68eb1a7bc00df593c55a64e81ac0c9b8f535bb06b390d3010816f")
	Testnet3Checkpoint = wire.BlockHeader{
		Version:    536870912,
		PrevBlock:  *testnet3Prev,
		MerkleRoot: *testnet3Merk,
		Timestamp:  time.Unix(1481479754, 0),
		Bits:       436861323,
		Nonce:      3058617296,
	}
	RegtestCheckpoint = chaincfg.RegressionNetParams.GenesisBlock.Header
}
