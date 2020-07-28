package ffi

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterministicPrivateKeyGeneration(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < 10000; i++ {
		var xs [32]byte
		n, err := rand.Read(xs[:])
		require.NoError(t, err)
		require.Equal(t, len(xs), n)

		first := PrivateKeyGenerateWithSeed(xs)
		secnd := PrivateKeyGenerateWithSeed(xs)

		assert.Equal(t, first, secnd)
	}
}

func TestBLSSigningAndVerification(t *testing.T) {
	// generate private keys
	fooPrivateKey := PrivateKeyGenerate()
	barPrivateKey := PrivateKeyGenerate()

	// get the public keys for the private keys
	fooPublicKey := PrivateKeyPublicKey(fooPrivateKey)
	barPublicKey := PrivateKeyPublicKey(barPrivateKey)

	// make messages to sign with the keys
	fooMessage := Message("hello foo")
	barMessage := Message("hello bar!")

	// calculate the digests of the messages
	fooDigest := Hash(fooMessage)
	barDigest := Hash(barMessage)

	// get the signature when signing the messages with the private keys
	fooSignature := PrivateKeySign(fooPrivateKey, fooMessage)
	barSignature := PrivateKeySign(barPrivateKey, barMessage)

	// assert the foo message was signed with the foo key
	assert.True(t, Verify(fooSignature, []Digest{fooDigest}, []PublicKey{fooPublicKey}))

	// assert the bar message was signed with the bar key
	assert.True(t, Verify(barSignature, []Digest{barDigest}, []PublicKey{barPublicKey}))

	// assert the foo message was signed with the foo key
	assert.True(t, HashVerify(fooSignature, []Message{fooMessage}, []PublicKey{fooPublicKey}))

	// assert the bar message was signed with the bar key
	assert.True(t, HashVerify(barSignature, []Message{barMessage}, []PublicKey{barPublicKey}))

	// assert the foo message was not signed by the bar key
	assert.False(t, Verify(fooSignature, []Digest{fooDigest}, []PublicKey{barPublicKey}))

	// assert the bar/foo message was not signed by the foo/bar key
	assert.False(t, Verify(barSignature, []Digest{barDigest}, []PublicKey{fooPublicKey}))
	assert.False(t, Verify(barSignature, []Digest{fooDigest}, []PublicKey{barPublicKey}))
	assert.False(t, Verify(fooSignature, []Digest{barDigest}, []PublicKey{fooPublicKey}))
}

func BenchmarkBLSVerify(b *testing.B) {
	priv := PrivateKeyGenerate()

	msg := Message("this is a message that i will be signing")
	digest := Hash(msg)

	sig := PrivateKeySign(priv, msg)
	// fmt.Println("SIG SIZE: ", len(sig))
	// fmt.Println("SIG: ", sig)
	pubk := PrivateKeyPublicKey(priv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !Verify(sig, []Digest{digest}, []PublicKey{pubk}) {
			b.Fatal("failed to verify")
		}
	}
}

func TestBlsAggregateErrors(t *testing.T) {
	t.Run("no signatures", func(t *testing.T) {
		var empty []Signature
		out := Aggregate(empty)
		require.Nil(t, out)
	})

	t.Run("nil signatures", func(t *testing.T) {
		out := Aggregate(nil)
		require.Nil(t, out)
	})
}

func BenchmarkBLSVerifyBatch(b *testing.B) {
	b.Run("10", benchmarkBLSVerifyBatchSize(10))
	b.Run("50", benchmarkBLSVerifyBatchSize(50))
	b.Run("100", benchmarkBLSVerifyBatchSize(100))
	b.Run("300", benchmarkBLSVerifyBatchSize(300))
	b.Run("1000", benchmarkBLSVerifyBatchSize(1000))
	b.Run("4000", benchmarkBLSVerifyBatchSize(4000))
}

func benchmarkBLSVerifyBatchSize(size int) func(b *testing.B) {
	return func(b *testing.B) {
		var digests []Digest
		var msgs []Message
		var sigs []Signature
		var pubks []PublicKey
		for i := 0; i < size; i++ {
			msg := Message(fmt.Sprintf("cats cats cats cats %d %d %d dogs", i, i, i))
			msgs = append(msgs, msg)
			digests = append(digests, Hash(msg))
			priv := PrivateKeyGenerate()
			sig := PrivateKeySign(priv, msg)
			sigs = append(sigs, *sig)
			pubk := PrivateKeyPublicKey(priv)
			pubks = append(pubks, pubk)
		}

		t := time.Now()
		agsig := Aggregate(sigs)
		fmt.Println("Aggregate took: ", time.Since(t))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if !Verify(agsig, digests, pubks) {
				b.Fatal("failed to verify")
			}
		}
	}
}

func BenchmarkBLSHashAndVerify(b *testing.B) {
	priv := PrivateKeyGenerate()

	msg := Message("this is a message that i will be signing")
	sig := PrivateKeySign(priv, msg)

	// fmt.Println("SIG SIZE: ", len(sig))
	// fmt.Println("SIG: ", sig)
	pubk := PrivateKeyPublicKey(priv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		digest := Hash(msg)
		if !Verify(sig, []Digest{digest}, []PublicKey{pubk}) {
			b.Fatal("failed to verify")
		}
	}
}

func BenchmarkBLSHashVerify(b *testing.B) {
	priv := PrivateKeyGenerate()

	msg := Message("this is a message that i will be signing")
	sig := PrivateKeySign(priv, msg)

	// fmt.Println("SIG SIZE: ", len(sig))
	// fmt.Println("SIG: ", sig)
	pubk := PrivateKeyPublicKey(priv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !HashVerify(sig, []Message{msg}, []PublicKey{pubk}) {
			b.Fatal("failed to verify")
		}
	}
}
