package net

import (
	"context"
	"errors"

	routing "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	libp2p "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	ps "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	"gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"io/ioutil"
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

const DefaultPointerPrefixLength = 14

var (
	// Initialize a clear pointerList for the DHT on start
	pointerList = []string{}
	log         = logging.MustGetLogger("retriever")
)

type MRConfig struct {
	Db        repo.Datastore
	IPFSNode  *core.IpfsNode
	DHT       *routing.IpfsDHT
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
	routing    *routing.IpfsDHT
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

func stringInSlice(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}

// Reset on startup
func (m *MessageRetriever) ResetPointerList() {
	pointerList = []string{}
}

func NewMessageRetriever(cfg MRConfig) *MessageRetriever {
	var client *http.Client
	if cfg.Dialer != nil {
		tbTransport := &http.Transport{Dial: cfg.Dialer.Dial}
		client = &http.Client{Transport: tbTransport, Timeout: time.Second * 30}
	} else {
		client = &http.Client{Timeout: time.Second * 30}
	}
	mr := MessageRetriever{
		db:         cfg.Db,
		node:       cfg.IPFSNode,
		routing:    cfg.DHT,
		bm:         cfg.BanManger,
		service:    cfg.Service,
		prefixLen:  cfg.PrefixLen,
		sendAck:    cfg.SendAck,
		sendError:  cfg.SendError,
		httpClient: client,
		dataPeers:  cfg.PushNodes,
		queueLock:  new(sync.Mutex),
		DoneChan:   make(chan struct{}),
		inFlight:   make(chan struct{}, 5),
		WaitGroup:  new(sync.WaitGroup),
	}

	mr.Add(2)
	return &mr
}

func (m *MessageRetriever) Run() {
	dht := time.NewTicker(time.Hour)
	peers := time.NewTicker(time.Minute)
	defer dht.Stop()
	defer peers.Stop()
	go m.fetchPointersFromPushNodes()
	go m.fetchPointersFromDHT()
	for {
		select {
		case <-dht.C:
			m.Add(1)
			go m.fetchPointersFromDHT()
		case <-peers.C:
			m.Add(1)
			go m.fetchPointersFromPushNodes()
		}
	}
}

// RunOnce - used to fetch messages only once
func (m *MessageRetriever) RunOnce() {
	m.Add(1)
	go m.fetchPointersFromDHT()
	m.Add(1)
	go m.fetchPointersFromPushNodes()
}

func (m *MessageRetriever) fetchPointersFromDHT() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mh, _ := multihash.FromB58String(m.node.Identity.Pretty())
	peerOut := make(chan ps.PeerInfo)
	go func(c chan ps.PeerInfo) {
		iout := ipfs.FindPointersAsync(m.routing, ctx, mh, m.prefixLen)
		for p := range iout {
			c <- p
		}
		close(c)

	}(peerOut)

	m.downloadMessages(peerOut)
}

func (m *MessageRetriever) fetchPointersFromPushNodes() {
	peerOut := make(chan ps.PeerInfo)
	go func(c chan ps.PeerInfo) {
		out := m.getPointersDataPeers()
		for p := range out {
			c <- p
		}
		close(c)

	}(peerOut)
	m.downloadMessages(peerOut)
}

func (m *MessageRetriever) downloadMessages(peerOut chan ps.PeerInfo) {
	wg := new(sync.WaitGroup)
	downloaded := 0

	inFlight := make(map[string]bool)
	// Iterate over the pointers, adding 1 to the waitgroup for each pointer found
	for p := range peerOut {
		if len(p.Addrs) > 0 && !m.db.OfflineMessages().Has(p.Addrs[0].String()) && !stringInSlice(p.Addrs[0].String(), pointerList) && !inFlight[p.Addrs[0].String()] {
			pointerList = append(pointerList, p.Addrs[0].String())
			log.Debugf("Looking for pointer [%v] at %v\n", p.ID.Pretty(), p.Addrs)
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
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*35)
			defer cancel()
			time.Sleep(time.Second*15)
			provs, err := ipfs.GetPointersFromPeer(m.routing, ctx, pid, &k)
			if err != nil {
				log.Errorf("Could not get pointers from push node because: %v", err)
				return
			}
			log.Debugf("Successfully queried %s for pointers", pid.Pretty())
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
		ciphertext, err = ipfs.Cat(n, addr.String(), time.Second*10)
		c <- struct{}{}
	}()

	select {
	case <-c:
		if err != nil {
			log.Errorf("Error retrieving offline message from: %s, Error: %s", addr.String(), err.Error())
			return
		}
		log.Debugf("Successfully downloaded offline message %s from: %s", addr.String(), pid.Pretty())

		err = m.db.OfflineMessages().Put(addr.String())
		if err != nil {
			log.Error(err)
		}
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
		err = m.db.OfflineMessages().Put(addr.String())
		if err != nil {
			log.Error(err)
		}
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
		log.Warningf("Unable to decrypt cipher text to plain text, CID: %s: Error:%s\n", addr.String(), err.Error())
		return
	}

	// Unmarshal plaintext
	env := pb.Envelope{}
	err = proto.Unmarshal(plaintext, &env)
	if err != nil {
		log.Warningf("Unable to unmarshal plaintext to encrypted Envelope, CID: %s: Error:%s\n", addr.String(), err.Error())
		return
	}

	// Validate the signature
	ser, err := proto.Marshal(env.Message)
	if err != nil {
		log.Warningf("Unable to serialize the encrypted message, CID: %s: Error:%s\n", addr.String(), err.Error())
		return
	}
	pubkey, err := libp2p.UnmarshalPublicKey(env.Pubkey)
	if err != nil {
		log.Warningf("Unable to unmarshal the public key from, CID: %s: Error:%s\n", addr.String(), err.Error())
		return
	}

	valid, err := pubkey.Verify(ser, env.Signature)
	if err != nil || !valid {
		log.Warningf("Unable to verify message signature, CID: %s: Error:%s\n", addr.String(), err.Error())
		return
	}

	id, err := peer.IDFromPublicKey(pubkey)
	if err != nil {
		log.Warningf("Unable to get a peer ID from the pubkey, CID: %s: Error:%s\n", addr.String(), err.Error())
		return
	}

	log.Debugf("Received offline message %s from: %s\n", addr.String(), id.Pretty())

	if m.bm.IsBanned(id) {
		log.Warningf("Received and dropped offline message from banned user: %s\n", id.Pretty())
		return
	}

	if err := m.node.Peerstore.AddPubKey(id, pubkey); err != nil {
		log.Errorf("adding pubkey to peerstore: %s", err.Error())
	}
	store := m.node.Repo.Datastore()
	if err := ipfs.PutCachedPubkey(store, id.Pretty(), env.Pubkey); err != nil {
		log.Errorf("caching pubkey: %s", err.Error())
	}

	// Respond with an ACK
	if env.Message.MessageType != pb.Message_OFFLINE_ACK {
		err = m.sendAck(id.Pretty(), pid)
		if err != nil {
			log.Error(err)
		}
	}

	// handle
	err = m.handleMessage(env, addr.String(), nil)
	if err != nil {
		log.Error(err)
	}
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
		return errors.New("nil handler for message")
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
			err = m.sendError(id.Pretty(), nil, *resp)
			if err != nil {
				log.Error(err)
			}
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
	pb.Message_ORDER_PAYMENT,
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
	pb.Message_OFFLINE_RELAY,
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
		err = m.db.OfflineMessages().DeleteMessage(url)
		if err != nil {
			log.Error(err)
		}
	}
}
