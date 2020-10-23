package math

import (
	"math/big"

	"github.com/filecoin-project/specs-actors/actors/util/math"
)

var (
	// Coefficents in Q.128 format
	expNumCoef  []*big.Int
	expDenoCoef []*big.Int
)

func init() {

	// parameters are in integer format,
	// coefficients are *2^-128 of that
	// so we can just load them if we treat them as Q.128
	num := []string{
		"-648770010757830093818553637600",
		"67469480939593786226847644286976",
		"-3197587544499098424029388939001856",
		"89244641121992890118377641805348864",
		"-1579656163641440567800982336819953664",
		"17685496037279256458459817590917169152",
		"-115682590513835356866803355398940131328",
		"340282366920938463463374607431768211456",
	}
	expNumCoef = math.Parse(num)

	deno := []string{
		"1225524182432722209606361",
		"114095592300906098243859450",
		"5665570424063336070530214243",
		"194450132448609991765137938448",
		"5068267641632683791026134915072",
		"104716890604972796896895427629056",
		"1748338658439454459487681798864896",
		"23704654329841312470660182937960448",
		"259380097567996910282699886670381056",
		"2250336698853390384720606936038375424",
		"14978272436876548034486263159246028800",
		"72144088983913131323343765784380833792",
		"224599776407103106596571252037123047424",
		"340282366920938463463374607431768211456",
	}
	expDenoCoef = math.Parse(deno)
}

// ExpNeg accepts x in Q.128 format and computes e^-x.
// It is most precise within [0, 1.725) range, where error is less than 3.4e-30.
// Over the [0, 5) range its error is less than 4.6e-15.
// Output is in Q.128 format.
func ExpNeg(x *big.Int) *big.Int {
	// exp is approximated by rational function
	// polynomials of the rational function are evaluated using Horner's method
	num := Polyval(expNumCoef, x)   // Q.128
	deno := Polyval(expDenoCoef, x) // Q.128

	num = num.Lsh(num, Precision128) // Q.256
	return num.Div(num, deno)        // Q.256 / Q.128 => Q.128
}
