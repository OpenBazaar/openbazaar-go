/* this is blockchain technology.  Well, except without the blocks.
Really it's header chain technology.
The blocks themselves don't really make a chain.  Just the headers do.
*/

package spvwallet

import (
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"math/big"
	"time"
)

// blockchain settings.  These are kindof bitcoin specific, but not contained in
// chaincfg.Params so they'll go here.  If you're into the [ANN]altcoin scene,
// you may want to paramaterize these constants.
const (
	targetTimespan      = time.Hour * 24 * 14
	targetSpacing       = time.Minute * 10
	epochLength         = int32(targetTimespan / targetSpacing) // 2016
	maxDiffAdjust       = 4
	minRetargetTimespan = int64(targetTimespan / maxDiffAdjust)
	maxRetargetTimespan = int64(targetTimespan * maxDiffAdjust)
)

/* checkProofOfWork verifies the header hashes into something
lower than specified by the 4-byte bits field. */
func checkProofOfWork(header wire.BlockHeader, p *chaincfg.Params) bool {
	target := blockchain.CompactToBig(header.Bits)

	// The target must more than 0.  Why can you even encode negative...
	if target.Sign() <= 0 {
		log.Debugf("block target %064x is neagtive(??)\n", target.Bytes())
		return false
	}
	// The target must be less than the maximum allowed (difficulty 1)
	if target.Cmp(p.PowLimit) > 0 {
		log.Debugf("block target %064x is "+
			"higher than max of %064x", target, p.PowLimit.Bytes())
		return false
	}
	// The header hash must be less than the claimed target in the header.
	blockHash := header.BlockHash()
	hashNum := blockchain.HashToBig(&blockHash)
	if hashNum.Cmp(target) > 0 {
		log.Debugf("block hash %064x is higher than "+
			"required target of %064x", hashNum, target)
		return false
	}
	return true
}

/* calcDiff returns a bool given two block headers.  This bool is
true if the correct dificulty adjustment is seen in the "next" header.
Only feed it headers n-2016 and n-1, otherwise it will calculate a difficulty
when no adjustment should take place, and return false.
Note that the epoch is actually 2015 blocks long, which is confusing. */
func calcDiffAdjust(start, end wire.BlockHeader, p *chaincfg.Params) uint32 {
	duration := end.Timestamp.UnixNano() - start.Timestamp.UnixNano()
	if duration < minRetargetTimespan {
		log.Debugf("whoa there, block %s off-scale high 4X diff adjustment!",
			end.BlockHash().String())
		duration = minRetargetTimespan
	} else if duration > maxRetargetTimespan {
		log.Debugf("Uh-oh! block %s off-scale low 0.25X diff adjustment!\n",
			end.BlockHash().String())
		duration = maxRetargetTimespan
	}

	// calculation of new 32-byte difficulty target
	// first turn the previous target into a big int
	prevTarget := blockchain.CompactToBig(start.Bits)
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
