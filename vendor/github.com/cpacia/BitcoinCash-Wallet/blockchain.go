// Copyright (C) 2015-2016 The Lightning Network Developers
// Copyright (c) 2016-2017 The OpenBazaar Developers

package bitcoincash

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
	"errors"
)

// Blockchain settings.  These are kindof Bitcoin specific, but not contained in
// chaincfg.Params so they'll go here.  If you're into the [ANN]altcoin scene,
// you may want to paramaterize these constants.
const (
	targetSpacing    = 600
	medianTimeBlocks = 11
)

var OrphanHeaderError = errors.New("header does not extend any known headers")

// Wrapper around Headers implementation that handles all blockchain operations
type Blockchain struct {
	lock        *sync.Mutex
	params      *chaincfg.Params
	db          Headers
	crationDate time.Time
	checkpoint  Checkpoint
}

func NewBlockchain(filePath string, walletCreationDate time.Time, params *chaincfg.Params) (*Blockchain, error) {
	hdb, err := NewHeaderDB(filePath)
	if err != nil {
		return nil, err
	}
	b := &Blockchain{
		lock:        new(sync.Mutex),
		params:      params,
		db:          hdb,
		crationDate: walletCreationDate,
	}
	b.checkpoint = GetCheckpoint(walletCreationDate, params)

	h, err := b.db.Height()
	if h == 0 || err != nil {
		log.Info("Initializing headers db with checkpoints")
		// Put the checkpoint to the db
		sh := StoredHeader{
			header:    b.checkpoint.Header,
			height:    b.checkpoint.Height,
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
	height := prevHeader.height

	// Due to the rolling difficulty period our checkpoint block consists of a block and a hash of a block 146 blocks later
	// During this period we can skip the validity checks as long as block checkpoint + 146 matches the hardcoded hash.
	if height+1 <= b.checkpoint.Height+147 {
		h := header.BlockHash()
		if b.checkpoint.Check2 != nil && height+1 == b.checkpoint.Height+147 && !b.checkpoint.Check2.IsEqual(&h) {
			return false
		}
		return true
	}

	// Get hash of n-1 header
	prevHash := prevHeader.header.BlockHash()

	// Check if headers link together.  That whole 'blockchain' thing.
	if prevHash.IsEqual(&header.PrevBlock) == false {
		log.Errorf("Headers %d and %d don't link.\n", height, height+1)
		return false
	}

	// Check the header meets the difficulty requirement
	if b.params.Name != chaincfg.RegressionNetParams.Name { // Don't need to check difficulty on regtest
		diffTarget, err := b.calcRequiredWork(header, int32(height+1), prevHeader)
		if err != nil {
			log.Errorf("Error calclating difficulty", err)
			return false
		}
		if header.Bits != diffTarget && b.params.Name == chaincfg.MainNetParams.Name {
			log.Warningf("Block %d %s incorrect difficulty.  Read %d, expect %d\n",
				height+1, header.BlockHash().String(), header.Bits, diffTarget)
			return false
		} else if diffTarget == b.params.PowLimitBits && header.Bits > diffTarget && b.params.Name == chaincfg.TestNet3Params.Name {
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
	// Special difficulty rule for testnet
	if b.params.ReduceMinDifficulty && header.Timestamp.After(prevHeader.header.Timestamp.Add(targetSpacing*2)) {
		return b.params.PowLimitBits, nil
	}

	suitableHeader, err := b.GetSuitableBlock(prevHeader)
	if err != nil {
		log.Error(err)
		return 0, err
	}
	epoch, err := b.GetEpoch(prevHeader.header)
	if err != nil {
		log.Error(err)
		return 0, err
	}
	return calcDiffAdjust(epoch, suitableHeader, b.params), nil
}

func (b *Blockchain) CalcMedianTimePast(header wire.BlockHeader) (time.Time, error) {
	timestamps := make([]int64, medianTimeBlocks)
	numNodes := 0
	iterNode := StoredHeader{header: header}
	var err error

	for i := 0; i < medianTimeBlocks; i++ {
		numNodes++
		timestamps[i] = iterNode.header.Timestamp.Unix()
		iterNode, err = b.db.GetPreviousHeader(iterNode.header)
		if err != nil {
			return time.Time{}, err
		}
	}
	timestamps = timestamps[:numNodes]
	sort.Sort(timeSorter(timestamps))
	medianTimestamp := timestamps[numNodes/2]
	return time.Unix(medianTimestamp, 0), nil
}

// Rollsback and grabs block n-144, n-145, and n-146, sorts them by timestamps and returns the middle header.
func (b *Blockchain) GetEpoch(hdr wire.BlockHeader) (StoredHeader, error) {
	sh := StoredHeader{header: hdr}
	var err error
	for i := 0; i < 144; i++ {
		sh, err = b.db.GetPreviousHeader(sh.header)
		if err != nil {
			return sh, err
		}
	}
	oneFourtyFour := sh
	sh, err = b.db.GetPreviousHeader(oneFourtyFour.header)
	if err != nil {
		return sh, err
	}
	oneFourtyFive := sh
	sh, err = b.db.GetPreviousHeader(oneFourtyFive.header)
	if err != nil {
		return sh, err
	}
	oneFourtySix := sh
	headers := []StoredHeader{oneFourtyFour, oneFourtyFive, oneFourtySix}
	sort.Sort(blockSorter(headers))
	return headers[1], nil
}

// Rollsback grabs the last two headers before this one. Sorts the three and returns the mid.
func (b *Blockchain) GetSuitableBlock(hdr StoredHeader) (StoredHeader, error) {
	n := hdr
	sh, err := b.db.GetPreviousHeader(hdr.header)
	if err != nil {
		return sh, err
	}
	n1 := sh
	sh, err = b.db.GetPreviousHeader(n1.header)
	if err != nil {
		return sh, err
	}
	n2 := sh
	headers := []StoredHeader{n, n1, n2}
	sort.Sort(blockSorter(headers))
	return headers[1], nil
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

func (b *Blockchain) GetBlockLocator() blockchain.BlockLocator {
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
	return blockchain.BlockLocator(ret)
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

// Rollback the header database to the last header before time t.
// We shouldn't go back further than the checkpoint
func (b *Blockchain) Rollback(t time.Time) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	checkpoint := GetCheckpoint(b.crationDate, b.params)
	checkPointHash := checkpoint.Header.BlockHash()
	sh, err := b.db.GetBestHeader()
	if err != nil {
		return err
	}
	// If t is greater than the timestamp at the tip then do nothing
	if sh.header.Timestamp.Before(t) {
		return nil
	}
	// If the tip is our checkpoint then do nothing
	checkHash := sh.header.BlockHash()
	if checkHash.IsEqual(&checkPointHash) {
		return nil
	}
	rollbackHeight := uint32(0)
	for i := 0; i < 1000000000; i++ {
		sh, err = b.db.GetPreviousHeader(sh.header)
		if err != nil {
			return err
		}
		checkHash := sh.header.BlockHash()
		// If we rolled back to the checkpoint then stop here and set the checkpoint as the tip
		if checkHash.IsEqual(&checkPointHash) {
			rollbackHeight = checkpoint.Height
			break
		}
		// If we hit a header created before t then stop here and set this header as the tip
		if sh.header.Timestamp.Before(t) {
			rollbackHeight = sh.height
			break
		}
	}
	err = b.db.DeleteAfter(rollbackHeight)
	if err != nil {
		return err
	}
	return b.db.Put(sh, true)
}

func (b *Blockchain) BestBlock() (StoredHeader, error) {
	sh, err := b.db.GetBestHeader()
	if err != nil {
		return StoredHeader{}, err
	}
	return sh, nil
}

func (b *Blockchain) GetHeader(hash *chainhash.Hash) (StoredHeader, error) {
	sh, err := b.db.GetHeader(*hash)
	if err != nil {
		return sh, err
	}
	return sh, nil
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
func calcDiffAdjust(start, end StoredHeader, p *chaincfg.Params) uint32 {
	work := new(big.Int).Sub(end.totalWork, start.totalWork)

	// In order to avoid difficulty cliffs, we bound the amplitude of the
	// adjustement we are going to do.
	duration := end.header.Timestamp.Unix() - start.header.Timestamp.Unix()
	if duration > 288*int64(targetSpacing) {
		duration = 288 * int64(targetSpacing)
	} else if duration < 72*int64(targetSpacing) {
		duration = 72 * int64(targetSpacing)
	}

	prjectedWork := new(big.Int).Mul(work, big.NewInt(int64(targetSpacing)))

	pw := new(big.Int).Div(prjectedWork, big.NewInt(duration))

	e := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)

	nt := new(big.Int).Sub(e, pw)

	newTarget := new(big.Int).Div(nt, pw)

	// clip again if above minimum target (too easy)
	if newTarget.Cmp(p.PowLimit) > 0 {
		newTarget.Set(p.PowLimit)
	}
	return blockchain.BigToCompact(newTarget)
}
