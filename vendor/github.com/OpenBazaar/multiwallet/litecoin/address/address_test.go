package address

import (
	"github.com/btcsuite/btcd/chaincfg"
	"testing"
)

func TestDecodeLitecoinAddress(t *testing.T) {
	// Mainnet
	addr, err := DecodeAddress("ltc1qj065d66h5943s357vfd9kltn6k4atn3qwqy8frycnfcf4ycwhrtqr6496q", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "ltc1qj065d66h5943s357vfd9kltn6k4atn3qwqy8frycnfcf4ycwhrtqr6496q" {
		t.Error("Address decoding error")
	}
	addr1, err := DecodeAddress("LKxmT8iooGt2d9xQn1y8PU6KwW3J8EDQ9a", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	if addr1.String() != "LKxmT8iooGt2d9xQn1y8PU6KwW3J8EDQ9a" {
		t.Error("Address decoding error")
	}
	// Testnet
	addr, err = DecodeAddress("mjFBdzsYNBCeabLNwyYYCt8epG7GhzYeTw", &chaincfg.TestNet3Params)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "mjFBdzsYNBCeabLNwyYYCt8epG7GhzYeTw" {
		t.Error("Address decoding error")
	}

	// Testnet witness
	addr, err = DecodeAddress("tltc1qxjqda2dlef5250yqgdhyscj2n2sv98yt6f9ewzvrmt0v86xuefxs9xya9u", &chaincfg.TestNet3Params)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "tltc1qxjqda2dlef5250yqgdhyscj2n2sv98yt6f9ewzvrmt0v86xuefxs9xya9u" {
		t.Error("Address decoding error")
	}

}

var dataElement = []byte{203, 72, 18, 50, 41, 156, 213, 116, 49, 81, 172, 75, 45, 99, 174, 25, 142, 123, 176, 169}

// Second address of https://github.com/Bitcoin-UAHF/spec/blob/master/cashaddr.md#examples-of-address-translation
func TestAddressPubKeyHash_EncodeAddress(t *testing.T) {
	// Mainnet
	addr, err := NewAddressPubKeyHash(dataElement, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "LdkomjvYVsoY5DdZx3LJVd1dXRhpKc18Xa" {
		t.Error("Address decoding error")
	}
	// Testnet
	addr, err = NewAddressPubKeyHash(dataElement, &chaincfg.TestNet3Params)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "mz3ooahhEEzjbXR2VUKP3XACBCwF5zhQBy" {
		t.Error("Address decoding error")
	}
}

var dataElement2 = []byte{118, 160, 64, 83, 189, 160, 168, 139, 218, 81, 119, 184, 106, 21, 195, 178, 159, 85, 152, 115, 118, 160, 64, 83, 189, 160, 168, 139, 218, 81, 119, 184}

// 4th address of https://github.com/Bitcoin-UAHF/spec/blob/master/cashaddr.md#examples-of-address-translation
func TestWitnessScriptHash_EncodeAddress(t *testing.T) {
	// Mainnet
	addr, err := NewAddressWitnessScriptHash(dataElement2, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "ltc1qw6syq5aa5z5ghkj3w7ux59wrk204txrnw6syq5aa5z5ghkj3w7uqdjs2cd" {
		t.Error("Address decoding error")
	}
	// Testnet
	addr, err = NewAddressWitnessScriptHash(dataElement2, &chaincfg.TestNet3Params)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "tltc1qw6syq5aa5z5ghkj3w7ux59wrk204txrnw6syq5aa5z5ghkj3w7uqxa558c" {
		t.Error("Address decoding error")
	}
}

func TestScriptParsing(t *testing.T) {
	addr, err := DecodeAddress("ltc1qj065d66h5943s357vfd9kltn6k4atn3qwqy8frycnfcf4ycwhrtqr6496q", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	script, err := PayToAddrScript(addr)
	if err != nil {
		t.Error(err)
	}
	addr2, err := ExtractPkScriptAddrs(script, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != addr2.String() {
		t.Error("Failed to convert script back into address")
	}
}
