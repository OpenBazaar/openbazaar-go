package core

import (
	"errors"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"gx/ipfs/QmPpYHPRGVpSJTkQDQDwTYZ1cYUR2NM4HS6M3iAXi8aoUa/go-libp2p-kad-dht"
	libp2p "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	"gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	routing "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing"

	"path"
	"sync"
	"time"

	"github.com/OpenBazaar/multiwallet"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/repo"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/gosimple/slug"
	"github.com/ipfs/go-ipfs/core"
	logging "github.com/op/go-logging"
	"golang.org/x/net/context"
	"golang.org/x/net/proxy"
)

const (
	// VERSION - current version
	VERSION = "0.13.1"
	// USERAGENT - user-agent header string
	USERAGENT = "/openbazaar-go:" + VERSION + "/"
)

var log = logging.MustGetLogger("core")

// Node - ob node
var Node *OpenBazaarNode

var inflightPublishRequests int

// OpenBazaarNode - represent ob node which encapsulates ipfsnode, wallet etc
type OpenBazaarNode struct {
	// IPFS node object
	IpfsNode *core.IpfsNode

	// An implementation of the custom DHT used by OpenBazaar
	DHT *dht.IpfsDHT

	// The roothash of the node directory inside the openbazaar repo.
	// This directory hash is published on IPNS at our peer ID making
	// the directory publicly viewable on the network.
	RootHash string

	// The path to the openbazaar repo in the file system
	RepoPath string

	// The OpenBazaar network service for direct communication between peers
	Service net.NetworkService

	// Database for storing node specific data
	Datastore repo.Datastore

	// Websocket channel used for pushing data to the UI
	Broadcast chan repo.Notifier

	// A map of cryptocurrency wallets
	Multiwallet multiwallet.MultiWallet

	// Storage for our outgoing messages
	MessageStorage sto.OfflineMessagingStorage

	// A service that periodically checks the dht for outstanding messages
	MessageRetriever *ret.MessageRetriever

	// OfflineMessageFailoverTimeout is the duration until the protocol
	// will stop looking for the peer to send a direct message and failover to
	// sending an offline message
	OfflineMessageFailoverTimeout time.Duration

	// A service that periodically republishes active pointers
	PointerRepublisher *rep.PointerRepublisher

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

	// RecordAgingNotifier is a worker that walks the cases datastore to
	// notify the user as disputes age past certain thresholds
	RecordAgingNotifier *recordAgingNotifier

	// Generic pubsub interface
	Pubsub ipfs.Pubsub

	// The master private key derived from the mnemonic
	MasterPrivateKey *hdkeychain.ExtendedKey

	// The number of DHT records to collect before returning. The larger the number
	// the slower the query but the less likely we will get an old record.
	IPNSQuorumSize uint

	TestnetEnable        bool
	RegressionTestEnable bool
}

// PublishLock seedLock - Unpin the current node repo, re-add it, then publish to IPNS
var PublishLock sync.Mutex
var seedLock sync.Mutex

// InitalPublishComplete - indicate publish completion
var InitalPublishComplete bool // = false

// TestNetworkEnabled indicates whether the node is operating with test parameters
func (n *OpenBazaarNode) TestNetworkEnabled() bool { return n.TestnetEnable }

// RegressionNetworkEnabled indicates whether the node is operating with regression parameters
func (n *OpenBazaarNode) RegressionNetworkEnabled() bool { return n.RegressionTestEnable }

