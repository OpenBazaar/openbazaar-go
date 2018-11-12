package secio

import (
	"context"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"

	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
)

func NewTestSessionGenerator(typ, bits int, t *testing.T) SessionGenerator {
	sk, pk, err := ci.GenerateKeyPair(typ, bits)
	if err != nil {
		t.Fatal(err)
	}

	p, err := peer.IDFromPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}

	return SessionGenerator{
		LocalID:    p,
		PrivateKey: sk,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Create a new pair of connected TCP sockets.
func NewConnPair(t *testing.T) (client net.Conn, server net.Conn) {
	lstnr, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
		return
	}

	var client_err error
	addr := lstnr.Addr()
	done := make(chan struct{})

	go func() {
		defer close(done)
		client, client_err = net.Dial(addr.Network(), addr.String())
	}()

	server, err = lstnr.Accept()
	<-done

	lstnr.Close()

	if err != nil {
		t.Fatalf("Failed to accept: %v", err)
	}

	if client_err != nil {
		t.Fatalf("Failed to connect: %v", client_err)
	}

	return client, server
}

// Create a new pair of connected sessions based off of the provided
// session generators.
func NewTestSessionPair(client_sg, server_sg SessionGenerator,
	t *testing.T) (client_sess Session, server_sess Session) {
	var (
		err        error
		client_err error
	)

	client, server := NewConnPair(t)

	// Connect the client and server sessions
	done := make(chan struct{})

	go func() {
		defer close(done)
		client_sess, client_err = client_sg.NewSession(context.TODO(), client)
	}()

	server_sess, err = server_sg.NewSession(context.TODO(), server)
	<-done

	if err != nil {
		t.Fatal(err)
	}

	if client_err != nil {
		t.Fatal(client_err)
	}

	return
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
func testIDs(client_sg, server_sg SessionGenerator,
	client_sess, server_sess Session, t *testing.T) {
	if client_sess.LocalPeer() != client_sg.LocalID {
		t.Fatal("Client Local Peer ID mismatch.")
	}

	if client_sess.RemotePeer() != server_sg.LocalID {
		t.Fatal("Client Remote Peer ID mismatch.")
	}

	if client_sess.LocalPeer() != server_sess.RemotePeer() {
		t.Fatal("Server Local Peer ID mismatch.")
	}
}

// Check the keys
func testKeys(client_sg, server_sg SessionGenerator,
	client_sess, server_sess Session, t *testing.T) {
	sk := server_sess.LocalPrivateKey()
	pk := sk.GetPublic()

	if !sk.Equals(server_sg.PrivateKey) {
		t.Error("Private key Mismatch.")
	}

	if !pk.Equals(client_sess.RemotePublicKey()) {
		t.Error("Public key mismatch.")
	}
}

// Check sending and receiving messages
func testReadWrite(client_sess, server_sess Session, t *testing.T) {
	client_rwc := client_sess.ReadWriter()
	server_rwc := server_sess.ReadWriter()

	before := []byte("hello world")
	err := client_rwc.WriteMsg(before)
	if err != nil {
		t.Fatal(err)
	}

	after, err := server_rwc.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}

	if string(before) != string(after) {
		t.Errorf("Message mismatch. %v != %v", before, after)
	}
}

// Setup a new session with a pair of locally connected sockets
func testSession(client_sg, server_sg SessionGenerator, t *testing.T) {
	client_sess, server_sess := NewTestSessionPair(client_sg, server_sg, t)

	testIDs(client_sg, server_sg, client_sess, server_sess, t)
	testKeys(client_sg, server_sg, client_sess, server_sess, t)
	testReadWrite(client_sess, server_sess, t)

	client_sess.Close()
	server_sess.Close()
}

// Run a set of sessions through the session setup and verification.
func TestSessions(t *testing.T) {
	client_sg := NewTestSessionGenerator(ci.RSA, 1024, t)
	server_sg := NewTestSessionGenerator(ci.Ed25519, 1024, t)

	t.Logf("Using default session parameters.")
	testSession(client_sg, server_sg, t)

	defer resetSessionParams()
	test_params := getMinimalSessionParams()
	for _, params := range test_params {
		SupportedExchanges = params.Exchange
		SupportedCiphers = params.Cipher
		SupportedHashes = params.Hash

		t.Logf("Using Exchange: %s Cipher: %s Hash: %s\n",
			params.Exchange, params.Cipher, params.Hash)
		testSession(client_sg, server_sg, t)
	}
}
