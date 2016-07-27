package spvwallet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
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
		hash := sh.header.BlockSha()
		err = hdrs.Put(hash.Bytes(), ser)
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
		b := hdrs.Get(hash.Bytes())
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
			m[sh.height] = sh.header.BlockSha().String()
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

const (
	MAX_HEADERS                = 2000
	MAINNET_CHECKPOINT_HEIGHT  = 407232
	TESTNET3_CHECKPOINT_HEIGHT = 895104
)

var MainnetCheckpoint wire.BlockHeader
var Testnet3Checkpoint wire.BlockHeader

type Blockchain struct {
	lock   *sync.Mutex
	params *chaincfg.Params
	db     Headers
}

type StoredHeader struct {
	header     wire.BlockHeader
	height     uint32
	totalWork  *big.Int
	diffTarget uint32
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
				header:     MainnetCheckpoint,
				height:     MAINNET_CHECKPOINT_HEIGHT,
				totalWork:  big.NewInt(0),
				diffTarget: MainnetCheckpoint.Bits,
			}
			b.db.Put(sh, true)
		} else if b.params.Name == chaincfg.TestNet3Params.Name {
			// Put the checkpoint to the db
			sh := StoredHeader{
				header:     Testnet3Checkpoint,
				height:     TESTNET3_CHECKPOINT_HEIGHT,
				totalWork:  big.NewInt(0),
				diffTarget: Testnet3Checkpoint.Bits,
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
	tipHash := bestHeader.header.BlockSha()
	var parentHeader StoredHeader

	// If the tip is also the parent of this header, then we can save a database read by skipping
	// the lookup of the parent header. Otherwise (ophan?) we need to fetch the parent.
	if header.PrevBlock.IsEqual(&tipHash) {
		parentHeader = bestHeader
	} else {
		parentHeader, err = b.db.GetPreviousHeader(header)
		if err != nil {
			return false, errors.New("Header does not extend any known headers")
		}
	}
	valid, diffTarget := b.CheckHeader(header, parentHeader)
	if !valid {
		return false, nil
	}
	// If this block is already the tip, return
	headerHash := header.BlockSha()
	if tipHash.IsEqual(&headerHash) {
		return newTip, nil
	}
	// Add the work of this header to the total work stored at the previous header
	cumulativeWork := new(big.Int).Add(parentHeader.totalWork, blockchain.CalcWork(header.Bits))

	// If the cumulative work is greater than the total work of our best header
	// then we have a new best header. Update the chain tip and check for a reorg.
	if cumulativeWork.Cmp(bestHeader.totalWork) == 1 {
		newTip = true
		prevHash := parentHeader.header.BlockSha()
		// If this header is not extending the previous best header then we have a reorg.
		if !tipHash.IsEqual(&prevHash) {
			log.Warning("REORG!!! REORG!!! REORG!!!")
		}
	}
	// Put the header to the database
	err = b.db.Put(StoredHeader{
		header:     header,
		height:     parentHeader.height + 1,
		totalWork:  cumulativeWork,
		diffTarget: diffTarget,
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

func (b *Blockchain) CheckHeader(header wire.BlockHeader, prevHeader StoredHeader) (bool, uint32) {
	diffTarget := blockchain.CompactToBig(prevHeader.diffTarget)

	// get hash of n-1 header
	prevHash := prevHeader.header.BlockSha()
	height := prevHeader.height
	// check if headers link together.  That whole 'blockchain' thing.
	if prevHash.IsEqual(&header.PrevBlock) == false {
		log.Errorf("Headers %d and %d don't link.\n", height, height+1)
		return false, 0
	}
	difficultyNerfing := false
	var testnetDiff *big.Int
	// see if we're on a difficulty adjustment block
	if (int32(height)+1)%epochLength == 0 {
		// if so, check if difficulty adjustment is valid.
		// That whole "controlled supply" thing.
		// calculate diff n based on n-2016 ... n-1
		epoch, err := b.GetEpoch()
		if err != nil {
			log.Error(err)
			return false, 0
		}
		diffTarget = calcDiffAdjust(*epoch, prevHeader.header, b.params)
		log.Debugf("Update epoch at height %d", height)
	} else {
		// not a new epoch
		// if on testnet, check for difficulty nerfing
		if b.params.ResetMinDifficulty && header.Timestamp.After(
			prevHeader.header.Timestamp.Add(targetSpacing*2)) {
			//	fmt.Debugf("nerf %d ", curHeight)
			difficultyNerfing = true
			testnetCompact := b.params.PowLimitBits // difficulty 1
			testnetDiff = blockchain.CompactToBig(testnetCompact)
		}
	}
	headerBigBits := blockchain.CompactToBig(header.Bits)
	if (difficultyNerfing && headerBigBits.Cmp(testnetDiff) == 1) || (!difficultyNerfing && headerBigBits.Cmp(diffTarget) == 1) {
		log.Warningf("Block %d %s incorrect difficuly.  Read %x, expect %x\n",
			height, header.BlockSha().String(), header.Bits, diffTarget)
		return false, 0
	}

	// check if there's a valid proof of work.  That whole "Bitcoin" thing.
	if !checkProofOfWork(header, b.params) {
		log.Debugf("Block %d Bad proof of work.\n", height)
		return false, 0
	}

	return true, blockchain.BigToCompact(diffTarget) // it must have worked if there's no errors and got to the end.
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
	log.Debug("Epoch", sh.header.BlockSha().String())
	return &sh.header, nil
}

func (b *Blockchain) GetNPrevBlockHashes(n int) []*wire.ShaHash {
	var ret []*wire.ShaHash
	hdr, err := b.db.GetBestHeader()
	if err != nil {
		return ret
	}
	tipSha := hdr.header.BlockSha()
	ret = append(ret, &tipSha)
	for i := 0; i < n-1; i++ {
		hdr, err = b.db.GetPreviousHeader(hdr.header)
		if err != nil {
			return ret
		}
		shaHash := hdr.header.BlockSha()
		ret = append(ret, &shaHash)
	}
	return ret
}

func (b *Blockchain) GetBlockLocatorHashes() []*wire.ShaHash {
	var ret []*wire.ShaHash
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
		hash := parent.header.BlockSha()
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

/*----- header serialization ------- */
/* byte length   desc            at offset
   80	 header	             0
    4	 height             80
   32	 total work         84
    4         difficulty target  116
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
	err = binary.Write(&buf, binary.BigEndian, sh.diffTarget)
	if err != nil {
		return nil, err
	}
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
	var difficultyTarget uint32
	err = binary.Read(r, binary.BigEndian, &difficultyTarget)
	if err != nil {
		return sh, err
	}
	sh = StoredHeader{
		header:     *hdr,
		height:     height,
		totalWork:  bi,
		diffTarget: difficultyTarget,
	}
	return sh, nil
}

func createCheckpoints() {
	mainnetPrev, _ := wire.NewShaHashFromStr("0000000000000000045645e2acd740a88d2b3a09369e9f0f80d5376e4b6c5189")
	mainnetMerk, _ := wire.NewShaHashFromStr("e4b259941d8a8d5f1c5f18a68366ef570c0a7876c1f22a54a4f143215e3f4d9b")
	MainnetCheckpoint = wire.BlockHeader{
		Version:    4,
		PrevBlock:  *mainnetPrev,
		MerkleRoot: *mainnetMerk,
		Timestamp:  time.Unix(1460622341, 0),
		Bits:       403056459,
		Nonce:      3800536668,
	}

	testnet3Prev, _ := wire.NewShaHashFromStr("0000000000001323db1ab3f247bcb1e92592004b43e4bed0966ed09f675cf269")
	testnet3Merk, _ := wire.NewShaHashFromStr("9ec9629ada4429e4a6e80776d7c22cb1c9d6672ce0f1b5a9829f8c69db640a86")
	Testnet3Checkpoint = wire.BlockHeader{
		Version:    536870912,
		PrevBlock:  *testnet3Prev,
		MerkleRoot: *testnet3Merk,
		Timestamp:  time.Unix(1467640552, 0),
		Bits:       436611976,
		Nonce:      693901454,
	}
}
