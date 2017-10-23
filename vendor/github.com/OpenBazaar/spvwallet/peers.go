package spvwallet

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/btcsuite/btcd/addrmgr"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/connmgr"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/bloom"
	"golang.org/x/net/proxy"
)

var (
	// Default number of outbound peers
	defaultTargetOutbound = uint32(12)

	// Default duration of time for retrying a connection
	defaultRetryDuration = time.Second * 5

	// Default port per chain params
	defaultPort uint16
)

var SFNodeBitcoinCash wire.ServiceFlag = 1 << 5

type PeerManagerConfig struct {

	// The network parameters to use
	Params *chaincfg.Params

	// The target number of outbound peers. Defaults to 10.
	TargetOutbound uint32

	// Duration of time to retry a connection. Defaults to 5 seconds.
	RetryDuration time.Duration

	// UserAgentName specifies the user agent name to advertise.  It is
	// highly recommended to specify this value.
	UserAgentName string

	// UserAgentVersion specifies the user agent version to advertise.  It
	// is highly recommended to specify this value and that it follows the
	// form "major.minor.revision" e.g. "2.6.41".
	UserAgentVersion string

	// The directory to store cached peers
	AddressCacheDir string

	// If this field is not nil the PeerManager will only connect to this address
	TrustedPeer net.Addr

	// Function to get bloom filter to give to peers
	GetFilter func() (*bloom.Filter, error)

	// Function to beging chain download
	StartChainDownload func(*peer.Peer)

	// Functon returns info about the last block in the chain
	GetNewestBlock func() (hash *chainhash.Hash, height int32, err error)

	// Listeners to handle messages from peers. If nil, no messages will be handled.
	Listeners *peer.MessageListeners

	// An optional proxy dialer. Will use net.Dial if nil.
	Proxy proxy.Dialer
}

type PeerManager struct {
	addrManager *addrmgr.AddrManager
	connManager *connmgr.ConnManager

	sourceAddr *wire.NetAddress

	peerConfig *peer.Config
	openPeers  map[uint64]*peer.Peer
	readyPeers map[*peer.Peer]struct{}
	peerMutex  *sync.RWMutex

	trustedPeer    net.Addr
	downloadPeer   *peer.Peer
	downloadQueues map[int32]map[chainhash.Hash]int32
	blockQueue     chan chainhash.Hash

	getFilter          func() (*bloom.Filter, error)
	startChainDownload func(*peer.Peer)

	targetOutbound uint32

	proxy proxy.Dialer
}

func NewPeerManager(config *PeerManagerConfig) (*PeerManager, error) {
	port, err := strconv.Atoi(config.Params.DefaultPort)
	defaultPort = uint16(port)
	if err != nil {
		return nil, err
	}

	pm := &PeerManager{
		addrManager:        addrmgr.New(config.AddressCacheDir, nil),
		peerMutex:          new(sync.RWMutex),
		openPeers:          make(map[uint64]*peer.Peer),
		readyPeers:         make(map[*peer.Peer]struct{}),
		downloadQueues:     make(map[int32]map[chainhash.Hash]int32),
		sourceAddr:         wire.NewNetAddressIPPort(net.ParseIP("0.0.0.0"), defaultPort, 0),
		trustedPeer:        config.TrustedPeer,
		getFilter:          config.GetFilter,
		startChainDownload: config.StartChainDownload,
		proxy:              config.Proxy,
		blockQueue:         make(chan chainhash.Hash, 32),
	}

	targetOutbound := config.TargetOutbound
	if config.TargetOutbound == 0 {
		targetOutbound = defaultTargetOutbound
	}

	if config.TrustedPeer != nil {
		targetOutbound = 1
	}
	pm.targetOutbound = targetOutbound

	retryDuration := config.RetryDuration
	if config.RetryDuration <= 0 {
		retryDuration = defaultRetryDuration
	}

	dial := net.Dial
	if config.Proxy != nil {
		dial = config.Proxy.Dial
	}

	connMgrConfig := &connmgr.Config{
		TargetOutbound:  targetOutbound,
		RetryDuration:   retryDuration,
		OnConnection:    pm.onConnection,
		OnDisconnection: pm.onDisconnection,
		GetNewAddress:   pm.getNewAddress,
		Dial: func(addr net.Addr) (net.Conn, error) {
			return dial("tcp", addr.String())
		},
	}

	connMgr, err := connmgr.New(connMgrConfig)
	if err != nil {
		return nil, err
	}
	pm.connManager = connMgr

	var listeners *peer.MessageListeners = config.Listeners
	if listeners == nil {
		listeners = &peer.MessageListeners{}
	}
	listeners.OnVerAck = pm.onVerack
	listeners.OnAddr = pm.onAddr

	pm.peerConfig = &peer.Config{
		NewestBlock:      config.GetNewestBlock,
		UserAgentName:    config.UserAgentName,
		UserAgentVersion: config.UserAgentVersion,
		ChainParams:      config.Params,
		DisableRelayTx:   true,
		Listeners:        *listeners,
	}
	if config.Proxy != nil {
		pm.peerConfig.Proxy = "0.0.0.0"
	}
	return pm, nil
}

