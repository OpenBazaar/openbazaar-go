package net

import (
	"context"
	"errors"
	routing "gx/ipfs/QmQHnqaNULV8WeUGgh97o9K3KAW6kWQmDyNf9UuikgnPTe/go-libp2p-kad-dht"
	"gx/ipfs/QmS73grfbWgWrNztd8Lns9GCG3jjRNDfcPYg2VYQzKDZSt/go-ipfs-ds-help"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	ps "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	multihash "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	libp2p "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"io/ioutil"
	gonet "net"
	"net/http"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/core"
	"github.com/op/go-logging"
	"golang.org/x/net/proxy"
)

const (
	DefaultPointerPrefixLength = 14
	KeyCachePrefix             = "/pubkey/"
)

var log = logging.MustGetLogger("retriever")

type MRConfig struct {
	Db        repo.Datastore
	IPFSNode  *core.IpfsNode
	BanManger *net.BanManager
	Service   net.NetworkService
	PrefixLen int
	PushNodes []peer.ID
	Dialer    proxy.Dialer
	SendAck   func(peerId string, pointerID peer.ID) error
	SendError func(peerId string, k *libp2p.PubKey, errorMessage pb.Message) error
}

type MessageRetriever struct {
	db         repo.Datastore
	node       *core.IpfsNode
	bm         *net.BanManager
	service    net.NetworkService
	prefixLen  int
	sendAck    func(peerId string, pointerID peer.ID) error
	sendError  func(peerId string, k *libp2p.PubKey, errorMessage pb.Message) error
	httpClient *http.Client
	dataPeers  []peer.ID
	queueLock  *sync.Mutex
	DoneChan   chan struct{}
	inFlight   chan struct{}
	*sync.WaitGroup
}

type offlineMessage struct {
	addr string
	env  pb.Envelope
}

func NewMessageRetriever(cfg MRConfig) *MessageRetriever {
	dial := gonet.Dial
	if cfg.Dialer != nil {
		dial = cfg.Dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Second * 30}
	mr := MessageRetriever{
		cfg.Db,
		cfg.IPFSNode,
		cfg.BanManger,
		cfg.Service,
		cfg.PrefixLen,
		cfg.SendAck,
		cfg.SendError,
		client,
		cfg.PushNodes,
		new(sync.Mutex),
		make(chan struct{}),
		make(chan struct{}, 5),
		new(sync.WaitGroup),
	}
	mr.Add(1)
	return &mr
}

func (m *MessageRetriever) Run() {
	dht := time.NewTicker(time.Hour)
	peers := time.NewTicker(time.Minute * 10)
	defer dht.Stop()
	defer peers.Stop()
	go m.fetchPointers(true)
	for {
		select {
		case <-dht.C:
			m.Add(1)
			go m.fetchPointers(true)
		case <-peers.C:
			m.Add(1)
			go m.fetchPointers(false)
		}
	}
}

func (m *MessageRetriever) fetchPointers(useDHT bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := new(sync.WaitGroup)
	downloaded := 0
	mh, _ := multihash.FromB58String(m.node.Identity.Pretty())
	peerOut := make(chan ps.PeerInfo)
	go func(c chan ps.PeerInfo) {
		pwg := new(sync.WaitGroup)
		pwg.Add(1)
		go func(c chan ps.PeerInfo) {
			out := m.getPointersDataPeers()
			for p := range out {
				c <- p
			}
			pwg.Done()
		}(c)
		if useDHT {
			pwg.Add(1)
			go func(c chan ps.PeerInfo) {
				iout := ipfs.FindPointersAsync(m.node.Routing.(*routing.IpfsDHT), ctx, mh, m.prefixLen)
				for p := range iout {
					c <- p
				}
				pwg.Done()
			}(c)
		}
		pwg.Wait()
		close(c)
	}(peerOut)

	inFlight := make(map[string]bool)
	// Iterate over the pointers, adding 1 to the waitgroup for each pointer found
	for p := range peerOut {
		if len(p.Addrs) > 0 && !m.db.OfflineMessages().Has(p.Addrs[0].String()) && !inFlight[p.Addrs[0].String()] {
			inFlight[p.Addrs[0].String()] = true
			log.Debugf("Found pointer with location %s", p.Addrs[0].String())
			// IPFS
			if len(p.Addrs[0].Protocols()) == 1 && p.Addrs[0].Protocols()[0].Code == ma.P_IPFS {
				wg.Add(1)
				downloaded++
				go m.fetchIPFS(p.ID, m.node, p.Addrs[0], wg)
			}

			// HTTPS
			if len(p.Addrs[0].Protocols()) == 2 && p.Addrs[0].Protocols()[0].Code == ma.P_IPFS && p.Addrs[0].Protocols()[1].Code == ma.P_HTTPS {
				enc, err := p.Addrs[0].ValueForProtocol(ma.P_IPFS)
				if err != nil {
					continue
				}
				mh, err := multihash.FromB58String(enc)
				if err != nil {
					continue
				}
				d, err := multihash.Decode(mh)
				if err != nil {
					continue
				}
				wg.Add(1)
				downloaded++
				go m.fetchHTTPS(p.ID, string(d.Digest), p.Addrs[0], wg)
			}
		}
	}

	// Wait for each goroutine to finish then process any remaining messages that needed to be processed last
	wg.Wait()

	m.processQueuedMessages()

	m.Done()
}

