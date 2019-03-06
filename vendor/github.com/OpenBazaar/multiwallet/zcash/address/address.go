package address

import (
	"errors"

	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/base58"
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

	NetIDs map[string]NetID
)

type NetID struct {
	AddressPubKeyHash []byte
	AddressScriptHash []byte
	ZAddress          []byte
}

func init() {
	NetIDs = make(map[string]NetID)
	NetIDs[chaincfg.MainNetParams.Name] = NetID{[]byte{0x1c, 0xb8}, []byte{0x1c, 0xbd}, []byte{0x16, 0x9a}}
	NetIDs[chaincfg.TestNet3Params.Name] = NetID{[]byte{0x1d, 0x25}, []byte{0x1c, 0xba}, []byte{0x16, 0xb6}}
	NetIDs[chaincfg.RegressionNetParams.Name] = NetID{[]byte{0x1d, 0x25}, []byte{0x1c, 0xba}, []byte{0x16, 0xb6}}
}

// checksum: first four bytes of sha256^2
func checksum(input []byte) (cksum [4]byte) {
	h := sha256.Sum256(input)
	h2 := sha256.Sum256(h[:])
	copy(cksum[:], h2[:4])
	return
}

// CheckEncode prepends a version byte and appends a four byte checksum.
func CheckEncode(input []byte, version []byte) string {
	b := make([]byte, 0, 2+len(input)+4)
	b = append(b, version...)
	b = append(b, input[:]...)
	cksum := checksum(b)
	b = append(b, cksum[:]...)
	return base58.Encode(b)
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies the checksum.
func CheckDecode(input string) (result []byte, version []byte, err error) {
	decoded := base58.Decode(input)
	if len(decoded) < 5 {
		return nil, nil, ErrInvalidFormat
	}
	version = append(version, decoded[0:2]...)
	var cksum [4]byte
	copy(cksum[:], decoded[len(decoded)-4:])
	if checksum(decoded[:len(decoded)-4]) != cksum {
		return nil, nil, ErrChecksumMismatch
	}
	payload := decoded[2 : len(decoded)-4]
	result = append(result, payload...)
	return
}

// encodeAddress returns a human-readable payment address given a ripemd160 hash
// and netID which encodes the zcash network and address type.  It is used
// in both pay-to-pubkey-hash (P2PKH) and pay-to-script-hash (P2SH) address
// encoding.
func encodeAddress(hash160 []byte, netID []byte) string {
	// Format is 2 bytes for a network and address class (i.e. P2PKH vs
	// P2SH), 20 bytes for a RIPEMD160 hash, and 4 bytes of checksum.
	return CheckEncode(hash160[:ripemd160.Size], netID)
}

// DecodeAddress decodes the string encoding of an address and returns
// the Address if addr is a valid encoding for a known address type.
//
// The zcash network the address is associated with is extracted if possible.
func DecodeAddress(addr string, defaultNet *chaincfg.Params) (btcutil.Address, error) {

	checkID, ok := NetIDs[defaultNet.Name]
	if !ok {
		return nil, errors.New("unknown network parameters")
	}

	// Switch on decoded length to determine the type.
	decoded, netID, err := CheckDecode(addr)
	if err != nil {
		if err == base58.ErrChecksum {
			return nil, ErrChecksumMismatch
		}
		return nil, errors.New("decoded address is of unknown format")
	}
	switch len(decoded) {
	case ripemd160.Size: // P2PKH or P2SH
		isP2PKH := bytes.Equal(netID, checkID.AddressPubKeyHash)
		isP2SH := bytes.Equal(netID, checkID.AddressScriptHash)
		switch hash160 := decoded; {
		case isP2PKH && isP2SH:
			return nil, ErrAddressCollision
		case isP2PKH:
			return newAddressPubKeyHash(hash160, defaultNet)
		case isP2SH:
			return newAddressScriptHashFromHash(hash160, defaultNet)
		default:
			return nil, ErrUnknownAddressType
		}

	default:
		return nil, errors.New("decoded address is of unknown size")
	}
}

// AddressPubKeyHash is an Address for a pay-to-pubkey-hash (P2PKH)
// transaction.
type AddressPubKeyHash struct {
	hash  [ripemd160.Size]byte
	netID []byte
}

// NewAddressPubKeyHash returns a new AddressPubKeyHash.  pkHash mustbe 20
// bytes.
func NewAddressPubKeyHash(pkHash []byte, net *chaincfg.Params) (*AddressPubKeyHash, error) {
	return newAddressPubKeyHash(pkHash, net)
}

// newAddressPubKeyHash is the internal API to create a pubkey hash address
// with a known leading identifier byte for a network, rather than looking
// it up through its parameters.  This is useful when creating a new address
// structure from a string encoding where the identifer byte is already
// known.
func newAddressPubKeyHash(pkHash []byte, net *chaincfg.Params) (*AddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, errors.New("pkHash must be 20 bytes")
	}

	netID, ok := NetIDs[net.Name]
	if !ok {
		return nil, errors.New("unknown network parameters")
	}

	addr := &AddressPubKeyHash{netID: netID.AddressPubKeyHash}
	copy(addr.hash[:], pkHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-pubkey-hash
// address.  Part of the Address interface.
func (a *AddressPubKeyHash) EncodeAddress() string {
	return encodeAddress(a.hash[:], a.netID)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a pubkey hash.  Part of the Address interface.
func (a *AddressPubKeyHash) ScriptAddress() []byte {
	return a.hash[:]
}

// IsForNet returns whether or not the pay-to-pubkey-hash address is associated
// with the passed zcash network.
func (a *AddressPubKeyHash) IsForNet(net *chaincfg.Params) bool {
	checkID, ok := NetIDs[net.Name]
	if !ok {
		return false
	}
	return bytes.Equal(a.netID, checkID.AddressPubKeyHash)
}

// String returns a human-readable string for the pay-to-pubkey-hash address.
// This is equivalent to calling EncodeAddress, but is provided so the type can
// be used as a fmt.Stringer.
func (a *AddressPubKeyHash) String() string {
	return a.EncodeAddress()
}

// Hash160 returns the underlying array of the pubkey hash.  This can be useful
// when an array is more appropiate than a slice (for example, when used as map
// keys).
func (a *AddressPubKeyHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

// AddressScriptHash is an Address for a pay-to-script-hash (P2SH)
// transaction.
type AddressScriptHash struct {
	hash  [ripemd160.Size]byte
	netID []byte
}

// NewAddressScriptHash returns a new AddressScriptHash.
func NewAddressScriptHash(serializedScript []byte, net *chaincfg.Params) (*AddressScriptHash, error) {
	scriptHash := btcutil.Hash160(serializedScript)
	return newAddressScriptHashFromHash(scriptHash, net)
}

// NewAddressScriptHashFromHash returns a new AddressScriptHash.  scriptHash
// must be 20 bytes.
func NewAddressScriptHashFromHash(scriptHash []byte, net *chaincfg.Params) (*AddressScriptHash, error) {
	return newAddressScriptHashFromHash(scriptHash, net)
}

// newAddressScriptHashFromHash is the internal API to create a script hash
// address with a known leading identifier byte for a network, rather than
// looking it up through its parameters.  This is useful when creating a new
// address structure from a string encoding where the identifer byte is already
// known.
func newAddressScriptHashFromHash(scriptHash []byte, net *chaincfg.Params) (*AddressScriptHash, error) {
	// Check for a valid script hash length.
	if len(scriptHash) != ripemd160.Size {
		return nil, errors.New("scriptHash must be 20 bytes")
	}

	netID, ok := NetIDs[net.Name]
	if !ok {
		return nil, errors.New("unknown network parameters")
	}

	addr := &AddressScriptHash{netID: netID.AddressScriptHash}
	copy(addr.hash[:], scriptHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-script-hash
// address.  Part of the Address interface.
func (a *AddressScriptHash) EncodeAddress() string {
	return encodeAddress(a.hash[:], a.netID)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a script hash.  Part of the Address interface.
func (a *AddressScriptHash) ScriptAddress() []byte {
	return a.hash[:]
}

// IsForNet returns whether or not the pay-to-script-hash address is associated
// with the passed zcash network.
func (a *AddressScriptHash) IsForNet(net *chaincfg.Params) bool {
	checkID, ok := NetIDs[net.Name]
	if !ok {
		return false
	}
	return bytes.Equal(a.netID, checkID.AddressScriptHash)
}

// String returns a human-readable string for the pay-to-script-hash address.
// This is equivalent to calling EncodeAddress, but is provided so the type can
// be used as a fmt.Stringer.
func (a *AddressScriptHash) String() string {
	return a.EncodeAddress()
}

// Hash160 returns the underlying array of the script hash.  This can be useful
// when an array is more appropiate than a slice (for example, when used as map
// keys).
func (a *AddressScriptHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

// PayToAddrScript creates a new script to pay a transaction output to a the
// specified address.
func PayToAddrScript(addr btcutil.Address) ([]byte, error) {
	const nilAddrErrStr = "unable to generate payment script for nil address"

	switch addr := addr.(type) {
	case *AddressPubKeyHash:
		if addr == nil {
			return nil, errors.New(nilAddrErrStr)
		}
		return payToPubKeyHashScript(addr.ScriptAddress())

	case *AddressScriptHash:
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
		return NewAddressScriptHashFromHash(pkScript[2:22], chainParams)
	} else if len(pkScript) == 1+1+1+20+1+1 && pkScript[0] == 0x76 && pkScript[1] == 0xa9 && pkScript[2] == 0x14 && pkScript[23] == 0x88 && pkScript[24] == 0xac {
		return NewAddressPubKeyHash(pkScript[3:23], chainParams)
	}
	return nil, errors.New("unknown script type")
}
