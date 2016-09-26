package spvwallet

import (
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"net"
	"strings"
	"time"
)

type ConnectionState int

const (
	CONNECTING = 0
	CONNECTED  = 1
	DEAD       = 2
)

// OpenPV starts a
func NewPeer(remoteNode string, blockchain *Blockchain, inTs *TxStore, params *chaincfg.Params, userAgent string, diconnectChan chan string, downloadPeer bool) (*Peer, error) {

	// create new SPVCon
	var err error
	p := new(Peer)

	// I should really merge SPVCon and TxStore, they're basically the same
	inTs.Param = params
	p.TS = inTs // copy pointer of txstore into spvcon

	p.blockchain = blockchain
	p.remoteAddress = remoteNode
	p.disconnectChan = diconnectChan
	p.downloadPeer = downloadPeer
	p.OKTxids = make(map[chainhash.Hash]int32)

	// format if ipv6 addr
	ip := net.ParseIP(remoteNode)
	if ip.To4() == nil && !strings.Contains(remoteNode, "127.0.0.1") {
		li := strings.LastIndex(remoteNode, ":")
		remoteNode = "[" + remoteNode[:li] + "]" + remoteNode[li:len(remoteNode)]
	}

	// open TCP connection
	p.con, err = net.DialTimeout("tcp", remoteNode, time.Second*5)
	if err != nil {
		log.Debugf("Connection to %s failed", remoteNode)
		return p, err
	}
	// assign version bits for local node
	p.localVersion = VERSION
	p.userAgent = userAgent
	go p.run()

	return p, nil
}

func (p *Peer) run() {
	myMsgVer, err := wire.NewMsgVersionFromConn(p.con, 0, 0)
	if err != nil {
		p.disconnectChan <- p.remoteAddress
		return
	}
	err = myMsgVer.AddUserAgent(p.userAgent, WALLET_VERSION)
	if err != nil {
		p.disconnectChan <- p.remoteAddress
		return
	}
	myMsgVer.DisableRelayTx = true

	// this actually sends
	n, err := wire.WriteMessageN(p.con, myMsgVer, p.localVersion, p.TS.Param.Net)
	if err != nil {
		p.disconnectChan <- p.remoteAddress
		return
	}
	p.WBytes += uint64(n)
	log.Debugf("Sent version message to %s\n", p.con.RemoteAddr().String())
	n, m, _, err := wire.ReadMessageN(p.con, p.localVersion, p.TS.Param.Net)
	if err != nil {
		p.disconnectChan <- p.remoteAddress
		return
	}
	p.RBytes += uint64(n)
	log.Debugf("Received %s message from %s\n", m.Command(), p.con.RemoteAddr().String())

	mv, ok := m.(*wire.MsgVersion)
	if ok {
		log.Infof("Connected to %s on %s", mv.UserAgent, p.con.RemoteAddr().String())
	} else {
		p.disconnectChan <- p.remoteAddress
		return
	}
	if !strings.Contains(mv.Services.String(), "SFNodeBloom") {
		p.disconnectChan <- p.remoteAddress
		return
	}
	p.connectionState = CONNECTED

	// set remote height
	p.remoteHeight = mv.LastBlock
	mva := wire.NewMsgVerAck()
	n, err = wire.WriteMessageN(p.con, mva, p.localVersion, p.TS.Param.Net)
	if err != nil {
		p.disconnectChan <- p.remoteAddress
		return
	}
	p.WBytes += uint64(n)

	p.inMsgQueue = make(chan wire.Message)
	go p.incomingMessageHandler()
	p.outMsgQueue = make(chan wire.Message)
	go p.outgoingMessageHandler()

	// create initial filter
	filt, err := p.TS.GimmeFilter()
	if err != nil {
		p.disconnectChan <- p.remoteAddress
		return
	}
	// send filter
	p.SendFilter(filt)
	log.Debugf("Sent filter to %s\n", p.con.RemoteAddr().String())

	p.blockQueue = make(chan HashAndHeight, 32)
	p.fPositives = make(chan int32, 4000) // a block full, approx
	go p.fPositiveHandler()

	if p.downloadPeer {
		log.Infof("Set %s as download peer", p.con.RemoteAddr().String())
		err := p.AskForHeaders()
		if err != nil {
			fmt.Println(err)
		}
	}
}
