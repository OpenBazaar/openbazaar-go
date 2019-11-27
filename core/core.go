package core

import (
	"errors"
	"fmt"
	"regexp"
	"unicode/utf8"

	"gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	libp2p "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	peer "gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	routing "gx/ipfs/QmYxUdYY9S6yg5tSPVin5GFTvtfsLauVcr7reHDD3dM8xf/go-libp2p-routing"

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
	VERSION = "0.13.6"
	// USERAGENT - user-agent header string
	USERAGENT = "/openbazaar-go:" + VERSION + "/"
)

var log = logging.MustGetLogger("core")

const EmojiPattern = "[\\x{2712}\\x{2714}\\x{2716}\\x{271d}\\x{2721}\\x{2728}\\x{2733}" +
	"\\x{2734}\\x{2744}\\x{2747}\\x{274c}\\x{274e}\\x{2753}-\\x{2755}\\x{2757}" +
	"\\x{2763}\\x{2764}\\x{2795}-\\x{2797}\\x{27a1}\\x{27b0}\\x{27bf}\\x{2934}" +
	"\\x{2935}\\x{2b05}-\\x{2b07}\\x{2b1b}\\x{2b1c}\\x{2b50}\\x{2b55}\\x{3030}" +
	"\\x{303d}\\x{1f004}\\x{1f0cf}\\x{1f170}\\x{1f171}\\x{1f17e}\\x{1f17f}" +
	"\\x{1f18e}\\x{1f191}-\\x{1f19a}\\x{1f201}\\x{1f202}\\x{1f21a}\\x{1f22f}" +
	"\\x{1f232}-\\x{1f23a}\\x{1f250}\\x{1f251}\\x{1f300}-\\x{1f321}\\x{1f324}-" +
	"\\x{1f393}\\x{1f396}\\x{1f397}\\x{1f399}-\\x{1f39b}\\x{1f39e}-\\x{1f3f0}" +
	"\\x{1f3f3}-\\x{1f3f5}\\x{1f3f7}-\\x{1f4fd}\\x{1f4ff}-\\x{1f53d}\\x{1f549}-" +
	"\\x{1f54e}\\x{1f550}-\\x{1f567}\\x{1f56f}\\x{1f570}\\x{1f573}-\\x{1f579}" +
	"\\x{1f587}\\x{1f58a}-\\x{1f58d}\\x{1f590}\\x{1f595}\\x{1f596}\\x{1f5a5}" +
	"\\x{1f5a8}\\x{1f5b1}\\x{1f5b2}\\x{1f5bc}\\x{1f5c2}-\\x{1f5c4}\\x{1f5d1}-" +
	"\\x{1f5d3}\\x{1f5dc}-\\x{1f5de}\\x{1f5e1}\\x{1f5e3}\\x{1f5ef}\\x{1f5f3}" +
	"\\x{1f5fa}-\\x{1f64f}\\x{1f680}-\\x{1f6c5}\\x{1f6cb}-\\x{1f6d0}\\x{1f6e0}-" +
	"\\x{1f6e5}\\x{1f6e9}\\x{1f6eb}\\x{1f6ec}\\x{1f6f0}\\x{1f6f3}\\x{1f910}-" +
	"\\x{1f918}\\x{1f980}-\\x{1f984}\\x{1f9c0}\\x{3297}\\x{3299}\\x{a9}\\x{ae}" +
	"\\x{203c}\\x{2049}\\x{2122}\\x{2139}\\x{2194}-\\x{2199}\\x{21a9}\\x{21aa}" +
	"\\x{231a}\\x{231b}\\x{2328}\\x{2388}\\x{23cf}\\x{23e9}-\\x{23f3}\\x{23f8}-" +
	"\\x{23fa}\\x{24c2}\\x{25aa}\\x{25ab}\\x{25b6}\\x{25c0}\\x{25fb}-\\x{25fe}" +
	"\\x{2600}-\\x{2604}\\x{260e}\\x{2611}\\x{2614}\\x{2615}\\x{2618}\\x{261d}" +
	"\\x{2620}\\x{2622}\\x{2623}\\x{2626}\\x{262a}\\x{262e}\\x{262f}\\x{2638}-" +
	"\\x{263a}\\x{2648}-\\x{2653}\\x{2660}\\x{2663}\\x{2665}\\x{2666}\\x{2668}" +
	"\\x{267b}\\x{267f}\\x{2692}-\\x{2694}\\x{2696}\\x{2697}\\x{2699}\\x{269b}" +
	"\\x{269c}\\x{26a0}\\x{26a1}\\x{26aa}\\x{26ab}\\x{26b0}\\x{26b1}\\x{26bd}" +
	"\\x{26be}\\x{26c4}\\x{26c5}\\x{26c8}\\x{26ce}\\x{26cf}\\x{26d1}\\x{26d3}" +
	"\\x{26d4}\\x{26e9}\\x{26ea}\\x{26f0}-\\x{26f5}\\x{26f7}-\\x{26fa}\\x{26fd}" +
	"\\x{2702}\\x{2705}\\x{2708}-\\x{270d}\\x{270f}]|\\x{23}\\x{20e3}|\\x{2a}" +
	"\\x{20e3}|\\x{30}\\x{20e3}|\\x{31}\\x{20e3}|\\x{32}\\x{20e3}|\\x{33}\\x{20e3}|" +
	"\\x{34}\\x{20e3}|\\x{35}\\x{20e3}|\\x{36}\\x{20e3}|\\x{37}\\x{20e3}|\\x{38}" +
	"\\x{20e3}|\\x{39}\\x{20e3}|\\x{1f1e6}[\\x{1f1e8}-\\x{1f1ec}\\x{1f1ee}" +
	"\\x{1f1f1}\\x{1f1f2}\\x{1f1f4}\\x{1f1f6}-\\x{1f1fa}\\x{1f1fc}\\x{1f1fd}" +
	"\\x{1f1ff}]|\\x{1f1e7}[\\x{1f1e6}\\x{1f1e7}\\x{1f1e9}-\\x{1f1ef}\\x{1f1f1}-" +
	"\\x{1f1f4}\\x{1f1f6}-\\x{1f1f9}\\x{1f1fb}\\x{1f1fc}\\x{1f1fe}\\x{1f1ff}]|" +
	"\\x{1f1e8}[\\x{1f1e6}\\x{1f1e8}\\x{1f1e9}\\x{1f1eb}-\\x{1f1ee}\\x{1f1f0}-" +
	"\\x{1f1f5}\\x{1f1f7}\\x{1f1fa}-\\x{1f1ff}]|\\x{1f1e9}[\\x{1f1ea}\\x{1f1ec}" +
	"\\x{1f1ef}\\x{1f1f0}\\x{1f1f2}\\x{1f1f4}\\x{1f1ff}]|\\x{1f1ea}[\\x{1f1e6}" +
	"\\x{1f1e8}\\x{1f1ea}\\x{1f1ec}\\x{1f1ed}\\x{1f1f7}-\\x{1f1fa}]|\\x{1f1eb}[" +
	"\\x{1f1ee}-\\x{1f1f0}\\x{1f1f2}\\x{1f1f4}\\x{1f1f7}]|\\x{1f1ec}[\\x{1f1e6}" +
	"\\x{1f1e7}\\x{1f1e9}-\\x{1f1ee}\\x{1f1f1}-\\x{1f1f3}\\x{1f1f5}-\\x{1f1fa}" +
	"\\x{1f1fc}\\x{1f1fe}]|\\x{1f1ed}[\\x{1f1f0}\\x{1f1f2}\\x{1f1f3}\\x{1f1f7}" +
	"\\x{1f1f9}\\x{1f1fa}]|\\x{1f1ee}[\\x{1f1e8}-\\x{1f1ea}\\x{1f1f1}-\\x{1f1f4}" +
	"\\x{1f1f6}-\\x{1f1f9}]|\\x{1f1ef}[\\x{1f1ea}\\x{1f1f2}\\x{1f1f4}\\x{1f1f5}]" +
	"|\\x{1f1f0}[\\x{1f1ea}\\x{1f1ec}-\\x{1f1ee}\\x{1f1f2}\\x{1f1f3}\\x{1f1f5}" +
	"\\x{1f1f7}\\x{1f1fc}\\x{1f1fe}\\x{1f1ff}]|\\x{1f1f1}[\\x{1f1e6}-\\x{1f1e8}" +
	"\\x{1f1ee}\\x{1f1f0}\\x{1f1f7}-\\x{1f1fb}\\x{1f1fe}]|\\x{1f1f2}[\\x{1f1e6}" +
	"\\x{1f1e8}-\\x{1f1ed}\\x{1f1f0}-\\x{1f1ff}]|\\x{1f1f3}[\\x{1f1e6}\\x{1f1e8}" +
	"\\x{1f1ea}-\\x{1f1ec}\\x{1f1ee}\\x{1f1f1}\\x{1f1f4}\\x{1f1f5}\\x{1f1f7}" +
	"\\x{1f1fa}\\x{1f1ff}]|\\x{1f1f4}\\x{1f1f2}|\\x{1f1f5}[\\x{1f1e6}\\x{1f1ea}-" +
	"\\x{1f1ed}\\x{1f1f0}-\\x{1f1f3}\\x{1f1f7}-\\x{1f1f9}\\x{1f1fc}\\x{1f1fe}]|" +
	"\\x{1f1f6}\\x{1f1e6}|\\x{1f1f7}[\\x{1f1ea}\\x{1f1f4}\\x{1f1f8}\\x{1f1fa}" +
	"\\x{1f1fc}]|\\x{1f1f8}[\\x{1f1e6}-\\x{1f1ea}\\x{1f1ec}-\\x{1f1f4}\\x{1f1f7}-" +
	"\\x{1f1f9}\\x{1f1fb}\\x{1f1fd}-\\x{1f1ff}]|\\x{1f1f9}[\\x{1f1e6}\\x{1f1e8}" +
	"\\x{1f1e9}\\x{1f1eb}-\\x{1f1ed}\\x{1f1ef}-\\x{1f1f4}\\x{1f1f7}\\x{1f1f9}" +
	"\\x{1f1fb}\\x{1f1fc}\\x{1f1ff}]|\\x{1f1fa}[\\x{1f1e6}\\x{1f1ec}\\x{1f1f2}" +
	"\\x{1f1f8}\\x{1f1fe}\\x{1f1ff}]|\\x{1f1fb}[\\x{1f1e6}\\x{1f1e8}\\x{1f1ea}" +
	"\\x{1f1ec}\\x{1f1ee}\\x{1f1f3}\\x{1f1fa}]|\\x{1f1fc}[\\x{1f1eb}\\x{1f1f8}]|" +
	"\\x{1f1fd}\\x{1f1f0}|\\x{1f1fe}[\\x{1f1ea}\\x{1f1f9}]|\\x{1f1ff}[\\x{1f1e6}" +
	"\\x{1f1f2}\\x{1f1fc}]"

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

	PublishLock sync.Mutex
	seedLock    sync.Mutex

	InitalPublishComplete bool
}

