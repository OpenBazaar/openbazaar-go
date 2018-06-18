package spvwallet

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"fmt"
	"github.com/btcsuite/btcd/addrmgr"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/connmgr"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
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

const MaxGetAddressAttempts = 10

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

	// Listeners to handle messages from peers. If nil, no messages will be handled.
	Listeners *peer.MessageListeners

	// An optional proxy dialer. Will use net.Dial if nil.
	Proxy proxy.Dialer

	// Function to return current block hash and height
	GetNewestBlock func() (hash *chainhash.Hash, height int32, err error)

	// The main channel over which to send outgoing events
	MsgChan chan interface{}
}

type PeerManager struct {
	addrManager            *addrmgr.AddrManager
	connManager            *connmgr.ConnManager
	sourceAddr             *wire.NetAddress
	peerConfig             *peer.Config
	peerMutex              *sync.RWMutex
	trustedPeer            net.Addr
	targetOutbound         uint32
	proxy                  proxy.Dialer
	recentlyTriedAddresses map[string]bool
	connectedPeers         map[uint64]*peer.Peer
	msgChan                chan interface{}
}

func NewPeerManager(config *PeerManagerConfig) (*PeerManager, error) {
	port, err := strconv.Atoi(config.Params.DefaultPort)
	defaultPort = uint16(port)
	if err != nil {
		return nil, err
	}

	pm := &PeerManager{
		addrManager: addrmgr.New(config.AddressCacheDir, nil),
		peerMutex:   new(sync.RWMutex),
		sourceAddr:  wire.NewNetAddressIPPort(net.ParseIP("0.0.0.0"), defaultPort, 0),
		trustedPeer: config.TrustedPeer,
		proxy:       config.Proxy,
		recentlyTriedAddresses: make(map[string]bool),
		connectedPeers:         make(map[uint64]*peer.Peer),
		msgChan:                config.MsgChan,
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
	listeners.OnHeaders = pm.onHeaders
	listeners.OnMerkleBlock = pm.onMerkleBlock
	listeners.OnInv = pm.onInv
	listeners.OnTx = pm.onTx
	listeners.OnReject = pm.onReject

	pm.peerConfig = &peer.Config{
		UserAgentName:    config.UserAgentName,
		UserAgentVersion: config.UserAgentVersion,
		ChainParams:      config.Params,
		DisableRelayTx:   true,
		NewestBlock:      config.GetNewestBlock,
		Listeners:        *listeners,
	}
	if config.Proxy != nil {
		pm.peerConfig.Proxy = "0.0.0.0"
	}
	return pm, nil
}

func (pm *PeerManager) ConnectedPeers() []*peer.Peer {
	pm.peerMutex.RLock()
	defer pm.peerMutex.RUnlock()
	var ret []*peer.Peer
	for _, p := range pm.connectedPeers {
		ret = append(ret, p)
	}
	return ret
}

func (pm *PeerManager) onConnection(req *connmgr.ConnReq, conn net.Conn) {
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()

	// Create a new peer for this connection
	p, err := peer.NewOutboundPeer(pm.peerConfig, conn.RemoteAddr().String())
	if err != nil {
		pm.connManager.Disconnect(req.ID())
		return
	}

	// Associate the connection with the peer
	p.AssociateConnection(conn)

	pm.connectedPeers[req.ID()] = p

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
	if !(p.NA().HasService(wire.SFNodeBloom) && p.NA().HasService(wire.SFNodeNetwork) && p.NA().HasService(wire.SFNodeWitness)) ||
		p.NA().HasService(SFNodeBitcoinCash) { // Don't connect to bitcoin cash nodes
		// onDisconnection will be called
		// which will remove the peer from openPeers
		log.Warningf("Peer %s does not support bloom filtering, diconnecting", p)
		p.Disconnect()
		return
	}
	log.Debugf("Connected to %s - %s\n", p.Addr(), p.UserAgent())
	// Tell the addr manager this is a good address
	pm.addrManager.Good(p.NA())
	if pm.msgChan != nil {
		pm.msgChan <- newPeerMsg{p}
	}
}

func (pm *PeerManager) onDisconnection(req *connmgr.ConnReq) {
	// Remove from connected peers
	pm.peerMutex.Lock()
	defer pm.peerMutex.Unlock()
	peer, ok := pm.connectedPeers[req.ID()]
	if !ok {
		return
	}
	log.Debugf("Peer %s disconnected", peer)
	delete(pm.connectedPeers, req.ID())
	if pm.msgChan != nil {
		pm.msgChan <- donePeerMsg{peer}
	}
}

// Called by connManager when it adds a new connection
func (pm *PeerManager) getNewAddress() (net.Addr, error) {
	// If we have a trusted peer we'll just return it
	if pm.trustedPeer == nil {
		pm.peerMutex.Lock()
		defer pm.peerMutex.Unlock()
		// We're going to loop here and pull addresses from the addrManager until we get one that we
		// are not currently connect to or haven't recently tried.
	loop:
		for tries := 0; tries < 100; tries++ {
			ka := pm.addrManager.GetAddress()
			if ka == nil {
				continue
			}

			// only allow recent nodes (10mins) after we failed 30
			// times
			if tries < 30 && time.Since(ka.LastAttempt()) < 10*time.Minute {
				continue
			}

			// allow nondefault ports after 50 failed tries.
			if tries < 50 && fmt.Sprintf("%d", ka.NetAddress().Port) != pm.peerConfig.ChainParams.DefaultPort {
				continue
			}

			knownAddress := ka.NetAddress()

			// Don't return addresses we're still connected to
			for _, p := range pm.connectedPeers {
				if p.NA().IP.String() == knownAddress.IP.String() {
					continue loop
				}
			}
			addr := &net.TCPAddr{
				Port: int(knownAddress.Port),
				IP:   knownAddress.IP,
			}
			pm.addrManager.Attempt(knownAddress)
			return addr, nil
		}
		return nil, errors.New("failed to find appropriate address to return")
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
		pm.peerMutex.RLock()
		defer pm.peerMutex.RUnlock()
		if len(pm.connectedPeers) > 0 {
			log.Debug("Querying peers for more addresses")
			for _, p := range pm.connectedPeers {
				p.QueueMessage(wire.NewMsgGetAddr(), nil)
			}
		} else {
			pm.queryDNSSeeds()
		}
	}
}

func (pm *PeerManager) onAddr(p *peer.Peer, msg *wire.MsgAddr) {
	pm.addrManager.AddAddresses(msg.AddrList, pm.sourceAddr)
}

func (pm *PeerManager) onHeaders(p *peer.Peer, msg *wire.MsgHeaders) {
	if pm.msgChan != nil {
		pm.msgChan <- headersMsg{msg, p}
	}
}

func (pm *PeerManager) onMerkleBlock(p *peer.Peer, msg *wire.MsgMerkleBlock) {
	if pm.msgChan != nil {
		pm.msgChan <- merkleBlockMsg{msg, p}
	}
}

func (pm *PeerManager) onInv(p *peer.Peer, msg *wire.MsgInv) {
	if pm.msgChan != nil {
		pm.msgChan <- invMsg{msg, p}
	}
}

func (pm *PeerManager) onTx(p *peer.Peer, msg *wire.MsgTx) {
	if pm.msgChan != nil {
		pm.msgChan <- txMsg{msg, p, nil}
	}
}

func (pm *PeerManager) onReject(p *peer.Peer, msg *wire.MsgReject) {
	log.Warningf("Received reject message from peer %d: Code: %s, Hash %s, Reason: %s", int(p.ID()), msg.Code.String(), msg.Hash.String(), msg.Reason)
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
			// onDisconnection will be called.
			peer.Disconnect()
			peer.WaitForDisconnect()
			wg.Done()
		}()
	}
	pm.addrManager.Stop()
	pm.connectedPeers = make(map[uint64]*peer.Peer)
	wg.Wait()
}
