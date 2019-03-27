package bchutil

import (
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"golang.org/x/crypto/ripemd160"
)

var (
	// ErrChecksumMismatch describes an error where decoding failed due
	// to a bad checksum.
	ErrChecksumMismatch = errors.New("checksum mismatch")

	// ErrUnknownAddressType describes an error where an address can not
	// decoded as a specific address type due to the string encoding
	// begining with an identifier byte unknown to any standard or
	// registered (via chaincfg.Register) network.
	ErrUnknownAddressType = errors.New("unknown address type")

	// ErrAddressCollision describes an error where an address can not
	// be uniquely determined as either a pay-to-pubkey-hash or
	// pay-to-script-hash address since the leading identifier is used for
	// describing both address kinds, but for different networks.  Rather
	// than assuming or defaulting to one or the other, this error is
	// returned and the caller must decide how to decode the address.
	ErrAddressCollision = errors.New("address collision")

	// ErrInvalidFormat describes an error where decoding failed due to invalid version
	ErrInvalidFormat = errors.New("invalid format: version and/or checksum bytes missing")

	Prefixes map[string]string
)

type AddressType int

const (
	P2PKH AddressType = 0
	P2SH  AddressType = 1
)

func init() {
	Prefixes = make(map[string]string)
	Prefixes[chaincfg.MainNetParams.Name] = "bitcoincash"
	Prefixes[chaincfg.TestNet3Params.Name] = "bchtest"
	Prefixes[chaincfg.RegressionNetParams.Name] = "bchreg"
}

type data []byte

/**
 * The cashaddr character set for encoding.
 */
const CHARSET string = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

/**
 * The cashaddr character set for decoding.
 */
var CHARSET_REV = [128]int8{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 15, -1, 10, 17, 21, 20, 26, 30, 7,
	5, -1, -1, -1, -1, -1, -1, -1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22,
	31, 27, 19, -1, 1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1,
	-1, -1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1, 1, 0,
	3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1, -1,
}

/**
 * Concatenate two byte arrays.
 */
func Cat(x, y data) data {
	return append(x, y...)
}

/**
 * This function will compute what 8 5-bit values to XOR into the last 8 input
 * values, in order to make the checksum 0. These 8 values are packed together
 * in a single 40-bit integer. The higher bits correspond to earlier values.
 */
