package address

import (
	"github.com/btcsuite/btcd/chaincfg"
	"testing"
)

func TestDecodeZcashAddress(t *testing.T) {
	// Mainnet
	addr, err := DecodeAddress("t1cQTWs2rPYM5R3zJiLA8MR3nZsXd1p2U6Q", &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "t1cQTWs2rPYM5R3zJiLA8MR3nZsXd1p2U6Q" {
		t.Error("Address decoding error")
	}
	// Testnet
	addr, err = DecodeAddress("tmUFCqhXFnCraZJBkP4TsD5iYArcSWSmgkT", &chaincfg.TestNet3Params)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "tmUFCqhXFnCraZJBkP4TsD5iYArcSWSmgkT" {
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
	if addr.String() != "t1cQTWs2rPYM5R3zJiLA8MR3nZsXd1p2U6Q" {
		t.Error("Address decoding error")
	}
	// Testnet
	addr, err = NewAddressPubKeyHash(dataElement, &chaincfg.TestNet3Params)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "tmUFCqhXFnCraZJBkP4TsD5iYArcSWSmgkT" {
		t.Error("Address decoding error")
	}
}

var dataElement2 = []byte{118, 160, 64, 83, 189, 160, 168, 139, 218, 81, 119, 184, 106, 21, 195, 178, 159, 85, 152, 115}

// 4th address of https://github.com/Bitcoin-UAHF/spec/blob/master/cashaddr.md#examples-of-address-translation
func TestCashWitnessScriptHash_EncodeAddress(t *testing.T) {
	// Mainnet
	addr, err := NewAddressScriptHashFromHash(dataElement2, &chaincfg.MainNetParams)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "t3VNrdy8EjPaEJv2DnRN414eVwVQR9M8iS3" {
		t.Error("Address decoding error")
	}
	// Testnet
	addr, err = NewAddressScriptHashFromHash(dataElement2, &chaincfg.TestNet3Params)
	if err != nil {
		t.Error(err)
	}
	if addr.String() != "t2HN3geENbrBbrcbxiAN6Ygq93ydayzuTqB" {
		t.Error("Address decoding error")
	}
}

func TestScriptParsing(t *testing.T) {
	addr, err := DecodeAddress("t3VNrdy8EjPaEJv2DnRN414eVwVQR9M8iS3", &chaincfg.MainNetParams)
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
