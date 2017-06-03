package core

import (
	"bytes"
	"errors"
	libp2p "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	routing "gx/ipfs/QmUc6twRJRE9MNrUGd8eo9WjHHxebGppdZfptGCASkR7fF/go-libp2p-routing"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	gonet "net"
	"net/http"
	"net/url"
	"path"
	"time"

	bstk "github.com/OpenBazaar/go-blockstackclient"
	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/bitcoin"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	rep "github.com/OpenBazaar/openbazaar-go/net/repointer"
	ret "github.com/OpenBazaar/openbazaar-go/net/retriever"
	"github.com/OpenBazaar/openbazaar-go/repo"
	sto "github.com/OpenBazaar/openbazaar-go/storage"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/op/go-logging"
	"golang.org/x/net/context"
	"golang.org/x/net/proxy"
)

var (
	VERSION   = "0.6.2"
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

	// The user-agent for this node
	UserAgent string

	// A dialer for Tor if available
	TorDialer proxy.Dialer

	// Manage blocked peers
	BanManager *net.BanManager
}

// Unpin the current node repo, re-add it, then publish to IPNS
func (n *OpenBazaarNode) SeedNode() error {
	ipfs.UnPinDir(n.Context, n.RootHash)
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
			dial := gonet.Dial
			if n.TorDialer != nil {
				dial = n.TorDialer.Dial
			}
			tbTransport := &http.Transport{Dial: dial}
			client := &http.Client{Transport: tbTransport, Timeout: time.Minute}
			client.Do(req)
		}()
	}
	go n.publish(rootHash)
	return nil
}

func (n *OpenBazaarNode) publish(hash string) {
	if inflightPublishRequests == 0 {
		n.Broadcast <- notifications.StatusNotification{"publishing"}
	}
	var err error
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

/* This is a placeholder until the libsignal is operational.
   For now we will just encrypt outgoing offline messages with the long lived identity key.
   Optionally you may provide a public key, to avoid doing an IPFS lookup */
func (n *OpenBazaarNode) EncryptMessage(peerID peer.ID, peerKey *libp2p.PubKey, message []byte) (ct []byte, rerr error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if peerKey == nil {
		pubKey, err := routing.GetPublicKey(n.IpfsNode.Routing, ctx, []byte(peerID))
		if err != nil {
			log.Errorf("Failed to find public key for %s", peerID.Pretty())
			return nil, err
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