// Connect directly to our data peers and ask them if they have the pointer we're interested in
func (m *MessageRetriever) getPointersDataPeers() <-chan ps.PeerInfo {
	peerOut := make(chan ps.PeerInfo, 100000)
	go m.getPointersFromDataPeersRoutine(peerOut)
	return peerOut
}

func (m *MessageRetriever) getPointersFromDataPeersRoutine(peerOut chan ps.PeerInfo) {
	defer close(peerOut)
	mh, _ := multihash.FromB58String(m.node.Identity.Pretty())
	keyhash := ipfs.CreatePointerKey(mh, DefaultPointerPrefixLength)
	k, _ := cid.Decode(keyhash.B58String())
	var wg sync.WaitGroup
	for _, p := range m.dataPeers {
		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			provs, err := ipfs.GetPointersFromPeer(m.node, ctx, pid, k)
			if err != nil {
				return
			}
			for _, pi := range provs {
				peerOut <- *pi
			}
		}(p)
	}
	wg.Wait()
}

// fetchIPFS will attempt to download an encrypted message using IPFS. If the message downloads successfully, we save the
// address to the database to prevent us from wasting bandwidth downloading it again.
func (m *MessageRetriever) fetchIPFS(pid peer.ID, n *core.IpfsNode, addr ma.Multiaddr, wg *sync.WaitGroup) {
	m.inFlight <- struct{}{}
	defer func() {
		wg.Done()
		<-m.inFlight
	}()

	c := make(chan struct{})
	var ciphertext []byte
	var err error

	go func() {
		ciphertext, err = ipfs.Cat(n, addr.String(), time.Minute*5)
		c <- struct{}{}
	}()

	select {
	case <-c:
		if err != nil {
			log.Errorf("Error retrieving offline message from %s, %s", addr.String(), err.Error())
			return
		}
		log.Debugf("Successfully downloaded offline message from %s", addr.String())
		m.db.OfflineMessages().Put(addr.String())
		m.attemptDecrypt(ciphertext, pid, addr)
	case <-m.DoneChan:
		return
	}
}

// fetchHTTPS will attempt to download an encrypted message from an HTTPS endpoint. If the message downloads successfully, we save the
// address to the database to prevent us from wasting bandwidth downloading it again.
func (m *MessageRetriever) fetchHTTPS(pid peer.ID, url string, addr ma.Multiaddr, wg *sync.WaitGroup) {
	m.inFlight <- struct{}{}
	defer func() {
		wg.Done()
		<-m.inFlight
	}()

	c := make(chan struct{})
	var ciphertext []byte
	var err error

	go func() {
		var resp *http.Response
		resp, err = m.httpClient.Get(url)
		if err != nil {
			log.Errorf("Error retrieving offline message from %s, %s", addr.String(), err.Error())
			c <- struct{}{}
			return
		}
		ciphertext, err = ioutil.ReadAll(resp.Body)
	}()

	select {
	case <-c:
		if err != nil {
			log.Errorf("Error retrieving offline message from %s, %s", addr.String(), err.Error())
			return
		}
		log.Debugf("Successfully downloaded offline message from %s", addr.String())
		m.db.OfflineMessages().Put(addr.String())
		m.attemptDecrypt(ciphertext, pid, addr)
	case <-m.DoneChan:
		return
	}
}

// attemptDecrypt will try to decrypt the message using our identity private key. If it decrypts it will be passed to
// a handler for processing. Not all messages will decrypt. Given the natural of the prefix addressing, we may download
// some messages intended for others. If we can't decrypt it, we can just discard it.
func (m *MessageRetriever) attemptDecrypt(ciphertext []byte, pid peer.ID, addr ma.Multiaddr) {
	// Decrypt and unmarshal plaintext
	plaintext, err := net.Decrypt(m.node.PrivateKey, ciphertext)
	if err != nil {
		log.Warning("Unable to decrypt offline message from %s: %s", addr.String(), err.Error())
		return
	}

	// Unmarshal plaintext
	env := pb.Envelope{}
	err = proto.Unmarshal(plaintext, &env)
	if err != nil {
		log.Warning("Unable to decrypt offline message from %s: %s", addr.String(), err.Error())
		return
	}

	// Validate the signature
	ser, err := proto.Marshal(env.Message)
	if err != nil {
		log.Warning("Unable to decrypt offline message from %s: %s", addr.String(), err.Error())
		return
	}
	pubkey, err := libp2p.UnmarshalPublicKey(env.Pubkey)
	if err != nil {
		log.Warning("Unable to decrypt offline message from %s: %s", addr.String(), err.Error())
		return
	}

	valid, err := pubkey.Verify(ser, env.Signature)
	if err != nil || !valid {
		log.Warning("Unable to decrypt offline message from %s: %s", addr.String(), err.Error())
		return
	}

	id, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		log.Warning("Unable to decrypt offline message from %s: %s", addr.String(), err.Error())
		return
	}

	if m.bm.IsBanned(id) {
		log.Warning("Received and dropped offline message from banned user: %s ", id.String())
		return
	}

	m.node.Peerstore.AddPubKey(id, pubkey)
	m.node.Repo.Datastore().Put(dshelp.NewKeyFromBinary([]byte(KeyCachePrefix+id.Pretty())), env.Pubkey)

	// Respond with an ACK
	if env.Message.MessageType != pb.Message_OFFLINE_ACK {
		m.sendAck(id.Pretty(), pid)
	}

	// handle
	m.handleMessage(env, addr.String(), nil)
}

