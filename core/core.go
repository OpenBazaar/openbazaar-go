package core

import (
	"bytes"
	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/repo"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/routing/dht"
	"github.com/op/go-logging"
	"golang.org/x/net/context"
	"gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	"net/http"
	"net/url"
	"path"
	"time"
)

var log = logging.MustGetLogger("core")

var Node *OpenBazaarNode

var inflightPublishRequests int

type OpenBazaarNode struct {
	// Context for issuing IPFS commands
	Context commands.Context

	// IPFS node object
	IpfsNode *core.IpfsNode

	/* The roothash of the node directory inside the openbazaar repo.
	   This directory hash is published on IPNS at our peer ID making
	   the directory publicly viewable on the network. */
	RootHash string

	// The path to the openbazaar repo in the file system
	RepoPath string

	// The OpenBazaar network service for direct communication between peers
	Service net.NetworkService

	// Database for storing node specific data
	Datastore repo.Datastore

	// Websocket channel used for pushing data to the UI
	Broadcast chan []byte

	// Bitcoin wallet implementation
	Wallet bitcoin.BitcoinWallet

	// Storage for our outgoing messages
	MessageStorage sto.OfflineMessagingStorage

	// A service that periodically checks the dht for outstanding messages
	MessageRetriever *ret.MessageRetriever

	// A service that periodically republishes active pointers
	PointerRepublisher *rep.PointerRepublisher

	// Used to resolve blockchainIDs to OpenBazaar IDs
	Resolver *bstk.BlockstackClient

	// A service that periodically fetches and caches the bitcoin exchange rates
	ExchangeRates bitcoin.ExchangeRates

	// An optional gateway URL where we can crosspost data to ensure persistence
	CrosspostGateways []*url.URL
}

// Unpin the current node repo, re-add it, then publish to IPNS
func (n *OpenBazaarNode) SeedNode() error {
	rootHash, aerr := ipfs.AddDirectory(n.Context, path.Join(n.RepoPath, "root"))
	if aerr != nil {
		return aerr
	}
	for _, g := range n.CrosspostGateways {
		go func() {
			req, err := http.NewRequest("PUT", g.String()+path.Join("ipfs", rootHash), new(bytes.Buffer))
			if err != nil {
				return
			}
			var client = &http.Client{
				Timeout: time.Second * 10,
			}
			client.Do(req)
		}()
	}
	go n.publish(rootHash)
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

/* This is a placeholder until the libsignal is operational.
   For now we will just encrypt outgoing offline messages with the long lived identity key. */
func (n *OpenBazaarNode) EncryptMessage(peerId peer.ID, message []byte) (ct []byte, rerr error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pubKey, err := n.IpfsNode.Routing.(*dht.IpfsDHT).GetPublicKey(ctx, peerId)
	if err != nil {
		log.Errorf("Failed to find public key for %s", peerId.Pretty())
		return nil, err
	}
	ciphertext, err := net.Encrypt(pubKey, message)
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}
