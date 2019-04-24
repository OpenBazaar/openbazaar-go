package bitswap

import (
	"math/rand"
	"time"

	"gx/ipfs/QmUe1WCHkQaz4UeNKiHDUBV2T6i9prc3DniqyHPXyfGaUq/go-ipfs-delay"
)

var sharedRNG = rand.New(rand.NewSource(time.Now().UnixNano()))

// InternetLatencyDelayGenerator generates three clusters of delays,
// typical of the type of peers you would encounter on the interenet.
// Given a base delay time T, the wait time generated will be either:
// 1. A normalized distribution around the base time
// 2. A normalized distribution around the base time plus a "medium" delay
// 3. A normalized distribution around the base time plus a "large" delay
// The size of the medium & large delays are determined when the generator
// is constructed, as well as the relative percentages with which delays fall
// into each of the three different clusters, and the standard deviation for
// the normalized distribution.
// This can be used to generate a number of scenarios typical of latency
// distribution among peers on the internet.
func InternetLatencyDelayGenerator(
	mediumDelay time.Duration,
	largeDelay time.Duration,
	percentMedium float64,
	percentLarge float64,
	std time.Duration,
	rng *rand.Rand) delay.Generator {
	if rng == nil {
		rng = sharedRNG
	}

	return &internetLatencyDelayGenerator{
		mediumDelay:   mediumDelay,
		largeDelay:    largeDelay,
		percentLarge:  percentLarge,
		percentMedium: percentMedium,
		std:           std,
		rng:           rng,
	}
}

type internetLatencyDelayGenerator struct {
	mediumDelay   time.Duration
	largeDelay    time.Duration
	percentLarge  float64
	percentMedium float64
	std           time.Duration
	rng           *rand.Rand
}

func (d *internetLatencyDelayGenerator) NextWaitTime(t time.Duration) time.Duration {
	clusterDistribution := d.rng.Float64()
	baseDelay := time.Duration(d.rng.NormFloat64()*float64(d.std)) + t
	if clusterDistribution < d.percentLarge {
		return baseDelay + d.largeDelay
	} else if clusterDistribution < d.percentMedium+d.percentLarge {
		return baseDelay + d.mediumDelay
	}
	return baseDelay
}