// TestNetworkEnabled indicates whether the node is operating with test parameters
func (n *OpenBazaarNode) TestNetworkEnabled() bool { return n.TestnetEnable }

// RegressionNetworkEnabled indicates whether the node is operating with regression parameters
func (n *OpenBazaarNode) RegressionNetworkEnabled() bool { return n.RegressionTestEnable }

// SeedNode - publish to IPNS
func (n *OpenBazaarNode) SeedNode() error {
	n.seedLock.Lock()
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
		n.seedLock.Unlock()
		return aerr
	}
	n.RootHash = rootHash
	n.seedLock.Unlock()
	n.InitalPublishComplete = true
	go n.publish(rootHash)
	return nil
}

func (n *OpenBazaarNode) publish(hash string) {
	// Multiple publishes may have been queued
	// We only need to publish the most recent
	n.PublishLock.Lock()
	defer n.PublishLock.Unlock()
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
			if retryTimeout > 8*time.Second {
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
		var (
			pubKey libp2p.PubKey
			store  = n.IpfsNode.Repo.Datastore()
		)
		keyval, err := ipfs.GetCachedPubkey(store, peerID.Pretty())
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

func ToHtmlEntities(str string) string {
	var rx = regexp.MustCompile(EmojiPattern)
	return rx.ReplaceAllStringFunc(str, func(s string) string {
		r, _ := utf8.DecodeRuneInString(s)
		html := fmt.Sprintf(`&#x%X;`, r)
		return html
	})
}

// createSlugFor Create a slug from a multi-lang string
func createSlugFor(slugName string) string {
	l := SentenceMaxCharacters - SlugBuffer

	slugName = ToHtmlEntities(slugName)

	slug := slug.Make(slugName)
	if len(slug) < SentenceMaxCharacters-SlugBuffer {
		l = len(slug)
	}
	return slug[:l]
}
