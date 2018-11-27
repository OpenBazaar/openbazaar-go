package secio

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"

	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	cs "gx/ipfs/QmZ3XKH272gU9px86XqWYeZHU65ayHxWs6Wbswvdj2VqVK/go-conn-security"
	cst "gx/ipfs/QmZ3XKH272gU9px86XqWYeZHU65ayHxWs6Wbswvdj2VqVK/go-conn-security/test"
)

func newTestTransport(t *testing.T, typ, bits int) *Transport {
	priv, pub, err := ci.GenerateKeyPair(typ, bits)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return &Transport{
		PrivateKey: priv,
		LocalID:    id,
	}
}

func TestTransport(t *testing.T) {
	at := newTestTransport(t, ci.RSA, 2048)
	bt := newTestTransport(t, ci.RSA, 2048)
	cst.SubtestAll(t, at, bt, at.LocalID, bt.LocalID)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Create a new pair of connected TCP sockets.
func newConnPair(t *testing.T) (net.Conn, net.Conn) {
	lstnr, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
		return nil, nil
	}

	var clientErr error
	var client net.Conn
	addr := lstnr.Addr()
	done := make(chan struct{})

	go func() {
		defer close(done)
		client, clientErr = net.Dial(addr.Network(), addr.String())
	}()

	server, err := lstnr.Accept()
	<-done

	lstnr.Close()

	if err != nil {
		t.Fatalf("Failed to accept: %v", err)
	}

	if clientErr != nil {
		t.Fatalf("Failed to connect: %v", clientErr)
	}

	return client, server
}

// Create a new pair of connected sessions based off of the provided
// session generators.
func connect(t *testing.T, clientTpt, serverTpt *Transport) (cs.Conn, cs.Conn) {
	client, server := newConnPair(t)

	// Connect the client and server sessions
	done := make(chan struct{})

	var clientConn cs.Conn
	var clientErr error
	go func() {
		defer close(done)
		clientConn, clientErr = clientTpt.SecureOutbound(context.TODO(), client, serverTpt.LocalID)
	}()

	serverConn, serverErr := serverTpt.SecureInbound(context.TODO(), server)
	<-done

	if serverErr != nil {
		t.Fatal(serverErr)
	}

	if clientErr != nil {
		t.Fatal(clientErr)
	}

	return clientConn, serverConn
}

// Shuffle a slice of strings
func shuffle(strs []string) []string {
	for i := len(strs) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		strs[i], strs[j] = strs[j], strs[i]
	}
	return strs
}

type sessionParam struct {
	Exchange string
	Cipher   string
	Hash     string
}

// Reset the global session parameters to the defaults
func resetSessionParams() {
	SupportedExchanges = DefaultSupportedExchanges
	SupportedCiphers = DefaultSupportedCiphers
	SupportedHashes = DefaultSupportedHashes
}

// Get the minimal set of session parameters we should test.
//
// We'll try each exchange, cipher, and hash at least once. The combination
// with other parameters is randomized.
func getMinimalSessionParams() []sessionParam {
	params := []sessionParam{}

	rand.Seed(time.Now().UnixNano())

	exchanges := shuffle(strings.Split(DefaultSupportedExchanges, ","))
	ciphers := shuffle(strings.Split(DefaultSupportedCiphers, ","))
	hashes := shuffle(strings.Split(DefaultSupportedHashes, ","))

	m := max(len(exchanges), max(len(ciphers), len(hashes)))
	for i := 0; i < m; i++ {
		param := sessionParam{
			Exchange: exchanges[i%len(exchanges)],
			Cipher:   ciphers[i%len(ciphers)],
			Hash:     hashes[i%len(hashes)],
		}

		params = append(params, param)
	}

	return params
}

// Get all of the combinations of session parameters possible
func getFullSessionParams() []sessionParam {
	params := []sessionParam{}

	exchanges := strings.Split(DefaultSupportedExchanges, ",")
	ciphers := strings.Split(DefaultSupportedCiphers, ",")
	hashes := strings.Split(DefaultSupportedHashes, ",")

	for _, exchange := range exchanges {
		for _, cipher := range ciphers {
			for _, hash := range hashes {
				param := sessionParam{
					Exchange: exchange,
					Cipher:   cipher,
					Hash:     hash,
				}
				params = append(params, param)
			}
		}
	}
	return params
}

// Check the peer IDs
func testIDs(t *testing.T, clientTpt, serverTpt *Transport, clientConn, serverConn cs.Conn) {
	if clientConn.LocalPeer() != clientTpt.LocalID {
		t.Fatal("Client Local Peer ID mismatch.")
	}

	if clientConn.RemotePeer() != serverTpt.LocalID {
		t.Fatal("Client Remote Peer ID mismatch.")
	}

	if clientConn.LocalPeer() != serverConn.RemotePeer() {
		t.Fatal("Server Local Peer ID mismatch.")
	}
}

// Check the keys
func testKeys(t *testing.T, clientTpt, serverTpt *Transport, clientConn, serverConn cs.Conn) {
	sk := serverConn.LocalPrivateKey()
	pk := sk.GetPublic()

	if !sk.Equals(serverTpt.PrivateKey) {
		t.Error("Private key Mismatch.")
	}

	if !pk.Equals(clientConn.RemotePublicKey()) {
		t.Error("Public key mismatch.")
	}
}

// Check sending and receiving messages
func testReadWrite(t *testing.T, clientConn, serverConn cs.Conn) {
	before := []byte("hello world")
	_, err := clientConn.Write(before)
	if err != nil {
		t.Fatal(err)
	}

	after := make([]byte, len(before))
	_, err = io.ReadFull(serverConn, after)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(before, after) {
		t.Errorf("Message mismatch. %v != %v", before, after)
	}
}

// Setup a new session with a pair of locally connected sockets
func testConnection(t *testing.T, clientTpt, serverTpt *Transport) {
	clientConn, serverConn := connect(t, clientTpt, serverTpt)

	testIDs(t, clientTpt, serverTpt, clientConn, serverConn)
	testKeys(t, clientTpt, serverTpt, clientConn, serverConn)
	testReadWrite(t, clientConn, serverConn)

	clientConn.Close()
	serverConn.Close()
}

// Run a set of sessions through the session setup and verification.
func TestConnections(t *testing.T) {
	clientTpt := newTestTransport(t, ci.RSA, 1024)
	serverTpt := newTestTransport(t, ci.Ed25519, 1024)

	t.Logf("Using default session parameters.")
	testConnection(t, clientTpt, serverTpt)

	defer resetSessionParams()
	testParams := getMinimalSessionParams()
	for _, params := range testParams {
		SupportedExchanges = params.Exchange
		SupportedCiphers = params.Cipher
		SupportedHashes = params.Hash

		t.Logf("Using Exchange: %s Cipher: %s Hash: %s\n",
			params.Exchange, params.Cipher, params.Hash)
		testConnection(t, clientTpt, serverTpt)
	}
}
