package ipfs

import (
	"bytes"
	"encoding/hex"
	"github.com/tyler-smith/go-bip39"
	"gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	"testing"
)

var keyHex string = "080112605742f7c29729cc98dc62bce3104b5b0a1c6f390625cffef34bf2fd471f79ae3b693fe98c4b55f3ff8569e69b63c2ef7ee5e47d30822aed14305a6c5db48779c9693fe98c4b55f3ff8569e69b63c2ef7ee5e47d30822aed14305a6c5db48779c9"

func TestIdentityFromKey(t *testing.T) {
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Error(err)
	}
	identity, err := IdentityFromKey(keyBytes)
	if err != nil {
		t.Error(err)
	}
	if identity.PeerID != "QmRxqeKuCpx47FZ4CU5etgPMCPA4Y43feZabaRcHVmt8yf" {
		t.Error("Incorrect identity returned")
	}
	decodedKey, err := crypto.ConfigDecodeKey(identity.PrivKey)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(decodedKey, keyBytes) {
		t.Error("Incorrect private key returned")
	}
}

func TestIdentityKeyFromSeed(t *testing.T) {
	seed := bip39.NewSeed("mule track design catch stairs remain produce evidence cannon opera hamster burst", "Secret Passphrase")
	key, err := IdentityKeyFromSeed(seed, 4096)
	if err != nil {
		t.Error(err)
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(key, keyBytes) {
		t.Error("Failed to extract correct private key from seed")
	}
}

func TestDeterministicReader_Read(t *testing.T) {
	seed := bip39.NewSeed("mule track design catch stairs remain produce evidence cannon opera hamster burst", "Secret Passphrase")
	reader := &DeterministicReader{Seed: seed, Counter: 0}
	b := make([]byte, 32)
	reader.Read(b)
	if hex.EncodeToString(b) != "5742f7c29729cc98dc62bce3104b5b0a1c6f390625cffef34bf2fd471f79ae3b" {
		t.Error("Deterministic reader returned incorrect bytes")
	}
	if reader.Counter != 1 {
		t.Error("Deterministic Reader failed to increment counter")
	}
	b = make([]byte, 512)
	reader.Read(b)
	if hex.EncodeToString(b) != "ca55fa705e93030eb84041689294c9faae0d265ec1aec75c07bfaa3cb56a5a0a24f4c0cb50805fb46498e5e2adeb0e03f0e20dc3936dca0dc8fed0e0f1b48a1def01e07ba9fbcd54971bd95f11779a67633ce8bab5a809cc7780eea88f4dc42828bea89eb76e43d80b986a394f692dea9f6722f3834c6d680f9c50167339b98f510d246292afb93caa1bd6addd693c59c41a9629d09d4aa5935847c255eeb24c7a0610af5ec4c94abbbab92c27553245c3b41779033f139d396e69e748fb3d462c190fbbfbb85cda69a051c788fa0a3bd940a1c5abd15aa07102b17d2ec4ee0f227d32efa2253d88bef7f58520a23bc6b9144b051f3280f0b21fef3f968d22571d3e889f8514918be8b99e3111f524e5e99bc11584153ce56e76e4ff9de7d9b40a2851f2585a98201a80628919922973b5adff6468be4986ec745ad5dc53708b28862ce77fb52b2868641e5d18e767db8ba66e01e6a05975c882be9bf9c527e52a38186443c03655bdb4943e7b8cb0761a57e4bacd1bf247047863350e9f1cdc011303c3f7f61d9201a12a39b2c63c18693f867c1999a5c071fc8783a8dcf71480841e42559f5ae1f4ea13dfde95940dca0f99da8076b04211e938c97d77225f4de9c96c331019bc3ae40eb7f7d84053924aa3e00f4e3bf452ba7c2dacb56b2d9e7ee3b8582004d4c40a3b72c6741f200e11c61cbe00e29f941b6e90d3335535" {
		t.Error("Deterministic reader returned incorrect bytes")
	}
	if reader.Counter != 2 {
		t.Error("Deterministic Reader failed to increment counter")
	}
}
