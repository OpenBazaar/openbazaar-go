package service

import (
	"context"
	"errors"
	inet "gx/ipfs/QmVtMT3fD7DzQNW7hdm6Xe6KPstzcggrhNpeVZ4422UpKK/go-libp2p-net"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	host "gx/ipfs/QmXzeAcmKDTfNZQBiyF22hQKuTK7P5z6MBBQLTk9bbiSUc/go-libp2p-host"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	ps "gx/ipfs/Qme1g4e3m2SmdiSGGU3vSWmUStwUjc5oECnEriaK9Xa1HU/go-libp2p-peerstore"
	"io"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/ipfs/go-ipfs/commands"
	ctxio "github.com/jbenet/go-context/io"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("service")

var ProtocolOpenBazaar protocol.ID = "/openbazaar/app/1.0.0"

type OpenBazaarService struct {
	host      host.Host
	self      peer.ID
	peerstore ps.Peerstore
	cmdCtx    commands.Context
	ctx       context.Context
	broadcast chan interface{}
	datastore repo.Datastore
	node      *core.OpenBazaarNode
	sender    map[peer.ID]*messageSender
	senderlk  sync.Mutex
}

func New(node *core.OpenBazaarNode, ctx commands.Context, datastore repo.Datastore) *OpenBazaarService {
	service := &OpenBazaarService{
		host:      node.IpfsNode.PeerHost.(host.Host),
		self:      node.IpfsNode.Identity,
		peerstore: node.IpfsNode.PeerHost.Peerstore(),
		cmdCtx:    ctx,
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

func (service *OpenBazaarService) HandleNewStream(s inet.Stream) {
	go service.handleNewMessage(s)
}

// listen on a stream until we need to send over it, then start listening again when sending is finished
func (service *OpenBazaarService) handleNewMessage(s inet.Stream) {
	mPeer := s.Conn().RemotePeer()
	// Check if banned
	if service.node.BanManager.IsBanned(mPeer) {
		return
	}
	var cr ctxio.Reader
	var r ggio.ReadCloser
	// ensure the message sender for this peer is updated with this stream, so we reply over it
	ms := service.messageSenderForPeer(mPeer, &s)

	defer s.Close()
	for {
		ctx, cancel := context.WithCancel(service.ctx)
		defer cancel()
		cr = ctxio.NewReader(ctx, s)
		r = ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
		ms.cancelHandler = cancel
		// Receive msg
		pmes := new(pb.Message)
		ms.lk.Lock()                            // prevent outbound messages or wait for them to complete before continuing
		if err := r.ReadMsg(pmes); err != nil { // blocks until incoming or outgoing message
			switch err {
			case io.EOF:
				// stream closed, end handler
				ms.lk.Unlock()
				return
			case ctx.Err():
				// we need to send a message, briefly stop handling incoming streams
				ms.lk.Unlock()
				continue
			default:
				ms.lk.Unlock()
				log.Errorf("Error unmarshaling data: %s", err)
				continue
			}
		}

		// Get handler for this msg type
		handler := service.HandlerForMsgType(pmes.MessageType)
		if handler == nil {
			log.Debug("Got back nil handler from handlerForMsgType")
			ms.lk.Unlock()
			return
		}

		// Dispatch handler
		rpmes, err := handler(mPeer, pmes, nil)
		if err != nil {
			log.Debugf("handle message error: %s", err)
			ms.lk.Unlock()
			return
		}

		// If nil response, return it before serializing
		if rpmes == nil {
			ms.lk.Unlock()
			continue
		}

		// Send out response msg
		if err := ms.SendMessageWithoutLock(service.ctx, pmes); err != nil {
			log.Debugf("send response error: %s", err)
			ms.lk.Unlock()
			return
		}
		ms.lk.Unlock() // allow message
		select {
		// end loop on context close
		case <-service.ctx.Done():
			return
		}
	}
}

func (service *OpenBazaarService) SendRequest(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Sending %s request to %s", pmes.MessageType.String(), p.Pretty())
	ms := service.messageSenderForPeer(p, nil)

	rpmes, err := ms.SendRequest(ctx, pmes)
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
	log.Debugf("Sending %s message to %s", pmes.MessageType.String(), p.Pretty())
	ms := service.messageSenderForPeer(p, nil)

	if err := ms.SendMessage(ctx, pmes); err != nil {
		return err
	}
	return nil
}