// handleMessage loads the handler for this message type and attempts to process the message. Some message types (such
// as those partaining to an order) need to be processed in order. In these cases the handler returns a net.OutOfOrderMessage error
// and we must save the message to the database to await further processing.
func (m *MessageRetriever) handleMessage(env pb.Envelope, addr string, id *peer.ID) error {
	if id == nil {
		// Get the peer ID from the public key
		pubkey, err := libp2p.UnmarshalPublicKey(env.Pubkey)
		if err != nil {
			log.Errorf("Error processing message %s. Type %s: %s", addr, env.Message.MessageType, err.Error())
			return err
		}
		i, err := peer.IDFromPublicKey(pubkey)
		if err != nil {
			log.Errorf("Error processing message %s. Type %s: %s", addr, env.Message.MessageType, err.Error())
			return err
		}
		id = &i
	}

	// Get handler for this message type
	handler := m.service.HandlerForMsgType(env.Message.MessageType)
	if handler == nil {
		log.Errorf("Nil handler for message type %s", env.Message.MessageType)
		return errors.New("Nil handler for message")
	}

	// Dispatch handler
	resp, err := handler(*id, env.Message, true)
	if err != nil {
		if err == net.OutOfOrderMessage {
			ser, err := proto.Marshal(&env)
			if err == nil {
				err := m.db.OfflineMessages().SetMessage(addr, ser)
				if err != nil {
					log.Errorf("Error saving offline message %s to database: %s", addr, err.Error())
				}
			} else {
				log.Errorf("Error serializing offline message %s for storage", addr)
			}
		} else if env.Message.MessageType == pb.Message_ORDER && resp != nil {
			log.Errorf("Error processing ORDER message: %s, sending ERROR response", err.Error())
			m.sendError(id.Pretty(), nil, *resp)
			return err
		} else {
			log.Errorf("Error processing message %s. Type %s: %s", addr, env.Message.MessageType, err.Error())
			return err
		}
	}
	return nil
}

var MessageProcessingOrder = []pb.Message_MessageType{
	pb.Message_ORDER,
	pb.Message_ORDER_CANCEL,
	pb.Message_ORDER_REJECT,
	pb.Message_ORDER_CONFIRMATION,
	pb.Message_ORDER_FULFILLMENT,
	pb.Message_ORDER_COMPLETION,
	pb.Message_DISPUTE_OPEN,
	pb.Message_DISPUTE_UPDATE,
	pb.Message_VENDOR_FINALIZED_PAYMENT,
	pb.Message_DISPUTE_CLOSE,
	pb.Message_REFUND,
	pb.Message_CHAT,
	pb.Message_FOLLOW,
	pb.Message_UNFOLLOW,
	pb.Message_MODERATOR_ADD,
	pb.Message_MODERATOR_REMOVE,
	pb.Message_OFFLINE_ACK,
}

// processQueuedMessages loads all the saved messaged from the database for processing. For each message it sorts them into a
// queue based on message type and then processes the queue in order. Any messages that successfully process can then be deleted
// from the database.
func (m *MessageRetriever) processQueuedMessages() {
	messageQueue := make(map[pb.Message_MessageType][]offlineMessage)
	for _, messageType := range MessageProcessingOrder {
		messageQueue[messageType] = []offlineMessage{}
	}

	// Load stored messages from database
	messages, err := m.db.OfflineMessages().GetMessages()
	if err != nil {
		return
	}
	// Sort them into the queue by message type
	for url, ser := range messages {
		env := new(pb.Envelope)
		err := proto.Unmarshal(ser, env)
		if err == nil {
			messageQueue[env.Message.MessageType] = append(messageQueue[env.Message.MessageType], offlineMessage{url, *env})
		} else {
			log.Error("Error unmarshalling serialized offline message from database")
		}
	}
	var toDelete []string
	// Process the queue in order
	for _, messageType := range MessageProcessingOrder {
		queue, ok := messageQueue[messageType]
		if !ok {
			continue
		}
		for _, om := range queue {
			err := m.handleMessage(om.env, om.addr, nil)
			if err == nil {
				toDelete = append(toDelete, om.addr)
			}
		}
	}
	// Delete messages that we're successfully processed from the database
	for _, url := range toDelete {
		m.db.OfflineMessages().DeleteMessage(url)
	}
}