// SeedNode - publish to IPNS
func (n *OpenBazaarNode) SeedNode() error {
	seedLock.Lock()
	ipfs.UnPinDir(n.IpfsNode, n.RootHash)
	var aerr error
	var rootHash string
	// There's an IPFS bug on Windows that might be related to the Windows indexer that could cause this to fail
	// If we fail the first time, let's retry a couple times before giving up.
	for i := 0; i < 3; i++ {
		rootHash, aerr = ipfs.AddDirectory(n.IpfsNode, path.Join(n.RepoPath, "root"))
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
		n.Broadcast <- repo.StatusNotification{Status: "publishing"}
	}

	err := n.sendToPushNodes(hash)
	if err != nil {
		log.Error(err)
		return
	}

	inflightPublishRequests++
	err = ipfs.Publish(n.IpfsNode, hash)

	inflightPublishRequests--
	if inflightPublishRequests == 0 {
		if err != nil {
			log.Error(err)
			n.Broadcast <- repo.StatusNotification{Status: "error publishing"}
		} else {
			n.Broadcast <- repo.StatusNotification{Status: "publish complete"}
		}
	}
}

func (n *OpenBazaarNode) sendToPushNodes(hash string) error {
	id, err := cid.Decode(hash)
	if err != nil {
		return err
	}

	var graph []cid.Cid
	if len(n.PushNodes) > 0 {
		graph, err = ipfs.FetchGraph(n.IpfsNode, &id)
		if err != nil {
			return err
		}
		pointers, err := n.Datastore.Pointers().GetByPurpose(ipfs.MESSAGE)
		if err != nil {
			return err
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
				graph = append(graph, c)
			}
		}
	}
	for _, p := range n.PushNodes {
		go n.retryableSeedStoreToPeer(p, hash, graph)
	}

	return nil
}

func (n *OpenBazaarNode) retryableSeedStoreToPeer(pid peer.ID, graphHash string, graph []cid.Cid) {
	var retryTimeout = 2 * time.Second
	for {
		if graphHash != n.RootHash {
			log.Errorf("root hash has changed, aborting push to %s", pid.Pretty())
			return
		}
		err := n.SendStore(pid.Pretty(), graph)
		if err != nil {
			if retryTimeout > 60*time.Second {
				log.Errorf("error pushing to peer %s: %s", pid.Pretty(), err.Error())
				return
			}
			log.Errorf("error pushing to peer %s...backing off: %s", pid.Pretty(), err.Error())
			time.Sleep(retryTimeout)
			retryTimeout *= 2
			continue
		}
		return
	}
}

// SetUpRepublisher - periodic publishing to IPNS
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

/*EncryptMessage This is a placeholder until the libsignal is operational.
  For now we will just encrypt outgoing offline messages with the long lived identity key.
  Optionally you may provide a public key, to avoid doing an IPFS lookup */
func (n *OpenBazaarNode) EncryptMessage(peerID peer.ID, peerKey *libp2p.PubKey, message []byte) (ct []byte, rerr error) {
	ctx, cancel := context.WithTimeout(context.Background(), n.OfflineMessageFailoverTimeout)
	defer cancel()
	if peerKey == nil {
		var pubKey libp2p.PubKey
		keyval, err := n.IpfsNode.Repo.Datastore().Get(datastore.NewKey(KeyCachePrefix + peerID.Pretty()))
		if err != nil {
			pubKey, err = routing.GetPublicKey(n.IpfsNode.Routing, ctx, peerID)
			if err != nil {
				log.Errorf("Failed to find public key for %s", peerID.Pretty())
				return nil, err
			}
		} else {
			pubKey, err = libp2p.UnmarshalPublicKey(keyval)
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
	}
	log.Errorf("peer public key and id do not match for peer: %s", peerID.Pretty())
	return nil, errors.New("peer public key and id do not match")
}

// IPFSIdentityString - IPFS identifier
func (n *OpenBazaarNode) IPFSIdentityString() string {
	return n.IpfsNode.Identity.Pretty()
}

// createSlugFor Create a slug from a multi-lang string
func createSlugFor(slugName string) string {
	l := SentenceMaxCharacters - SlugBuffer

	slug := slug.Make(slugName)
	if len(slug) < SentenceMaxCharacters-SlugBuffer {
		l = len(slug)
	}
	return slug[:l]
}
