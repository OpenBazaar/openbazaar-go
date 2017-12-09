package net

import (
	"context"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/net"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/proto"
	"github.com/ipfs/go-ipfs/commands"
	"golang.org/x/net/proxy"

	"github.com/ipfs/go-ipfs/core"

	routing "gx/ipfs/QmUCS9EnqNq1kCnJds2eLDypBiS21aSiCf1MVzSUVB9TGA/go-libp2p-kad-dht"

	"github.com/op/go-logging"
	"gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
	ps "gx/ipfs/QmPgDWmTmuzvP7QE5zwo1TmjbJme9pmZHNujB2453jkCTr/go-libp2p-peerstore"
	multihash "gx/ipfs/QmU9a9NV9RdPNwZQDYd5uKsm6N6LJLSvLbywDDYFbaaC6P/go-multihash"
	ma "gx/ipfs/QmXY77cVe7rVRQXZZQRioukUM7aRW3BTcAgJe12MCtb3Ji/go-multiaddr"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	libp2p "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	"io/ioutil"
	gonet "net"
	"net/http"
	"sync"
	"time"
)

const DefaultPointerPrefixLength = 14

var log = logging.MustGetLogger("retriever")

type MessageRetriever struct {
	db           repo.Datastore
	node         *core.IpfsNode
	bm           *net.BanManager
	ctx          commands.Context
	service      net.NetworkService
	prefixLen    int
	sendAck      func(peerId string, pointerID peer.ID) error
	messageQueue map[pb.Message_MessageType][]offlineMessage
	httpClient   *http.Client
	dataPeers    []peer.ID
	queueLock    *sync.Mutex
	*sync.WaitGroup
}

type offlineMessage struct {
	addr string
	env  pb.Envelope
}

func NewMessageRetriever(db repo.Datastore, ctx commands.Context, node *core.IpfsNode, bm *net.BanManager, service net.NetworkService, prefixLen int, pushNodes []peer.ID, dialer proxy.Dialer, sendAck func(peerId string, pointerID peer.ID) error) *MessageRetriever {
	dial := gonet.Dial
	if dialer != nil {
		dial = dialer.Dial
	}
	tbTransport := &http.Transport{Dial: dial}
	client := &http.Client{Transport: tbTransport, Timeout: time.Second * 30}
	mr := MessageRetriever{db, node, bm, ctx, service, prefixLen, sendAck, make(map[pb.Message_MessageType][]offlineMessage), client, pushNodes, new(sync.Mutex), new(sync.WaitGroup)}
	// Add one for initial wait at start up
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
			go m.fetchPointers(true)
		case <-peers.C:
			go m.fetchPointers(false)
		}
	}
}

func (m *MessageRetriever) fetchPointers(useDHT bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := new(sync.WaitGroup)
	wg.Add(1)
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

	// Iterate over the pointers, adding 1 to the waitgroup for each pointer found
	for p := range peerOut {
		if len(p.Addrs) > 0 && !m.db.OfflineMessages().Has(p.Addrs[0].String()) {
			log.Debugf("Found pointer with location %s", p.Addrs[0].String())
			// IPFS
			if len(p.Addrs[0].Protocols()) == 1 && p.Addrs[0].Protocols()[0].Code == ma.P_IPFS {
				wg.Add(1)
				downloaded++
				go m.fetchIPFS(p.ID, m.ctx, p.Addrs[0], wg)
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
	// We have finished fetching pointers from the DHT
	wg.Done()

	// Wait for each goroutine to finish then process any remaining messages that needed to be processed last
	wg.Wait()

	m.processQueue()
	m.processOldMessages()

	// For initial start up only
	if m.WaitGroup != nil {
		m.Done()
		m.WaitGroup = nil
	}
}

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

func (m *MessageRetriever) fetchIPFS(pid peer.ID, ctx commands.Context, addr ma.Multiaddr, wg *sync.WaitGroup) {
	defer wg.Done()
	ciphertext, err := ipfs.Cat(ctx, addr.String())
	if err != nil {
		log.Errorf("Error retrieving offline message from %s, %s", addr.String(), err.Error())
		return
	}
	log.Debugf("Successfully downloaded offline message from %s", addr.String())
	m.db.OfflineMessages().Put(addr.String())
	m.attemptDecrypt(ciphertext, pid, addr)
}

func (m *MessageRetriever) fetchHTTPS(pid peer.ID, url string, addr ma.Multiaddr, wg *sync.WaitGroup) {
	defer wg.Done()
	resp, err := m.httpClient.Get(url)
	if err != nil {
		log.Errorf("Error retrieving offline message from %s, %s", addr.String(), err.Error())
		return
	}
	ciphertext, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error retrieving offline message from %s, %s", addr.String(), err.Error())
		return
	}
	log.Debugf("Successfully downloaded offline message from %s", addr.String())
	m.db.OfflineMessages().Put(addr.String())
	m.attemptDecrypt(ciphertext, pid, addr)

}

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
		log.Warning("Unable to decrypt offline message from %s: %s", addr.String(), err.Error())
		return
	}

	m.node.Peerstore.AddPubKey(id, pubkey)

	// Respond with an ACK
	if env.Message.MessageType != pb.Message_OFFLINE_ACK {
		m.sendAck(id.Pretty(), pid)
	}

	// Queue
	m.queueMessage(env, addr.String())
}