func PolyMod(v data) uint64 {
	/**
	 * The input is interpreted as a list of coefficients of a polynomial over F
	 * = GF(32), with an implicit 1 in front. If the input is [v0,v1,v2,v3,v4],
	 * that polynomial is v(x) = 1*x^5 + v0*x^4 + v1*x^3 + v2*x^2 + v3*x + v4.
	 * The implicit 1 guarantees that [v0,v1,v2,...] has a distinct checksum
	 * from [0,v0,v1,v2,...].
	 *
	 * The output is a 40-bit integer whose 5-bit groups are the coefficients of
	 * the remainder of v(x) mod g(x), where g(x) is the cashaddr generator, x^8
	 * + {19}*x^7 + {3}*x^6 + {25}*x^5 + {11}*x^4 + {25}*x^3 + {3}*x^2 + {19}*x
	 * + {1}. g(x) is chosen in such a way that the resulting code is a BCH
	 * code, guaranteeing detection of up to 4 errors within a window of 1025
	 * characters. Among the various possible BCH codes, one was selected to in
	 * fact guarantee detection of up to 5 errors within a window of 160
	 * characters and 6 erros within a window of 126 characters. In addition,
	 * the code guarantee the detection of a burst of up to 8 errors.
	 *
	 * Note that the coefficients are elements of GF(32), here represented as
	 * decimal numbers between {}. In this finite field, addition is just XOR of
	 * the corresponding numbers. For example, {27} + {13} = {27 ^ 13} = {22}.
	 * Multiplication is more complicated, and requires treating the bits of
	 * values themselves as coefficients of a polynomial over a smaller field,
	 * GF(2), and multiplying those polynomials mod a^5 + a^3 + 1. For example,
	 * {5} * {26} = (a^2 + 1) * (a^4 + a^3 + a) = (a^4 + a^3 + a) * a^2 + (a^4 +
	 * a^3 + a) = a^6 + a^5 + a^4 + a = a^3 + 1 (mod a^5 + a^3 + 1) = {9}.
	 *
	 * During the course of the loop below, `c` contains the bitpacked
	 * coefficients of the polynomial constructed from just the values of v that
	 * were processed so far, mod g(x). In the above example, `c` initially
	 * corresponds to 1 mod (x), and after processing 2 inputs of v, it
	 * corresponds to x^2 + v0*x + v1 mod g(x). As 1 mod g(x) = 1, that is the
	 * starting value for `c`.
	 */
	c := uint64(1)
	for _, d := range v {
		/**
		 * We want to update `c` to correspond to a polynomial with one extra
		 * term. If the initial value of `c` consists of the coefficients of
		 * c(x) = f(x) mod g(x), we modify it to correspond to
		 * c'(x) = (f(x) * x + d) mod g(x), where d is the next input to
		 * process.
		 *
		 * Simplifying:
		 * c'(x) = (f(x) * x + d) mod g(x)
		 *         ((f(x) mod g(x)) * x + d) mod g(x)
		 *         (c(x) * x + d) mod g(x)
		 * If c(x) = c0*x^5 + c1*x^4 + c2*x^3 + c3*x^2 + c4*x + c5, we want to
		 * compute
		 * c'(x) = (c0*x^5 + c1*x^4 + c2*x^3 + c3*x^2 + c4*x + c5) * x + d
		 *                                                             mod g(x)
		 *       = c0*x^6 + c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d
		 *                                                             mod g(x)
		 *       = c0*(x^6 mod g(x)) + c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 +
		 *                                                             c5*x + d
		 * If we call (x^6 mod g(x)) = k(x), this can be written as
		 * c'(x) = (c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d) + c0*k(x)
		 */

		// First, determine the value of c0:
		c0 := byte(c >> 35)

		// Then compute c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d:
		c = ((c & 0x07ffffffff) << 5) ^ uint64(d)

		// Finally, for each set bit n in c0, conditionally add {2^n}k(x):
		if c0&0x01 > 0 {
			// k(x) = {19}*x^7 + {3}*x^6 + {25}*x^5 + {11}*x^4 + {25}*x^3 +
			//        {3}*x^2 + {19}*x + {1}
			c ^= 0x98f2bc8e61
		}

		if c0&0x02 > 0 {
			// {2}k(x) = {15}*x^7 + {6}*x^6 + {27}*x^5 + {22}*x^4 + {27}*x^3 +
			//           {6}*x^2 + {15}*x + {2}
			c ^= 0x79b76d99e2
		}

		if c0&0x04 > 0 {
			// {4}k(x) = {30}*x^7 + {12}*x^6 + {31}*x^5 + {5}*x^4 + {31}*x^3 +
			//           {12}*x^2 + {30}*x + {4}
			c ^= 0xf33e5fb3c4
		}

		if c0&0x08 > 0 {
			// {8}k(x) = {21}*x^7 + {24}*x^6 + {23}*x^5 + {10}*x^4 + {23}*x^3 +
			//           {24}*x^2 + {21}*x + {8}
			c ^= 0xae2eabe2a8
		}

		if c0&0x10 > 0 {
			// {16}k(x) = {3}*x^7 + {25}*x^6 + {7}*x^5 + {20}*x^4 + {7}*x^3 +
			//            {25}*x^2 + {3}*x + {16}
			c ^= 0x1e4f43e470
		}
	}

	/**
	 * PolyMod computes what value to xor into the final values to make the
	 * checksum 0. However, if we required that the checksum was 0, it would be
	 * the case that appending a 0 to a valid list of values would result in a
	 * new valid list. For that reason, cashaddr requires the resulting checksum
	 * to be 1 instead.
	 */
	return c ^ 1
}