func (pm *PeerManager) ReadyPeers() []*peer.Peer {
	var peers []*peer.Peer
	pm.peerMutex.RLock()
	defer pm.peerMutex.RUnlock()
	for peer := range pm.readyPeers {
		peers = append(peers, peer)
	}
	return peers
}

func (pm *PeerManager) DownloadPeer() *peer.Peer {
	return pm.downloadPeer
}

func (pm *PeerManager) BlockQueue() chan chainhash.Hash {
	return pm.blockQueue
}

func (pm *PeerManager) onConnection(req *connmgr.ConnReq, conn net.Conn) {
	// Don't let the connection manager connect us to the same peer more than once unless we're using a proxy
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()
	if pm.proxy == nil {
		for _, peer := range pm.openPeers {
			if conn.RemoteAddr().String() == peer.Addr() {
				pm.connManager.Disconnect(req.ID())
				return
			}
		}
	}

	// Create a new peer for this connection
	p, err := peer.NewOutboundPeer(pm.peerConfig, conn.RemoteAddr().String())
	if err != nil {
		pm.connManager.Disconnect(req.ID())
		return
	}

	// Add to open peers
	pm.openPeers[req.ID()] = p

	// Associate the connection with the peer
	p.AssociateConnection(conn)

	// Tell the addr manager we made a connection
	pm.addrManager.Connected(p.NA())

	// Handle disconnect
	go func() {
		p.WaitForDisconnect()
		pm.connManager.Disconnect(req.ID())
	}()
}

func (pm *PeerManager) onVerack(p *peer.Peer, msg *wire.MsgVerAck) {
	// Check this peer offers bloom filtering services. If not dump them.
	p.NA().Services = p.Services()
	if !(p.NA().HasService(wire.SFNodeBloom) && p.NA().HasService(wire.SFNodeNetwork)) ||
		p.NA().HasService(SFNodeBitcoinCash) { // Don't connect to bitcoin cash nodes
		// onDisconnection will be called
		// which will remove the peer from openPeers
		p.Disconnect()
		return
	}
	log.Debugf("Connected to %s - %s\n", p.Addr(), p.UserAgent())
	// Tell the addr manager this is a good address
	pm.addrManager.Good(p.NA())

	filter, err := pm.getFilter()
	if err != nil {
		log.Error(err)
		return
	}
	p.QueueMessage(filter.MsgFilterLoad(), nil)

	pm.peerMutex.Lock()
	pm.readyPeers[p] = struct{}{}
	if pm.downloadPeer == nil {
		pm.setDownloadPeer(p)
	}
	pm.peerMutex.Unlock()
}

func (pm *PeerManager) onDisconnection(req *connmgr.ConnReq) {
	// Remove from connected peers
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()
	peer, ok := pm.openPeers[req.ID()]
	if !ok {
		return
	}

	log.Debugf("Peer%d disconnected", peer.ID())
	delete(pm.openPeers, req.ID())
	delete(pm.downloadQueues, peer.ID())
	delete(pm.readyPeers, peer)

	// If this was our download peer we lost, replace him
	if pm.downloadPeer != nil && peer != nil {
		if pm.downloadPeer.ID() == peer.ID() {
			close(pm.blockQueue)
			go pm.selectNewDownloadPeer()
		}
	}
}

func (pm *PeerManager) selectNewDownloadPeer() {
	for peer := range pm.readyPeers {
		pm.setDownloadPeer(peer)
		break
	}
}

func (pm *PeerManager) setDownloadPeer(peer *peer.Peer) {
	log.Infof("Setting peer%d as download peer\n", peer.ID())
	pm.downloadPeer = peer
	if pm.startChainDownload != nil {
		pm.blockQueue = make(chan chainhash.Hash, 32)
		go pm.startChainDownload(pm.downloadPeer)
	}
}

