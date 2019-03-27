package bitswap

import (
	"math/rand"
)

type fixedRateLimitGenerator struct {
	rateLimit float64
}

// FixedRateLimitGenerator returns a rate limit generatoe that always generates
// the specified rate limit in bytes/sec.
func FixedRateLimitGenerator(rateLimit float64) RateLimitGenerator {
	return &fixedRateLimitGenerator{rateLimit}
}

func (rateLimitGenerator *fixedRateLimitGenerator) NextRateLimit() float64 {
	return rateLimitGenerator.rateLimit
}

type variableRateLimitGenerator struct {
	rateLimit float64
	std       float64
	rng       *rand.Rand
}

// VariableRateLimitGenerator makes rate limites that following a normal distribution.
func VariableRateLimitGenerator(rateLimit float64, std float64, rng *rand.Rand) RateLimitGenerator {
	if rng == nil {
		rng = sharedRNG
	}

	return &variableRateLimitGenerator{
		std:       std,
		rng:       rng,
		rateLimit: rateLimit,
	}
}

func (rateLimitGenerator *variableRateLimitGenerator) NextRateLimit() float64 {
	return rateLimitGenerator.rng.NormFloat64()*rateLimitGenerator.std + rateLimitGenerator.rateLimit
}
