// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bchutil

import (
	"errors"

	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

var (
	bitpayP2PkH = byte(0x1C)
	bitpayP2SH  = byte(0x28)
)

// UnsupportedWitnessVerError describes an error where a segwit address being
// decoded has an unsupported witness version.
type UnsupportedWitnessVerError byte

func (e UnsupportedWitnessVerError) Error() string {
	return "unsupported witness version: " + string(e)
}

// UnsupportedWitnessProgLenError describes an error where a segwit address
// being decoded has an unsupported witness program length.
type UnsupportedWitnessProgLenError int

func (e UnsupportedWitnessProgLenError) Error() string {
	return "unsupported witness program length: " + string(e)
}

// encodeAddress returns a human-readable payment address given a ripemd160 hash
// and netID which encodes the bitcoin network and address type.  It is used
// in both pay-to-pubkey-hash (P2PKH) and pay-to-script-hash (P2SH) address
// encoding.
func encodeBitpayAddress(hash160 []byte, netID byte) string {
	// Format is 1 byte for a network and address class (i.e. P2PKH vs
	// P2SH), 20 bytes for a RIPEMD160 hash, and 4 bytes of checksum.
	return base58.CheckEncode(hash160[:ripemd160.Size], netID)
}

// DecodeAddress decodes the string encoding of an address and returns
// the Address if addr is a valid encoding for a known address type.
//
// The bitcoin network the address is associated with is extracted if possible.
// When the address does not encode the network, such as in the case of a raw
// public key, the address will be associated with the passed defaultNet.
func DecodeBitpay(addr string, defaultNet *chaincfg.Params) (btcutil.Address, error) {

	// Switch on decoded length to determine the type.
	decoded, netID, err := base58.CheckDecode(addr)
	if err != nil {
		if err == base58.ErrChecksum {
			return nil, ErrChecksumMismatch
		}
		return nil, errors.New("decoded address is of unknown format")
	}
	switch len(decoded) {
	case ripemd160.Size: // P2PKH or P2SH
		isP2PKH := chaincfg.IsPubKeyHashAddrID(netID) || netID == bitpayP2PkH
		isP2SH := chaincfg.IsScriptHashAddrID(netID) || netID == bitpayP2SH
		switch hash160 := decoded; {
		case isP2PKH && isP2SH:
			return nil, ErrAddressCollision
		case isP2PKH:
			return newBitpayAddressPubKeyHash(hash160, netID)
		case isP2SH:
			return newBitpayAddressScriptHashFromHash(hash160, netID)
		default:
			return nil, ErrUnknownAddressType
		}

	default:
		return nil, errors.New("decoded address is of unknown size")
	}
}

// AddressPubKeyHash is an Address for a pay-to-pubkey-hash (P2PKH)
// transaction.
type BitpayAddressPubKeyHash struct {
	hash  [ripemd160.Size]byte
	netID byte
}

// NewAddressPubKeyHash returns a new AddressPubKeyHash.  pkHash mustbe 20
// bytes.
func NewBitpayAddressPubKeyHash(pkHash []byte, net *chaincfg.Params) (*BitpayAddressPubKeyHash, error) {
	var v byte
	if net.Name == chaincfg.MainNetParams.Name {
		v = bitpayP2PkH
	} else {
		v = net.PubKeyHashAddrID
	}
	return newBitpayAddressPubKeyHash(pkHash, v)
}

// newAddressPubKeyHash is the internal API to create a pubkey hash address
// with a known leading identifier byte for a network, rather than looking
// it up through its parameters.  This is useful when creating a new address
// structure from a string encoding where the identifer byte is already
// known.
func newBitpayAddressPubKeyHash(pkHash []byte, netID byte) (*BitpayAddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, errors.New("pkHash must be 20 bytes")
	}

	addr := &BitpayAddressPubKeyHash{netID: netID}
	copy(addr.hash[:], pkHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-pubkey-hash
// address.  Part of the Address interface.
func (a *BitpayAddressPubKeyHash) EncodeAddress() string {
	return encodeBitpayAddress(a.hash[:], a.netID)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a pubkey hash.  Part of the Address interface.
func (a *BitpayAddressPubKeyHash) ScriptAddress() []byte {
	return a.hash[:]
}

// IsForNet returns whether or not the pay-to-pubkey-hash address is associated
// with the passed bitcoin network.
func (a *BitpayAddressPubKeyHash) IsForNet(net *chaincfg.Params) bool {
	return a.netID == net.PubKeyHashAddrID
}

// String returns a human-readable string for the pay-to-pubkey-hash address.
// This is equivalent to calling EncodeAddress, but is provided so the type can
// be used as a fmt.Stringer.
func (a *BitpayAddressPubKeyHash) String() string {
	return a.EncodeAddress()
}

// Hash160 returns the underlying array of the pubkey hash.  This can be useful
// when an array is more appropiate than a slice (for example, when used as map
// keys).
func (a *BitpayAddressPubKeyHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

// AddressScriptHash is an Address for a pay-to-script-hash (P2SH)
// transaction.
type BitpayAddressScriptHash struct {
	hash  [ripemd160.Size]byte
	netID byte
}

// NewAddressScriptHash returns a new AddressScriptHash.
func NewBitpayAddressScriptHash(serializedScript []byte, net *chaincfg.Params) (*BitpayAddressScriptHash, error) {
	scriptHash := btcutil.Hash160(serializedScript)
	var v byte
	if net.Name == chaincfg.MainNetParams.Name {
		v = bitpayP2SH
	} else {
		v = net.ScriptHashAddrID
	}
	return newBitpayAddressScriptHashFromHash(scriptHash, v)
}

// NewAddressScriptHashFromHash returns a new AddressScriptHash.  scriptHash
// must be 20 bytes.
func NewBitpayAddressScriptHashFromHash(scriptHash []byte, net *chaincfg.Params) (*BitpayAddressScriptHash, error) {
	var v byte
	if net.Name == chaincfg.MainNetParams.Name {
		v = bitpayP2SH
	} else {
		v = net.ScriptHashAddrID
	}
	return newBitpayAddressScriptHashFromHash(scriptHash, v)
}

// newAddressScriptHashFromHash is the internal API to create a script hash
// address with a known leading identifier byte for a network, rather than
// looking it up through its parameters.  This is useful when creating a new
// address structure from a string encoding where the identifer byte is already
// known.
func newBitpayAddressScriptHashFromHash(scriptHash []byte, netID byte) (*BitpayAddressScriptHash, error) {
	// Check for a valid script hash length.
	if len(scriptHash) != ripemd160.Size {
		return nil, errors.New("scriptHash must be 20 bytes")
	}

	addr := &BitpayAddressScriptHash{netID: netID}
	copy(addr.hash[:], scriptHash)
	return addr, nil
}

// EncodeAddress returns the string encoding of a pay-to-script-hash
// address.  Part of the Address interface.
func (a *BitpayAddressScriptHash) EncodeAddress() string {
	return encodeBitpayAddress(a.hash[:], a.netID)
}

// ScriptAddress returns the bytes to be included in a txout script to pay
// to a script hash.  Part of the Address interface.
func (a *BitpayAddressScriptHash) ScriptAddress() []byte {
	return a.hash[:]
}

// IsForNet returns whether or not the pay-to-script-hash address is associated
// with the passed bitcoin network.
func (a *BitpayAddressScriptHash) IsForNet(net *chaincfg.Params) bool {
	return a.netID == net.ScriptHashAddrID
}

// String returns a human-readable string for the pay-to-script-hash address.
// This is equivalent to calling EncodeAddress, but is provided so the type can
// be used as a fmt.Stringer.
func (a *BitpayAddressScriptHash) String() string {
	return a.EncodeAddress()
}

// Hash160 returns the underlying array of the script hash.  This can be useful
// when an array is more appropiate than a slice (for example, when used as map
// keys).
func (a *BitpayAddressScriptHash) Hash160() *[ripemd160.Size]byte {
	return &a.hash
}

// PayToAddrScript creates a new script to pay a transaction output to a the
// specified address.
func bitpayPayToAddrScript(addr btcutil.Address) ([]byte, error) {
	const nilAddrErrStr = "unable to generate payment script for nil address"

	switch addr := addr.(type) {
	case *BitpayAddressPubKeyHash:
		if addr == nil {
			return nil, errors.New(nilAddrErrStr)
		}
		return payToPubKeyHashScript(addr.ScriptAddress())

	case *BitpayAddressScriptHash:
		if addr == nil {
			return nil, errors.New(nilAddrErrStr)
		}
		return payToScriptHashScript(addr.ScriptAddress())
	}
	return nil, fmt.Errorf("unable to generate payment script for unsupported "+
		"address type %T", addr)
}
