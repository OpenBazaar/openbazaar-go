package spvwallet

import (
	"github.com/btcsuite/btcd/wire"
)

func (p *Peer) incomingMessageHandler() {
	for {
		n, xm, _, err := wire.ReadMessageN(p.con, p.localVersion, p.TS.Param.Net)
		if err != nil {
			p.disconnectChan <- p.remoteAddress
			return
		}
		p.RBytes += uint64(n)
		switch m := xm.(type) {
		case *wire.MsgVersion:
			log.Debugf("Got version message.  Agent %s, version %d, at height %d\n",
				m.UserAgent, m.ProtocolVersion, m.LastBlock)
			p.remoteVersion = uint32(m.ProtocolVersion) // weird cast! bug?
		case *wire.MsgVerAck:
			log.Debugf("Got verack from %s. Whatever.\n", p.con.RemoteAddr().String())
		case *wire.MsgAddr:
			log.Debugf("Received %d addresses from %s\n", len(m.AddrList), p.con.RemoteAddr().String())
		case *wire.MsgPing:
			// log.Debugf("Got a ping message.  We should pong back or they will kick us off.")
			go p.PongBack(m.Nonce)
		case *wire.MsgPong:
			log.Debugf("Got a pong response. OK.\n")
		case *wire.MsgMerkleBlock:
			if p.TS.chainState == WAITING {
				p.IngestBlockAndHeader(m)
			} else {
				p.IngestMerkleBlock(m)
			}
		case *wire.MsgHeaders: // concurrent because we keep asking for blocks
			go p.HeaderHandler(m)
		case *wire.MsgTx: // not concurrent! txs must be in order
			p.TxHandler(m)
		case *wire.MsgReject:
			log.Warningf("Rejected! cmd: %s code: %s tx: %s reason: %s",
				m.Cmd, m.Code.String(), m.Hash.String(), m.Reason)
		case *wire.MsgInv:
			p.InvHandler(m)
		case *wire.MsgNotFound:
			log.Warningf("Got not found response from %s:\n", p.con.RemoteAddr().String())
			for i, thing := range m.InvList {
				log.Warningf("\t$d) %s: %s", i, thing.Type, thing.Hash)
			}
		case *wire.MsgGetData:
			p.GetDataHandler(m)

		default:
			log.Warningf("Received unknown message type %s from %s\n", m.Command(), p.con.RemoteAddr().String())
		}
	}
	return
}

// this one seems kindof pointless?  could get ridf of it and let
// functions call WriteMessageN themselves...
func (p *Peer) outgoingMessageHandler() {
	for {
		msg := <-p.outMsgQueue
		n, err := wire.WriteMessageN(p.con, msg, p.localVersion, p.TS.Param.Net)
		if err != nil {
			log.Errorf("Write message error: %s", err.Error())
		}
		p.WBytes += uint64(n)
	}
	return
}

// fPositiveHandler monitors false positives and when it gets enough of them,
//
func (p *Peer) fPositiveHandler() {
	var fpAccumulator int32
	for {
		fpAccumulator += <-p.fPositives // blocks here
		if fpAccumulator > 7 {
			p.UpdateFilterAndSend()
			// clear the channel
		finClear:
			for {
				select {
				case x := <-p.fPositives:
					fpAccumulator += x
				default:
					break finClear
				}
			}

			log.Debugf("Reset %d false positives for peer %s\n", fpAccumulator, p.con.RemoteAddr().String())
			// reset accumulator
			fpAccumulator = 0
		}
	}
}

func (p *Peer) HeaderHandler(m *wire.MsgHeaders) {
	moar, err := p.IngestHeaders(m)
	if err != nil {
		log.Errorf("Header error: %s\n", err.Error())
		return
	}
	// more to get? if so, ask for them and return
	if moar {
		err = p.AskForHeaders()
		if err != nil {
			log.Errorf("AskForHeaders error: %s", err.Error())
		}
		return
	}

	// no moar, done w/ headers, get blocks
	err = p.AskForBlocks()
	if err != nil {
		log.Errorf("AskForBlocks error: %s", err.Error())
		return
	}
}

// TxHandler takes in transaction messages that come in from either a request
// after an inv message or after a merkle block message.
func (p *Peer) TxHandler(m *wire.MsgTx) {
	p.OKMutex.Lock()
	height, ok := p.OKTxids[m.TxSha()]
	p.OKMutex.Unlock()
	if !ok {
		log.Warningf("Received unknown tx: %s", m.TxSha().String())
		return
	}

	// check for double spends
	//	allTxs, err := s.TS.GetAllTxs()
	//	if err != nil {
	//		log.Debugf("Can't get txs from db: %s", err.Error())
	//		return
	//	}
	//	dubs, err := CheckDoubleSpends(m, allTxs)
	//	if err != nil {
	//		log.Debugf("CheckDoubleSpends error: %s", err.Error())
	//		return
	//	}
	//	if len(dubs) > 0 {
	//		for i, dub := range dubs {
	//			fmt.Debugf("dub %d known tx %s and new tx %s are exclusive!!!\n",
	//				i, dub.String(), m.TxSha().String())
	//		}
	//	}
	hits, err := p.TS.Ingest(m, height)
	if err != nil {
		log.Errorf("Incoming Tx error: %s\n", err.Error())
		return
	}
	if hits == 0 {
		log.Debugf("Tx %s from %s had no hits, filter false positive.",
			m.TxSha().String(), p.con.RemoteAddr().String())
		p.fPositives <- 1 // add one false positive to chan
		return
	}
	p.UpdateFilterAndSend()
	log.Noticef("Tx %s ingested and matches %d utxo/adrs.",
		m.TxSha().String(), hits)
	//TODO: remove txid from map
}

// GetDataHandler responds to requests for tx data, which happen after
// advertising our txs via an inv message
func (p *Peer) GetDataHandler(m *wire.MsgGetData) {
	log.Debugf("Received getdata request from %s\n", p.con.RemoteAddr().String())
	var sent int32
	for i, thing := range m.InvList {
		log.Debugf("\t%d)%s : %s",
			i, thing.Type.String(), thing.Hash.String())

		if thing.Type == wire.InvTypeTx {
			tx, err := p.TS.db.Txns().Get(thing.Hash)
			if err != nil {
				log.Errorf("Error getting tx %s: %s",
					thing.Hash.String(), err.Error())
			}
			//tx.Flags = 0x00 // dewitnessify
			p.outMsgQueue <- tx
			sent++
			continue
		}
		// didn't match, so it's not something we're responding to
		log.Debugf("We only respond to tx requests, ignoring")

	}
	log.Debugf("Sent %d of %d requested items to %s", sent, len(m.InvList), p.con.RemoteAddr().String())
}

func (p *Peer) InvHandler(m *wire.MsgInv) {
	log.Debugf("Received inv message from %s\n", p.con.RemoteAddr().String())
	for _, thing := range m.InvList {
		if thing.Type == wire.InvTypeTx {
			// new tx, OK it at 0 and request
			p.OKMutex.Lock()
			p.OKTxids[thing.Hash] = 0
			p.AskForTx(thing.Hash)
			p.OKMutex.Unlock()
		}
		if thing.Type == wire.InvTypeBlock { // new block what to do?
			switch {
			case p.TS.chainState == WAITING:
			// start getting headers
				p.AskForMerkleBlock(thing.Hash)
			default:
			// drop it as if its component particles had high thermal energies
				log.Debug("Received inv block but ignoring; not synched\n")
			}
		}
	}
}