/**
 * Convert to lower case.
 *
 * Assume the input is a character.
 */
func LowerCase(c byte) byte {
	// ASCII black magic.
	return c | 0x20
}

/**
 * Expand the address prefix for the checksum computation.
 */
func ExpandPrefix(prefix string) data {
	ret := make(data, len(prefix)+1)
	for i := 0; i < len(prefix); i++ {
		ret[i] = byte(prefix[i]) & 0x1f
	}

	ret[len(prefix)] = 0
	return ret
}

/**
 * Verify a checksum.
 */
func VerifyChecksum(prefix string, payload data) bool {
	return PolyMod(Cat(ExpandPrefix(prefix), payload)) == 0
}

/**
 * Create a checksum.
 */
func CreateChecksum(prefix string, payload data) data {
	enc := Cat(ExpandPrefix(prefix), payload)
	// Append 8 zeroes.
	enc = Cat(enc, data{0, 0, 0, 0, 0, 0, 0, 0})
	// Determine what to XOR into those 8 zeroes.
	mod := PolyMod(enc)
	ret := make(data, 8)
	for i := 0; i < 8; i++ {
		// Convert the 5-bit groups in mod to checksum values.
		ret[i] = byte((mod >> uint(5*(7-i))) & 0x1f)
	}
	return ret
}

/**
 * Encode a cashaddr string.
 */
func Encode(prefix string, payload data) string {
	checksum := CreateChecksum(prefix, payload)
	combined := Cat(payload, checksum)
	ret := ""

	for _, c := range combined {
		ret += string(CHARSET[c])
	}

	return ret
}

/**
 * Decode a cashaddr string.
 */
func DecodeCashAddress(str string) (string, data, error) {
	// Go over the string and do some sanity checks.
	lower, upper := false, false
	prefixSize := 0
	for i := 0; i < len(str); i++ {
		c := byte(str[i])
		if c >= 'a' && c <= 'z' {
			lower = true
			continue
		}

		if c >= 'A' && c <= 'Z' {
			upper = true
			continue
		}

		if c >= '0' && c <= '9' {
			// We cannot have numbers in the prefix.
			if prefixSize == 0 {
				return "", data{}, errors.New("Addresses cannot have numbers in the prefix")
			}

			continue
		}

		if c == ':' {
			// The separator must not be the first character, and there must not
			// be 2 separators.
			if i == 0 || prefixSize != 0 {
				return "", data{}, errors.New("The separator must not be the first character")
			}

			prefixSize = i
			continue
		}

		// We have an unexpected character.
		return "", data{}, errors.New("Unexpected character")
	}

	// We must have a prefix and a data part and we can't have both uppercase
	// and lowercase.
	if prefixSize == 0 {
		return "", data{}, errors.New("Address must have a prefix")
	}

	if upper && lower {
		return "", data{}, errors.New("Addresses cannot use both upper and lower case characters")
	}

	// Get the prefix.
	var prefix string
	for i := 0; i < prefixSize; i++ {
		prefix += string(LowerCase(str[i]))
	}

	// Decode values.
	valuesSize := len(str) - 1 - prefixSize
	values := make(data, valuesSize)
	for i := 0; i < valuesSize; i++ {
		c := byte(str[i+prefixSize+1])
		// We have an invalid char in there.
		if c > 127 || CHARSET_REV[c] == -1 {
			return "", data{}, errors.New("Invalid character")
		}

		values[i] = byte(CHARSET_REV[c])
	}

	// Verify the checksum.
	if !VerifyChecksum(prefix, values) {
		return "", data{}, ErrChecksumMismatch
	}

	return prefix, values[:len(values)-8], nil
}

