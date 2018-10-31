package service

import (
	"context"
	"errors"
	host "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	ps "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	inet "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"io"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	ctxio "github.com/jbenet/go-context/io"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("service")

var ProtocolOpenBazaar protocol.ID = "/openbazaar/app/1.0.0"

type OpenBazaarService struct {
	host      host.Host
	self      peer.ID
	peerstore ps.Peerstore
	ctx       context.Context
	broadcast chan repo.Notifier
	datastore repo.Datastore
	node      *core.OpenBazaarNode
	sender    map[peer.ID]*messageSender
	senderlk  sync.Mutex
}

func New(node *core.OpenBazaarNode, datastore repo.Datastore) *OpenBazaarService {
	service := &OpenBazaarService{
		host:      node.IpfsNode.PeerHost.(host.Host),
		self:      node.IpfsNode.Identity,
		peerstore: node.IpfsNode.PeerHost.Peerstore(),
		ctx:       node.IpfsNode.Context(),
		broadcast: node.Broadcast,
		datastore: datastore,
		node:      node,
		sender:    make(map[peer.ID]*messageSender),
	}
	node.IpfsNode.PeerHost.SetStreamHandler(ProtocolOpenBazaar, service.HandleNewStream)
	log.Infof("OpenBazaar service running at %s", ProtocolOpenBazaar)
	return service
}

func (service *OpenBazaarService) DisconnectFromPeer(p peer.ID) error {
	log.Debugf("Disconnecting from %s", p.Pretty())
	service.senderlk.Lock()
	defer service.senderlk.Unlock()
	ms, ok := service.sender[p]
	if !ok {
		return nil
	}
	if ms != nil && ms.s != nil {
		ms.s.Close()
	}
	delete(service.sender, p)
	return nil
}

func (service *OpenBazaarService) HandleNewStream(s inet.Stream) {
	log.Debugf("handling stream from peer %s", s.Conn().RemotePeer())
	go service.handleNewMessage(s)
}

func (service *OpenBazaarService) handleNewMessage(s inet.Stream) {
	defer s.Close()
	// Check if banned
	var mPeer = s.Conn().RemotePeer()
	if service.node.BanManager.IsBanned(mPeer) {
		return
	}

	var (
		cr = ctxio.NewReader(service.ctx, s)
		r  = ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	)

	ms, err := service.messageSenderForPeer(mPeer)
	if err != nil {
		log.Error("Error getting message sender")
		return
	}

	for {
		select {
		// end loop on context close
		case <-service.ctx.Done():
			return
		default:
		}
		// Receive msg
		pmes := new(pb.Message)
		if err := r.ReadMsg(pmes); err != nil {
			s.Reset()
			if err == io.EOF {
				log.Debugf("Disconnected from peer %s", mPeer.Pretty())
			}
			return
		}

		if pmes.IsResponse {
			ms.requestlk.Lock()
			ch, ok := ms.requests[pmes.RequestId]
			if ok {
				// this is a request response
				select {
				case ch <- pmes:
					// message returned to requester
				case <-time.After(time.Second):
					// in case ch is closed on the other end - the lock should prevent this happening
					log.Debug("request id was not removed from map on timeout")
				}
				close(ch)
				delete(ms.requests, pmes.RequestId)
			} else {
				log.Debug("received response message with unknown request id: requesting function may have timed out")
			}
			ms.requestlk.Unlock()
			s.Reset()
			return
		}

		// Get handler for this msg type
		handler := service.HandlerForMsgType(pmes.MessageType)
		if handler == nil {
			s.Reset()
			log.Debug("Got back nil handler from handlerForMsgType")
			return
		}

		// Dispatch handler
		rpmes, err := handler(mPeer, pmes, nil)
		if err != nil {
			log.Debugf("%s handle message error: %s", pmes.MessageType.String(), err)
		}

		// If nil response, return it before serializing
		if rpmes == nil {
			continue
		}

		// give back request id
		rpmes.RequestId = pmes.RequestId
		rpmes.IsResponse = true

		// send out response msg
		if err := ms.SendMessage(service.ctx, rpmes); err != nil {
			s.Reset()
			log.Debugf("send response error: %s", err)
			return
		}
	}
}

func (service *OpenBazaarService) SendRequest(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Sending %s request to %s", pmes.MessageType.String(), p.Pretty())
	ms, err := service.messageSenderForPeer(p)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	rpmes, err := ms.SendRequest(cancelCtx, pmes)
	if err != nil {
		log.Debugf("No response from %s", p.Pretty())
		return nil, err
	}

	if rpmes == nil {
		log.Debugf("No response from %s", p.Pretty())
		return nil, errors.New("no response from peer")
	}

	log.Debugf("Received response from %s", p.Pretty())
	return rpmes, nil
}

func (service *OpenBazaarService) SendMessage(ctx context.Context, p peer.ID, pmes *pb.Message) error {
	if pmes.MessageType != pb.Message_BLOCK {
		log.Debugf("Sending %s message to %s", pmes.MessageType.String(), p.Pretty())
	}

	ms, err := service.messageSenderForPeer(p)
	if err != nil {
		return err
	}

	// Add new p2p-circuit address for all people [extreme hack]
	newAddr, err := ma.NewMultiaddr("/ip4/138.68.5.113/tcp/9005/ws/ipfs/QmRmZGBZNorXMSiNfwx5Z1pbDMoN4nBCTK4KrbvUbfAfMp/p2p-circuit/ipfs/" + peer.IDB58Encode(ms.p))
	if err != nil {
		return err
	}
	service.host.Peerstore().AddAddr(ms.p, newAddr, ps.PermanentAddrTTL)

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := ms.SendMessage(cancelCtx, pmes); err != nil {
		return err
	}
	return nil
}
