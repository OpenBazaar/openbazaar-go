package core

import (
	"errors"
	routing "gx/ipfs/QmPR2JzfKd9poHx9XBhzoFeBBC31ZM3W5iUPKJZWyaoZZm/go-libp2p-routing"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	libp2p "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"path"
	"time"

	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/namesys"
	"github.com/OpenBazaar/openbazaar-go/net"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/repo"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/op/go-logging"
	"golang.org/x/net/context"
	"golang.org/x/net/proxy"
	"gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	ds "gx/ipfs/QmVSase1JP7cq9QkPT46oNwdp9pT6kBkG3oqS14y3QcZjG/go-datastore"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	"sync"
)

var (
	VERSION   = "0.11.1"
	USERAGENT = "/openbazaar-go:" + VERSION + "/"
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
	Broadcast chan interface{}

	// Bitcoin wallet implementation
	Wallet wallet.Wallet

	// Storage for our outgoing messages
	MessageStorage sto.OfflineMessagingStorage

	// A service that periodically checks the dht for outstanding messages
	MessageRetriever *ret.MessageRetriever

	// A service that periodically republishes active pointers
	PointerRepublisher *rep.PointerRepublisher

	// Used to resolve domains to OpenBazaar IDs
	NameSystem *namesys.NameSystem

	// A service that periodically fetches and caches the bitcoin exchange rates
	ExchangeRates bitcoin.ExchangeRates

	// Optional nodes to push user data to
	PushNodes []peer.ID

	// The user-agent for this node
	UserAgent string

	// A dialer for Tor if available
	TorDialer proxy.Dialer

	// Manage blocked peers
	BanManager *net.BanManager

	// Allow other nodes to push data to this node for storage
	AcceptStoreRequests bool

	// Last ditch API to find records that dropped out of the DHT
	IPNSBackupAPI string

	TestnetEnable        bool
	RegressionTestEnable bool
}

// Unpin the current node repo, re-add it, then publish to IPNS
var seedLock sync.Mutex
var PublishLock sync.Mutex
var InitalPublishComplete bool = false

// TestNetworkEnabled indicates whether the node is operating with test parameters
func (n *OpenBazaarNode) TestNetworkEnabled() bool { return n.TestnetEnable }

// RegressionNetworkEnabled indicates whether the node is operating with regression parameters
func (n *OpenBazaarNode) RegressionNetworkEnabled() bool { return n.RegressionTestEnable }

func (n *OpenBazaarNode) SeedNode() error {
	seedLock.Lock()
	ipfs.UnPinDir(n.Context, n.RootHash)
	var aerr error
	var rootHash string
	// There's an IPFS bug on Windows that might be related to the Windows indexer that could cause this to fail
	// If we fail the first time, let's retry a couple times before giving up.
	for i := 0; i < 3; i++ {
		rootHash, aerr = ipfs.AddDirectory(n.Context, path.Join(n.RepoPath, "root"))
		if aerr == nil {
			break
		}
		time.Sleep(time.Millisecond * 500)
	}
	if aerr != nil {
		seedLock.Unlock()
		return aerr
	}
	n.RootHash = rootHash
	seedLock.Unlock()
	InitalPublishComplete = true
	go n.publish(rootHash)
	return nil
}

func (n *OpenBazaarNode) publish(hash string) {
	// Multiple publishes may have been queued
	// We only need to publish the most recent
	PublishLock.Lock()
	defer PublishLock.Unlock()
	if hash != n.RootHash {
		return
	}

	if inflightPublishRequests == 0 {
		n.Broadcast <- notifications.StatusNotification{"publishing"}
	}

	id, err := cid.Decode(hash)
	if err != nil {
		log.Error(err)
		return
	}

	var graph []cid.Cid
	if len(n.PushNodes) > 0 {
		graph, err = ipfs.FetchGraph(n.IpfsNode.DAG, id)
		if err != nil {
			log.Error(err)
			return
		}
		pointers, err := n.Datastore.Pointers().GetByPurpose(ipfs.MESSAGE)
		if err != nil {
			log.Error(err)
			return
		}
		// Check if we're seeding any outgoing messages and add their CIDs to the graph
		for _, p := range pointers {
			if len(p.Value.Addrs) > 0 {
				s, err := p.Value.Addrs[0].ValueForProtocol(ma.P_IPFS)
				if err != nil {
					continue
				}
				c, err := cid.Decode(s)
				if err != nil {
					continue
				}
				graph = append(graph, *c)
			}
		}
	}
	for _, p := range n.PushNodes {
		go func(pid peer.ID) {
			err := n.SendStore(pid.Pretty(), graph)
			if err != nil {
				log.Errorf("Error pushing data to peer %s: %s", pid.Pretty(), err.Error())
			}
		}(p)
	}

	inflightPublishRequests++
	_, err = ipfs.Publish(n.Context, hash)

	inflightPublishRequests--
	if inflightPublishRequests == 0 {
		if err != nil {
			log.Error(err)
			n.Broadcast <- notifications.StatusNotification{"error publishing"}
		} else {
			n.Broadcast <- notifications.StatusNotification{"publish complete"}
		}
	}
}

func (n *OpenBazaarNode) SetUpRepublisher(interval time.Duration) {
	if interval == 0 {
		return
	}
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			n.UpdateFollow()
			n.SeedNode()
		}
	}()
}

/* This is a placeholder until the libsignal is operational.
   For now we will just encrypt outgoing offline messages with the long lived identity key.
   Optionally you may provide a public key, to avoid doing an IPFS lookup */
func (n *OpenBazaarNode) EncryptMessage(peerID peer.ID, peerKey *libp2p.PubKey, message []byte) (ct []byte, rerr error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if peerKey == nil {
		var pubKey libp2p.PubKey
		keyval, err := n.IpfsNode.Repo.Datastore().Get(ds.NewKey(KeyCachePrefix + peerID.String()))
		if err != nil {
			pubKey, err = routing.GetPublicKey(n.IpfsNode.Routing, ctx, []byte(peerID))
			if err != nil {
				log.Errorf("Failed to find public key for %s", peerID.Pretty())
				return nil, err
			}
		} else {
			pubKey, err = libp2p.UnmarshalPublicKey(keyval.([]byte))
			if err != nil {
				log.Errorf("Failed to find public key for %s", peerID.Pretty())
				return nil, err
			}
		}
		peerKey = &pubKey
	}
	if peerID.MatchesPublicKey(*peerKey) {
		ciphertext, err := net.Encrypt(*peerKey, message)
		if err != nil {
			return nil, err
		}
		return ciphertext, nil
	} else {
		log.Errorf("peer public key and id do not match for peer: %s", peerID.Pretty())
		return nil, errors.New("peer public key and id do not match")
	}
}

func (n *OpenBazaarNode) IPFSIdentityString() string {
	return n.IpfsNode.Identity.Pretty()
}
