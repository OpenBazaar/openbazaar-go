package spvwallet

import (
	"fmt"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"math/big"
	"sort"
	"sync"
	"time"
)

// Blockchain settings.  These are kindof Bitcoin specific, but not contained in
// chaincfg.Params so they'll go here.  If you're into the [ANN]altcoin scene,
// you may want to paramaterize these constants.
const (
	targetTimespan      = time.Hour * 24 * 14
	targetSpacing       = time.Minute * 10
	epochLength         = int32(targetTimespan / targetSpacing) // 2016
	maxDiffAdjust       = 4
	minRetargetTimespan = int64(targetTimespan / maxDiffAdjust)
	maxRetargetTimespan = int64(targetTimespan * maxDiffAdjust)
	medianTimeBlocks    = 11
)

type ChainState int

const (
	SYNCING = 0
	WAITING = 1
)

// Wrapper around Headers implementation that handles all blockchain operations
type Blockchain struct {
	lock   *sync.Mutex
	params *chaincfg.Params
	db     Headers
	state  ChainState
}

func NewBlockchain(filePath string, walletCreationDate time.Time, params *chaincfg.Params) (*Blockchain, error) {
	b := &Blockchain{
		lock:   new(sync.Mutex),
		params: params,
		db:     NewHeaderDB(filePath),
	}

	h, err := b.db.Height()
	if h == 0 || err != nil {
		log.Info("Initializing headers db with checkpoints")
		checkpoint := GetCheckpoint(walletCreationDate, params)
		// Put the checkpoint to the db
		sh := StoredHeader{
			header:    checkpoint.Header,
			height:    checkpoint.Height,
			totalWork: big.NewInt(0),
		}
		err := b.db.Put(sh, true)
		if err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (b *Blockchain) CommitHeader(header wire.BlockHeader) (bool, *StoredHeader, uint32, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	newTip := false
	var commonAncestor *StoredHeader
	// Fetch our current best header from the db
	bestHeader, err := b.db.GetBestHeader()
	if err != nil {
		return false, nil, 0, err
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
			return false, nil, 0, fmt.Errorf("Header %s does not extend any known headers", header.BlockHash().String())
		}
	}
	valid := b.CheckHeader(header, parentHeader)
	if !valid {
		return false, nil, 0, nil
	}
	// If this block is already the tip, return
	headerHash := header.BlockHash()
	if tipHash.IsEqual(&headerHash) {
		return newTip, nil, 0, nil
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
			commonAncestor, err = b.GetCommonAncestor(StoredHeader{header: header, height: parentHeader.height + 1}, bestHeader)
			if err != nil {
				log.Errorf("Error calculating common ancestor: %s", err.Error())
				return newTip, commonAncestor, 0, err
			}
			log.Warningf("REORG!!! REORG!!! REORG!!! At block %d, Wiped out %d blocks", int(bestHeader.height), int(bestHeader.height-commonAncestor.height))
		}
	}
	newHeight := parentHeader.height + 1
	// Put the header to the database
	err = b.db.Put(StoredHeader{
		header:    header,
		height:    newHeight,
		totalWork: cumulativeWork,
	}, newTip)
	if err != nil {
		return newTip, commonAncestor, 0, err
	}
	return newTip, commonAncestor, newHeight, nil
}

func (b *Blockchain) CheckHeader(header wire.BlockHeader, prevHeader StoredHeader) bool {
	// Get hash of n-1 header
	prevHash := prevHeader.header.BlockHash()
	height := prevHeader.height

	// Check if headers link together.  That whole 'blockchain' thing.
	if prevHash.IsEqual(&header.PrevBlock) == false {
		log.Errorf("Headers %d and %d don't link.\n", height, height+1)
		return false
	}

	// Check the header meets the difficulty requirement
	if !b.params.ReduceMinDifficulty {
		diffTarget, err := b.calcRequiredWork(header, int32(height+1), prevHeader)
		if err != nil {
			log.Errorf("Error calclating difficulty", err)
			return false
		}
		if header.Bits != diffTarget {
			log.Warningf("Block %d %s incorrect difficulty.  Read %d, expect %d\n",
				height+1, header.BlockHash().String(), header.Bits, diffTarget)
			return false
		}
	}

	// Check if there's a valid proof of work.  That whole "Bitcoin" thing.
	if !checkProofOfWork(header, b.params) {
		log.Debugf("Block %d bad proof of work.\n", height+1)
		return false
	}

	return true // it must have worked if there's no errors and got to the end.
}

// Get the PoW target this block should meet. We may need to handle a difficulty adjustment
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
		// Just return the bits from the last header
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