func (pm *PeerManager) QueueTxForDownload(peer *peer.Peer, txid chainhash.Hash, height int32) {
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()
	queue, ok := pm.downloadQueues[peer.ID()]
	if !ok {
		queue = make(map[chainhash.Hash]int32)
		pm.downloadQueues[peer.ID()] = queue
	}
	queue[txid] = height
}

func (pm *PeerManager) DequeueTx(peer *peer.Peer, txid chainhash.Hash) (int32, error) {
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()
	queue, ok := pm.downloadQueues[peer.ID()]
	if !ok {
		return 0, errors.New("Transaction not found")
	}
	height, ok := queue[txid]
	if !ok {
		return 0, errors.New("Transaction not found")
	}
	delete(queue, txid)
	return height, nil
}

// Iterates over our peers and sees if any are reporting a height
// greater than our height. If so switch them to the download peer
// and start the chain download again.
func (pm *PeerManager) CheckForMoreBlocks(height uint32) bool {
	pm.peerMutex.RLock()
	defer pm.peerMutex.RUnlock()

	moar := false
	for peer := range pm.readyPeers {
		if uint32(peer.LastBlock()) > height {
			pm.downloadPeer = peer
			go pm.startChainDownload(peer)
			moar = true
		}
	}
	return moar
}

// Called by connManager when it adds a new connection
func (pm *PeerManager) getNewAddress() (net.Addr, error) {
	if pm.trustedPeer == nil {
		ka := pm.addrManager.GetAddress()
		if ka == nil {
			return &net.TCPAddr{}, errors.New("Adder manager returned nil address")
		}
		knownAddress := ka.NetAddress()
		addr := &net.TCPAddr{
			Port: int(knownAddress.Port),
			IP:   knownAddress.IP,
		}
		return addr, nil
	} else {
		return pm.trustedPeer, nil
	}
}

// Query the DNS seeds and pass the addresses into the address manager.
func (pm *PeerManager) queryDNSSeeds() {
	wg := new(sync.WaitGroup)
	for _, seed := range pm.peerConfig.ChainParams.DNSSeeds {
		wg.Add(1)
		go func(host string) {
			returnedAddresses := 0
			var addrs []string
			var err error
			if pm.proxy != nil {
				for i := 0; i < 5; i++ {
					ips, err := TorLookupIP(host)
					if err != nil {
						wg.Done()
						return
					}
					for _, ip := range ips {
						addrs = append(addrs, ip.String())
					}
				}
			} else {
				addrs, err = net.LookupHost(host)
				if err != nil {
					wg.Done()
					return
				}
			}
			for _, addr := range addrs {
				netAddr := wire.NewNetAddressIPPort(net.ParseIP(addr), defaultPort, 0)
				pm.addrManager.AddAddress(netAddr, pm.sourceAddr)
				returnedAddresses++
			}
			log.Debugf("%s returned %s addresses\n", host, strconv.Itoa(returnedAddresses))
			wg.Done()
		}(seed.Host)
	}
	wg.Wait()
}

// If we have connected peers let's use them to get more addresses. If not, use the DNS seeds
func (pm *PeerManager) getMoreAddresses() {
	if pm.addrManager.NeedMoreAddresses() {
		if len(pm.readyPeers) > 0 {
			pm.peerMutex.RLock()
			log.Debug("Querying peers for more addresses")
			for peer := range pm.readyPeers {
				peer.QueueMessage(wire.NewMsgGetAddr(), nil)
			}
			pm.peerMutex.RUnlock()
		} else {
			pm.queryDNSSeeds()
		}
	}
}

func (pm *PeerManager) onAddr(p *peer.Peer, msg *wire.MsgAddr) {
	pm.addrManager.AddAddresses(msg.AddrList, pm.sourceAddr)
}

func (pm *PeerManager) Start() {
	pm.addrManager.Start()
	log.Infof("Loaded %d peers from cache\n", pm.addrManager.NumAddresses())
	if pm.trustedPeer == nil && pm.addrManager.NeedMoreAddresses() {
		log.Info("Querying DNS seeds")
		pm.queryDNSSeeds()
	}
	pm.connManager.Start()
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				pm.getMoreAddresses()
			}
		}
	}()
}

func (pm *PeerManager) Stop() {
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()
	wg := new(sync.WaitGroup)
	for _, peer := range pm.openPeers {
		wg.Add(1)
		go func() {
			// onDisconnection will be called.
			peer.Disconnect()
			peer.WaitForDisconnect()
			wg.Done()
		}()
	}
	pm.openPeers = make(map[uint64]*peer.Peer)
	wg.Wait()
}