func CheckEncodeCashAddress(input []byte, prefix string, t AddressType) string {
	k, err := packAddressData(t, input)
	if err != nil {
		fmt.Println("%v", err)
		return ""
	}
	return Encode(prefix, k)
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies the checksum.
func CheckDecodeCashAddress(input string) (result []byte, prefix string, t AddressType, err error) {
	prefix, data, err := DecodeCashAddress(input)
	if err != nil {
		return data, prefix, P2PKH, err
	}
	data, err = convertBits(data, 5, 8, false)
	if err != nil {
		return data, prefix, P2PKH, err
	}
	if len(data) != 21 {
		return data, prefix, P2PKH, errors.New("Incorrect data length")
	}
	switch data[0] {
	case 0x00:
		t = P2PKH
	case 0x08:
		t = P2SH
	}
	return data[1:21], prefix, t, nil
}

// encodeAddress returns a human-readable payment address given a ripemd160 hash
// and prefix which encodes the bitcoin cash network and address type.  It is used
// in both pay-to-pubkey-hash (P2PKH) and pay-to-script-hash (P2SH) address
// encoding.
func encodeCashAddress(hash160 []byte, prefix string, t AddressType) string {
	return CheckEncodeCashAddress(hash160[:ripemd160.Size], prefix, t)
}

// DecodeAddress decodes the string encoding of an address and returns
// the Address if addr is a valid encoding for a known address type.
//
// The bitcoin cash network the address is associated with is extracted if possible.
func DecodeAddress(addr string, defaultNet *chaincfg.Params) (btcutil.Address, error) {
	pre, ok := Prefixes[defaultNet.Name]
	if !ok {
		return nil, errors.New("unknown network parameters")
	}

	// Add prefix if it does not exist
	if len(addr) >= len(pre)+1 && addr[:len(pre)+1] != pre+":" {
		addr = pre + ":" + addr
	}

	// Switch on decoded length to determine the type.
	decoded, _, typ, err := CheckDecodeCashAddress(addr)
	if err != nil {
		if err == ErrChecksumMismatch {
			return nil, ErrChecksumMismatch
		}
		return nil, errors.New("decoded address is of unknown format")
	}
	switch len(decoded) {
	case ripemd160.Size: // P2PKH or P2SH
		switch typ {
		case P2PKH:
			return newCashAddressPubKeyHash(decoded, defaultNet)
		case P2SH:
			return newCashAddressScriptHashFromHash(decoded, defaultNet)
		default:
			return nil, ErrUnknownAddressType
		}

	default:
		return nil, errors.New("decoded address is of unknown size")
	}
}

// AddressPubKeyHash is an Address for a pay-to-pubkey-hash (P2PKH)
// transaction.
type CashAddressPubKeyHash struct {
	hash   [ripemd160.Size]byte
	prefix string
}

// NewAddressPubKeyHash returns a new AddressPubKeyHash.  pkHash mustbe 20
// bytes.
func NewCashAddressPubKeyHash(pkHash []byte, net *chaincfg.Params) (*CashAddressPubKeyHash, error) {
	return newCashAddressPubKeyHash(pkHash, net)
}

// newAddressPubKeyHash is the internal API to create a pubkey hash address
// with a known leading identifier byte for a network, rather than looking
// it up through its parameters.  This is useful when creating a new address
// structure from a string encoding where the identifer byte is already
// known.
func newCashAddressPubKeyHash(pkHash []byte, net *chaincfg.Params) (*CashAddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, errors.New("pkHash must be 20 bytes")
	}

	prefix, ok := Prefixes[net.Name]
	if !ok {
		return nil, errors.New("unknown network parameters")
	}

	addr := &CashAddressPubKeyHash{prefix: prefix}
	copy(addr.hash[:], pkHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-pubkey-hash
// address.  Part of the Address interface.
func (a *CashAddressPubKeyHash) EncodeAddress() string {
	return encodeCashAddress(a.hash[:], a.prefix, P2PKH)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a pubkey hash.  Part of the Address interface.
func (a *CashAddressPubKeyHash) ScriptAddress() []byte {
	return a.hash[:]
}

// IsForNet returns whether or not the pay-to-pubkey-hash address is associated
// with the passed bitcoin cash network.
func (a *CashAddressPubKeyHash) IsForNet(net *chaincfg.Params) bool {
	checkPre, ok := Prefixes[net.Name]
	if !ok {
		return false
	}
	return a.prefix == checkPre
}

// String returns a human-readable string for the pay-to-pubkey-hash address.
// This is equivalent to calling EncodeAddress, but is provided so the type can
// be used as a fmt.Stringer.
func (a *CashAddressPubKeyHash) String() string {
	return a.EncodeAddress()
}

// Hash160 returns the underlying array of the pubkey hash.  This can be useful
// when an array is more appropiate than a slice (for example, when used as map
// keys).
func (a *CashAddressPubKeyHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

// AddressScriptHash is an Address for a pay-to-script-hash (P2SH)
// transaction.
type CashAddressScriptHash struct {
	hash   [ripemd160.Size]byte
	prefix string
}

// NewAddressScriptHash returns a new AddressScriptHash.
func NewCashAddressScriptHash(serializedScript []byte, net *chaincfg.Params) (*CashAddressScriptHash, error) {
	scriptHash := btcutil.Hash160(serializedScript)
	return newCashAddressScriptHashFromHash(scriptHash, net)
}

// NewAddressScriptHashFromHash returns a new AddressScriptHash.  scriptHash
// must be 20 bytes.
func NewCashAddressScriptHashFromHash(scriptHash []byte, net *chaincfg.Params) (*CashAddressScriptHash, error) {
	return newCashAddressScriptHashFromHash(scriptHash, net)
}

// newAddressScriptHashFromHash is the internal API to create a script hash
// address with a known leading identifier byte for a network, rather than
// looking it up through its parameters.  This is useful when creating a new
// address structure from a string encoding where the identifer byte is already
// known.
func newCashAddressScriptHashFromHash(scriptHash []byte, net *chaincfg.Params) (*CashAddressScriptHash, error) {
	// Check for a valid script hash length.
	if len(scriptHash) != ripemd160.Size {
		return nil, errors.New("scriptHash must be 20 bytes")
	}

	pre, ok := Prefixes[net.Name]
	if !ok {
		return nil, errors.New("unknown network parameters")
	}

	addr := &CashAddressScriptHash{prefix: pre}
	copy(addr.hash[:], scriptHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-script-hash
// address.  Part of the Address interface.
func (a *CashAddressScriptHash) EncodeAddress() string {
	return encodeCashAddress(a.hash[:], a.prefix, P2SH)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a script hash.  Part of the Address interface.
func (a *CashAddressScriptHash) ScriptAddress() []byte {
	return a.hash[:]
}

// IsForNet returns whether or not the pay-to-script-hash address is associated
// with the passed bitcoin cash network.
func (a *CashAddressScriptHash) IsForNet(net *chaincfg.Params) bool {
	pre, ok := Prefixes[net.Name]
	if !ok {
		return false
	}
	return pre == a.prefix
}

// String returns a human-readable string for the pay-to-script-hash address.
// This is equivalent to calling EncodeAddress, but is provided so the type can
// be used as a fmt.Stringer.
func (a *CashAddressScriptHash) String() string {
	return a.EncodeAddress()
}

// Hash160 returns the underlying array of the script hash.  This can be useful
// when an array is more appropiate than a slice (for example, when used as map
// keys).
func (a *CashAddressScriptHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

// PayToAddrScript creates a new script to pay a transaction output to a the
// specified address.
func cashPayToAddrScript(addr btcutil.Address) ([]byte, error) {
	const nilAddrErrStr = "unable to generate payment script for nil address"

	switch addr := addr.(type) {
	case *CashAddressPubKeyHash:
		if addr == nil {
			return nil, errors.New(nilAddrErrStr)
		}
		return payToPubKeyHashScript(addr.ScriptAddress())

	case *CashAddressScriptHash:
		if addr == nil {
			return nil, errors.New(nilAddrErrStr)
		}
		return payToScriptHashScript(addr.ScriptAddress())
	}
	return nil, fmt.Errorf("unable to generate payment script for unsupported "+
		"address type %T", addr)
}

// payToPubKeyHashScript creates a new script to pay a transaction
// output to a 20-byte pubkey hash. It is expected that the input is a valid
// hash.
func payToPubKeyHashScript(pubKeyHash []byte) ([]byte, error) {
	return txscript.NewScriptBuilder().AddOp(txscript.OP_DUP).AddOp(txscript.OP_HASH160).
		AddData(pubKeyHash).AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_CHECKSIG).
		Script()
}

// payToScriptHashScript creates a new script to pay a transaction output to a
// script hash. It is expected that the input is a valid hash.
func payToScriptHashScript(scriptHash []byte) ([]byte, error) {
	return txscript.NewScriptBuilder().AddOp(txscript.OP_HASH160).AddData(scriptHash).
		AddOp(txscript.OP_EQUAL).Script()
}

// ExtractPkScriptAddrs returns the type of script, addresses and required
// signatures associated with the passed PkScript.  Note that it only works for
// 'standard' transaction script types.  Any data such as public keys which are
// invalid are omitted from the results.
func ExtractPkScriptAddrs(pkScript []byte, chainParams *chaincfg.Params) (btcutil.Address, error) {
	// No valid addresses or required signatures if the script doesn't
	// parse.
	if len(pkScript) == 1+1+20+1 && pkScript[0] == 0xa9 && pkScript[1] == 0x14 && pkScript[22] == 0x87 {
		return NewCashAddressScriptHashFromHash(pkScript[2:22], chainParams)
	} else if len(pkScript) == 1+1+1+20+1+1 && pkScript[0] == 0x76 && pkScript[1] == 0xa9 && pkScript[2] == 0x14 && pkScript[23] == 0x88 && pkScript[24] == 0xac {
		return NewCashAddressPubKeyHash(pkScript[3:23], chainParams)
	}
	return nil, errors.New("unknown script type")
}

// Base32 conversion contains some licensed code

// https://github.com/sipa/bech32/blob/master/ref/go/src/bech32/bech32.go
// Copyright (c) 2017 Takatoshi Nakagawa
// MIT License

func convertBits(data data, fromBits uint, tobits uint, pad bool) (data, error) {
	// General power-of-2 base conversion.
	var uintArr []uint
	for _, i := range data {
		uintArr = append(uintArr, uint(i))
	}
	acc := uint(0)
	bits := uint(0)
	var ret []uint
	maxv := uint((1 << tobits) - 1)
	maxAcc := uint((1 << (fromBits + tobits - 1)) - 1)
	for _, value := range uintArr {
		acc = ((acc << fromBits) | value) & maxAcc
		bits += fromBits
		for bits >= tobits {
			bits -= tobits
			ret = append(ret, (acc>>bits)&maxv)
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, (acc<<(tobits-bits))&maxv)
		}
	} else if bits >= fromBits || ((acc<<(tobits-bits))&maxv) != 0 {
		return []byte{}, errors.New("encoding padding error")
	}
	var dataArr []byte
	for _, i := range ret {
		dataArr = append(dataArr, byte(i))
	}
	return dataArr, nil
}

func packAddressData(addrType AddressType, addrHash data) (data, error) {
	// Pack addr data with version byte.
	if addrType != P2PKH && addrType != P2SH {
		return data{}, errors.New("invalid addrtype")
	}
	versionByte := uint(addrType) << 3
	encodedSize := (uint(len(addrHash)) - 20) / 4
	if (len(addrHash)-20)%4 != 0 {
		return data{}, errors.New("invalid addrhash size")
	}
	if encodedSize < 0 || encodedSize > 8 {
		return data{}, errors.New("encoded size out of valid range")
	}
	versionByte |= encodedSize
	var addrHashUint data
	for _, e := range addrHash {
		addrHashUint = append(addrHashUint, byte(e))
	}
	data := append([]byte{byte(versionByte)}, addrHashUint...)
	packedData, err := convertBits(data, 8, 5, true)
	if err != nil {
		return []byte{}, err
	}
	return packedData, nil
}
