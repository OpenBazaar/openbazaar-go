package core

import (
	"errors"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/repo"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/op/go-logging"
	"gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	"path"
)

var log = logging.MustGetLogger("core")

var Node *OpenBazaarNode

type OpenBazaarNode struct {
	// Context for issuing IPFS commands
	Context commands.Context

	// IPFS node object
	IpfsNode *core.IpfsNode

	// The roothash of the node directory inside the openbazaar repo.
	// This directory hash is published on IPNS at our peer ID making
	// the directory publically viewable on the network.
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

	// TODO: Libsignal Client
	// TODO: Pointer Republisher
}

// Unpin the current node repo, re-add it, then publish to ipns
func (n *OpenBazaarNode) SeedNode() error {
	if err := ipfs.UnPinDir(n.Context, n.RootHash); err != nil {
		return err
	}
	hash, aerr := ipfs.AddDirectory(n.Context, path.Join(n.RepoPath, "root"))
	if aerr != nil {
		return aerr
	}
	_, perr := ipfs.Publish(n.Context, hash)
	if perr != nil {
		return perr
	}
	n.RootHash = hash
	return nil
}

// This is a placeholder until the libsignal is operational
// For now we will just encrypt outgoing offline messages with the long lived identity key.
func (n *OpenBazaarNode) EncryptMessage(peerId peer.ID, message []byte) (ct []byte, rerr error) {
	defer func() {
		if r := recover(); r != nil {
			ct = nil
			log.Warningf("Could not find public key for %s", peerId.Pretty())
			rerr = errors.New("Could not find public key for peer")
		}
	}()
	pubKey := n.IpfsNode.Peerstore.PubKey(peerId)
	ciphertext, err := pubKey.Encrypt(message)
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}
