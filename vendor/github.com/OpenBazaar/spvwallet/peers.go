package spvwallet

import (
	"github.com/btcsuite/btcd/addrmgr"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/connmgr"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/bloom"
	"net"
	"strconv"
	"sync"
	"time"
)

var (
	// Default number of outbound peers
	defaultTargetOutbound = uint32(12)

	// Default duration of time for retrying a connection
	defaultRetryDuration = time.Second * 5

	// Default port per chain params
	defaultPort uint16
)

type Config struct {

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
}

type PeerManager struct {
	addrManager *addrmgr.AddrManager
	connManager *connmgr.ConnManager

	sourceAddr *wire.NetAddress

	peerConfig     *peer.Config
	connectedPeers map[uint64]*peer.Peer
	peerMutex      *sync.RWMutex

	trustedPeer  net.Addr
	downloadPeer *peer.Peer

	getFilter          func() (*bloom.Filter, error)
	startChainDownload func(*peer.Peer)
}

func NewPeerManager(config *Config) (*PeerManager, error) {
	port, err := strconv.Atoi(config.Params.DefaultPort)
	defaultPort = uint16(port)
	if err != nil {
		return nil, err
	}

	pm := &PeerManager{
		addrManager:        addrmgr.New(config.AddressCacheDir, nil),
		peerMutex:          new(sync.RWMutex),
		connectedPeers:     make(map[uint64]*peer.Peer),
		sourceAddr:         wire.NewNetAddressIPPort(net.ParseIP("0.0.0.0"), defaultPort, 0),
		trustedPeer:        config.TrustedPeer,
		getFilter:          config.GetFilter,
		startChainDownload: config.StartChainDownload,
	}

	targetOutbound := config.TargetOutbound
	if config.TargetOutbound == 0 {
		targetOutbound = defaultTargetOutbound
	}

	if config.TrustedPeer != nil {
		targetOutbound = 1
	}

	retryDuration := config.RetryDuration
	if config.RetryDuration <= 0 {
		retryDuration = defaultRetryDuration
	}

	connMgrConfig := &connmgr.Config{
		TargetOutbound:  targetOutbound,
		RetryDuration:   retryDuration,
		OnConnection:    pm.onConnection,
		OnDisconnection: pm.onDisconnection,
		GetNewAddress:   pm.getNewAddress,
		Dial: func(addr net.Addr) (net.Conn, error) {
			return net.Dial("tcp", addr.String())
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
	return pm, nil
}

func (pm *PeerManager) ConnectedPeers() []*peer.Peer {
	var peers []*peer.Peer
	pm.peerMutex.RLock()
	defer pm.peerMutex.RUnlock()
	for _, peer := range pm.connectedPeers {
		peers = append(peers, peer)
	}
	return peers
}

func (pm *PeerManager) DownloadPeer() *peer.Peer {
	return pm.downloadPeer
}

func (pm *PeerManager) onConnection(req *connmgr.ConnReq, conn net.Conn) {
	// Don't let the connection manager connect us to the same peer more than once
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()
	for _, peer := range pm.connectedPeers {
		if conn.RemoteAddr().String() == peer.Addr() {
			pm.connManager.Disconnect(req.ID())
			return
		}
	}

	// Create a new peer for this connection
	p, err := peer.NewOutboundPeer(pm.peerConfig, conn.RemoteAddr().String())
	if err != nil {
		pm.connManager.Disconnect(req.ID())
		return
	}

	// Add to connected peers
	pm.connectedPeers[req.ID()] = p

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
	if !(p.NA().HasService(wire.SFNodeBloom) && p.NA().HasService(wire.SFNodeNetwork)) {
		pm.peerMutex.Lock()
		for id, peer := range pm.connectedPeers {
			if peer.ID() == p.ID() {
				delete(pm.connectedPeers, id)
				pm.connManager.Disconnect(id)
				break
			}
		}
		pm.peerMutex.Unlock()
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
	if pm.downloadPeer == nil {
		pm.setDownloadPeer(p)
	}
	pm.peerMutex.Unlock()
}

func (pm *PeerManager) onDisconnection(req *connmgr.ConnReq) {
	// Remove from connected peers
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()
	peer, ok := pm.connectedPeers[req.ID()]
	if ok {
		log.Debugf("Peer%d disconnected", peer.ID())
		delete(pm.connectedPeers, req.ID())
	}
	pm.connManager.Disconnect(req.ID())

	// If this was our download peer we lost, replace him
	if pm.downloadPeer != nil && peer != nil {
		if pm.downloadPeer.ID() == peer.ID() {
			for id, _ := range pm.connectedPeers {
				pm.setDownloadPeer(pm.connectedPeers[id])
				break
			}
		}
	}
}

func (pm *PeerManager) setDownloadPeer(peer *peer.Peer) {
	log.Infof("Setting peer%d as download peer\n", peer.ID())
	pm.downloadPeer = peer
	if pm.startChainDownload != nil {
		go pm.startChainDownload(pm.downloadPeer)
	}
}

// Iterates over our peers and sees if any are reporting a height
// greater than our height. If so switch them to the download peer
// and start the chain download again.
func (pm *PeerManager) CheckForMoreBlocks(height uint32) (moar bool) {
	pm.peerMutex.RLock()
	defer pm.peerMutex.RUnlock()

	for _, peer := range pm.connectedPeers {
		if uint32(peer.LastBlock()) > height {
			pm.downloadPeer = peer
			go pm.startChainDownload(peer)
			return true
		}
	}
	return false
}

// Called by connManager when it adds a new connection
func (pm *PeerManager) getNewAddress() (net.Addr, error) {
	if pm.trustedPeer == nil {
		knownAddress := pm.addrManager.GetAddress().NetAddress()
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
			addrs, err := net.LookupHost(host)
			if err != nil {
				wg.Done()
				return
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
		if len(pm.connectedPeers) > 0 {
			pm.peerMutex.RLock()
			log.Debug("Querying peers for more addresses")
			for _, peer := range pm.connectedPeers {
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
	for _, peer := range pm.connectedPeers {
		wg.Add(1)
		go func() {
			peer.Disconnect()
			peer.WaitForDisconnect()
			wg.Done()
		}()
	}
	wg.Wait()
}
