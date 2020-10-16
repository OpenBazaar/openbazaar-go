package math

import (
	gbig "math/big"

	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/util/math"
)

var (
	// Coefficients in Q.128 format
	lnNumCoef   []*gbig.Int
	lnDenomCoef []*gbig.Int
	ln2         big.Int
)

func init() {
	// ln approximation coefficients
	// parameters are in integer format,
	// coefficients are *2^-128 of that
	// so we can just load them if we treat them as Q.128
	num := []string{
		"261417938209272870992496419296200268025",
		"7266615505142943436908456158054846846897",
		"32458783941900493142649393804518050491988",
		"17078670566130897220338060387082146864806",
		"-35150353308172866634071793531642638290419",
		"-20351202052858059355702509232125230498980",
		"-1563932590352680681114104005183375350999",
	}
	lnNumCoef = math.Parse(num)

	denom := []string{
		"49928077726659937662124949977867279384",
		"2508163877009111928787629628566491583994",
		"21757751789594546643737445330202599887121",
		"53400635271583923415775576342898617051826",
		"41248834748603606604000911015235164348839",
		"9015227820322455780436733526367238305537",
		"340282366920938463463374607431768211456",
	}
	lnDenomCoef = math.Parse(denom)

	constStrs := []string{
		"235865763225513294137944142764154484399", // ln(2)
	}
	constBigs := Parse(constStrs)
	ln2 = big.NewFromGo(constBigs[0])
}

// The natural log of Q.128 x.
func Ln(z big.Int) big.Int {
	// bitlen - 1 - precision
	k := int64(z.BitLen()) - 1 - Precision128 // Q.0
	x := big.Zero()                           // nolint:ineffassign

	if k > 0 {
		x = big.Rsh(z, uint(k)) // Q.128
	} else {
		x = big.Lsh(z, uint(-k)) // Q.128
	}

	// ln(z) = ln(x * 2^k) = ln(x) + k * ln2
	lnz := big.Mul(big.NewInt(k), ln2)         // Q.0 * Q.128 => Q.128
	return big.Sum(lnz, lnBetweenOneAndTwo(x)) // Q.128
}

// The natural log of x, specified in Q.128 format
// Should only use with 1 <= x <= 2
// Output is in Q.128 format.
func lnBetweenOneAndTwo(x big.Int) big.Int {
	// ln is approximated by rational function
	// polynomials of the rational function are evaluated using Horner's method
	num := Polyval(lnNumCoef, x.Int)     // Q.128
	denom := Polyval(lnDenomCoef, x.Int) // Q.128

	num = num.Lsh(num, Precision128)          // Q.128 => Q.256
	return big.NewFromGo(num.Div(num, denom)) // Q.256 / Q.128 => Q.128
}
