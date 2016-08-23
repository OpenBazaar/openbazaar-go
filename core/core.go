package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/repo"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/routing/dht"
	"github.com/op/go-logging"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/net/context"
	"gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	"io"
	"path"
)

var log = logging.MustGetLogger("core")

var salt = []byte("salt")
var encVersion = make([]byte, 4)

var Node *OpenBazaarNode

var inflightPublishRequests int

type OpenBazaarNode struct {
	// Context for issuing IPFS commands
	Context commands.Context

	// IPFS node object
	IpfsNode *core.IpfsNode

	// The roothash of the node directory inside the openbazaar repo.
	// This directory hash is published on IPNS at our peer ID making
	// the directory publicly viewable on the network.
	RootHash string

	// The path to the openbazaar repo in the file system.
	RepoPath string

	// The OpenBazaar network service for direct communication between peers
	Service net.NetworkService

	// Database for storing node specific data
	Datastore repo.Datastore

	// Websocket channel used for pushing data to the UI.
	Broadcast chan []byte

	// Bitcoin wallet implementation
	Wallet bitcoin.BitcoinWallet

	// Storage for our outgoing messages
	MessageStorage sto.OfflineMessagingStorage

	// A service that periodically checks the dht for outstanding messages
	MessageRetriever *net.MessageRetriever

	// A service that periodically republishes active pointers
	PointerRepublisher *net.PointerRepublisher

	// Used to resolve blockchainIDs to OpenBazaar IDs
	Resolver *bstk.BlockstackClient

	// A service that periodically fetches and caches the bitcoin exchange rates
	ExchangeRates bitcoin.ExchangeRates
}

// Unpin the current node repo, re-add it, then publish to ipns
func (n *OpenBazaarNode) SeedNode() error {
	hash, aerr := ipfs.AddDirectory(n.Context, path.Join(n.RepoPath, "root"))
	if aerr != nil {
		return aerr
	}
	go n.publish(hash)
	return nil
}

func (n *OpenBazaarNode) publish(hash string) {
	if inflightPublishRequests == 0 {
		n.Broadcast <- []byte(`{"status": "publishing"}`)
	}
	var err, perr error
	inflightPublishRequests++
	_, err = ipfs.Publish(n.Context, hash)
	if hash != n.RootHash {
		perr = ipfs.UnPinDir(n.Context, n.RootHash)
		n.RootHash = hash
	}
	inflightPublishRequests--
	if inflightPublishRequests == 0 {
		if err != nil || perr != nil {
			log.Error(err, perr)
			n.Broadcast <- []byte(`{"status": "error publishing"}`)
		} else {
			n.Broadcast <- []byte(`{"status": "publish complete"}`)
		}
	}
}

// This is a placeholder until the libsignal is operational
// For now we will just encrypt outgoing offline messages with the long lived identity key.
func (n *OpenBazaarNode) EncryptMessage(peerId peer.ID, message []byte) (ct []byte, rerr error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pubKey, err := n.IpfsNode.Routing.(*dht.IpfsDHT).GetPublicKey(ctx, peerId)
	if err != nil {
		log.Errorf("Failed to find public key for %s", peerId.Pretty())
		return nil, err
	}

	// Encrypt random aes key with RSA pubkey
	symmetricKey := make([]byte, 32)
	rand.Read(symmetricKey)

	encKey, err := pubKey.Encrypt(symmetricKey)
	if err != nil {
		return nil, err
	}

	// Generate mac and encryption keys
	hash := sha256.New

	hkdf := hkdf.New(hash, symmetricKey, salt, nil)

	aesKey := make([]byte, 32)
	_, err = io.ReadFull(hkdf, aesKey)
	if err != nil {
		return nil, err
	}
	macKey := make([]byte, 32)
	_, err = io.ReadFull(hkdf, macKey)
	if err != nil {
		return nil, err
	}

	// Encrypt message with aes key
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	ciphertext := make([]byte, aes.BlockSize+len(message))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], message)

	// Create the hmac
	mac := hmac.New(sha256.New, macKey)
	mac.Write(ciphertext)
	messageMac := mac.Sum(nil)

	// Prepend the ciphertext with the encrypted aes key
	ciphertext = append(encKey, ciphertext...)

	// Prepend version
	ciphertext = append(encVersion, ciphertext...)

	// Append the mac
	ciphertext = append(ciphertext, messageMac...)
	return ciphertext, nil
}