func (m *MessageRetriever) queueMessage(env pb.Envelope, addr string) {
	// Order messages need to be processed in the correct order so let's cue them up
	m.queueLock.Lock()
	defer m.queueLock.Unlock()
	switch env.Message.MessageType {
	case pb.Message_ORDER:
		m.messageQueue[pb.Message_ORDER] = append(m.messageQueue[pb.Message_ORDER], offlineMessage{addr, env})
	case pb.Message_ORDER_CANCEL:
		m.messageQueue[pb.Message_ORDER_CANCEL] = append(m.messageQueue[pb.Message_ORDER_CANCEL], offlineMessage{addr, env})
	case pb.Message_ORDER_REJECT:
		m.messageQueue[pb.Message_ORDER_REJECT] = append(m.messageQueue[pb.Message_ORDER_REJECT], offlineMessage{addr, env})
	case pb.Message_ORDER_CONFIRMATION:
		m.messageQueue[pb.Message_ORDER_CONFIRMATION] = append(m.messageQueue[pb.Message_ORDER_CONFIRMATION], offlineMessage{addr, env})
	case pb.Message_ORDER_FULFILLMENT:
		m.messageQueue[pb.Message_ORDER_FULFILLMENT] = append(m.messageQueue[pb.Message_ORDER_FULFILLMENT], offlineMessage{addr, env})
	case pb.Message_ORDER_COMPLETION:
		m.messageQueue[pb.Message_ORDER_COMPLETION] = append(m.messageQueue[pb.Message_ORDER_COMPLETION], offlineMessage{addr, env})
	case pb.Message_DISPUTE_OPEN:
		m.messageQueue[pb.Message_DISPUTE_OPEN] = append(m.messageQueue[pb.Message_DISPUTE_OPEN], offlineMessage{addr, env})
	case pb.Message_DISPUTE_UPDATE:
		m.messageQueue[pb.Message_DISPUTE_UPDATE] = append(m.messageQueue[pb.Message_DISPUTE_UPDATE], offlineMessage{addr, env})
	case pb.Message_DISPUTE_CLOSE:
		m.messageQueue[pb.Message_DISPUTE_CLOSE] = append(m.messageQueue[pb.Message_DISPUTE_CLOSE], offlineMessage{addr, env})
	case pb.Message_REFUND:
		m.messageQueue[pb.Message_REFUND] = append(m.messageQueue[pb.Message_REFUND], offlineMessage{addr, env})
	default:
		m.handleMessage(env, nil)
	}
}

func (m *MessageRetriever) processQueue() {
	processMessages := func(queue []offlineMessage) {
		for _, om := range queue {
			err := m.handleMessage(om.env, nil)
			if err != nil && err == net.OutOfOrderMessage {
				ser, err := proto.Marshal(&om.env)
				if err == nil {
					m.db.OfflineMessages().SetMessage(om.addr, ser)
				}
			}
		}
	}

	queue, ok := m.messageQueue[pb.Message_ORDER]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_ORDER_CANCEL]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_ORDER_REJECT]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_ORDER_CONFIRMATION]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_ORDER_FULFILLMENT]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_REFUND]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_DISPUTE_OPEN]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_DISPUTE_UPDATE]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_DISPUTE_CLOSE]
	if ok {
		processMessages(queue)
	}
	queue, ok = m.messageQueue[pb.Message_ORDER_COMPLETION]
	if ok {
		processMessages(queue)
	}
	m.messageQueue = make(map[pb.Message_MessageType][]offlineMessage)
}

func (m *MessageRetriever) processOldMessages() {
	messages, err := m.db.OfflineMessages().GetMessages()
	if err != nil {
		return
	}
	for url, ser := range messages {
		env := new(pb.Envelope)
		err := proto.Unmarshal(ser, env)
		if err == nil {
			m.queueMessage(*env, url)
		}
		m.db.OfflineMessages().DeleteMessage(url)
	}
	m.processQueue()
}

func (m *MessageRetriever) handleMessage(env pb.Envelope, id *peer.ID) error {
	if id == nil {
		// Get the peer ID from the public key
		pubkey, err := libp2p.UnmarshalPublicKey(env.Pubkey)
		if err != nil {
			return nil
		}
		i, err := peer.IDFromPublicKey(pubkey)
		if err != nil {
			return nil
		}
		id = &i
	}

	// Get handler for this message type
	handler := m.service.HandlerForMsgType(env.Message.MessageType)
	if handler == nil {
		log.Debug("Got back nil handler from HandlerForMsgType")
		return nil
	}

	// Dispatch handler
	_, err := handler(*id, env.Message, true)
	if err != nil && err != net.OutOfOrderMessage {
		log.Errorf("Handle message error: %s", err)
	}
	return err
}