func (b *Blockchain) CalcMedianTimePast(header *wire.BlockHeader) (time.Time, error) {
	iterNode := StoredHeader{header: *header}
	var err error

	timestamps := make([]int64, medianTimeBlocks)
	numNodes := 0

	for i := 0; i < medianTimeBlocks; i++ {
		numNodes++
		timestamps[i] = iterNode.header.Timestamp.Unix()
		iterNode, err = b.db.GetHeader(header.BlockHash())
		if err != nil {
			return time.Time{}, err
		}
	}
	timestamps = timestamps[:numNodes]
	sort.Sort(timeSorter(timestamps))
	medianTimestamp := timestamps[numNodes/2]
	return time.Unix(medianTimestamp, 0), nil
	return time.Now(), nil
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
		if start >= 9 {
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

// Returns last header before reorg point
func (b *Blockchain) GetCommonAncestor(bestHeader, prevBestHeader StoredHeader) (*StoredHeader, error) {
	var err error
	rollback := func(parent StoredHeader, n int) (StoredHeader, error) {
		for i := 0; i < n; i++ {
			parent, err = b.db.GetPreviousHeader(parent.header)
			if err != nil {
				return parent, err
			}
		}
		return parent, nil
	}

	majority := bestHeader
	minority := prevBestHeader
	if bestHeader.height > prevBestHeader.height {
		majority, err = rollback(majority, int(bestHeader.height-prevBestHeader.height))
		if err != nil {
			return nil, err
		}
	} else if prevBestHeader.height > bestHeader.height {
		minority, err = rollback(minority, int(prevBestHeader.height-bestHeader.height))
		if err != nil {
			return nil, err
		}
	}

	for {
		majorityHash := majority.header.BlockHash()
		minorityHash := minority.header.BlockHash()
		if majorityHash.IsEqual(&minorityHash) {
			return &majority, nil
		}
		majority, err = b.db.GetPreviousHeader(majority.header)
		if err != nil {
			return nil, err
		}
		minority, err = b.db.GetPreviousHeader(minority.header)
		if err != nil {
			return nil, err
		}
	}
}

func (b *Blockchain) ChainState() ChainState {
	return b.state
}

func (b *Blockchain) SetChainState(state ChainState) {
	b.state = state
}

func (b *Blockchain) Close() {
	b.lock.Lock()
	b.db.Close()
}

// Verifies the header hashes into something lower than specified by the 4-byte bits field.
func checkProofOfWork(header wire.BlockHeader, p *chaincfg.Params) bool {
	target := blockchain.CompactToBig(header.Bits)

	// The target must more than 0.  Why can you even encode negative...
	if target.Sign() <= 0 {
		log.Debugf("Block target %064x is neagtive(??)\n", target.Bytes())
		return false
	}
	// The target must be less than the maximum allowed (difficulty 1)
	if target.Cmp(p.PowLimit) > 0 {
		log.Debugf("Block target %064x is "+
			"higher than max of %064x", target, p.PowLimit.Bytes())
		return false
	}
	// The header hash must be less than the claimed target in the header.
	blockHash := header.BlockHash()
	hashNum := blockchain.HashToBig(&blockHash)
	if hashNum.Cmp(target) > 0 {
		log.Debugf("Block hash %064x is higher than "+
			"required target of %064x", hashNum, target)
		return false
	}
	return true
}

// This function takes in a start and end block header and uses the timestamps in each
// to calculate how much of a difficulty adjustment is needed. It returns a new compact
// difficulty target.
func calcDiffAdjust(start, end wire.BlockHeader, p *chaincfg.Params) uint32 {
	duration := end.Timestamp.UnixNano() - start.Timestamp.UnixNano()
	if duration < minRetargetTimespan {
		log.Debugf("Whoa there, block %s off-scale high 4X diff adjustment!",
			end.BlockHash().String())
		duration = minRetargetTimespan
	} else if duration > maxRetargetTimespan {
		log.Debugf("Uh-oh! block %s off-scale low 0.25X diff adjustment!\n",
			end.BlockHash().String())
		duration = maxRetargetTimespan
	}

	// calculation of new 32-byte difficulty target
	// first turn the previous target into a big int
	prevTarget := blockchain.CompactToBig(end.Bits)
	// new target is old * duration...
	newTarget := new(big.Int).Mul(prevTarget, big.NewInt(duration))
	// divided by 2 weeks
	newTarget.Div(newTarget, big.NewInt(int64(targetTimespan)))

	// clip again if above minimum target (too easy)
	if newTarget.Cmp(p.PowLimit) > 0 {
		newTarget.Set(p.PowLimit)
	}

	return blockchain.BigToCompact(newTarget)
}
