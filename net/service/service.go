package service

import (
	"errors"
	ps "gx/ipfs/QmQdnfvZQuhdT93LNc5bos52wAmdr3G2p6G8teLJMEN32P/go-libp2p-peerstore"
	peer "gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
	host "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/host"
	inet "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/net"
	protocol "gx/ipfs/QmVCe3SNMjkcPgnpFhZs719dheq6xE7gJwjzV7aWcUM4Ms/go-libp2p/p2p/protocol"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

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
	broadcast chan []byte
	datastore repo.Datastore
	node      *core.OpenBazaarNode
}

var OBService *OpenBazaarService

func SetupOpenBazaarService(node *core.OpenBazaarNode, ctx commands.Context, datastore repo.Datastore) *OpenBazaarService {
	OBService = &OpenBazaarService{
		host:      node.IpfsNode.PeerHost.(host.Host),
		self:      node.IpfsNode.Identity,
		peerstore: node.IpfsNode.PeerHost.Peerstore(),
		cmdCtx:    ctx,
		ctx:       node.IpfsNode.Context(),
		broadcast: node.Broadcast,
		datastore: datastore,
		node:      node,
	}
	node.IpfsNode.PeerHost.SetStreamHandler(ProtocolOpenBazaar, OBService.HandleNewStream)
	log.Infof("OpenBazaar service running at %s", ProtocolOpenBazaar)
	return OBService
}

func (service *OpenBazaarService) HandleNewStream(s inet.Stream) {
	go service.handleNewMessage(s)
}

func (service *OpenBazaarService) handleNewMessage(s inet.Stream) {
	cr := ctxio.NewReader(service.ctx, s)
	cw := ctxio.NewWriter(service.ctx, s)
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(cw)
	mPeer := s.Conn().RemotePeer()

	// Receive message
	defer s.Close()
	pmes := new(pb.Message)
	if err := r.ReadMsg(pmes); err != nil {
		log.Errorf("Error unmarshaling data: %s", err)
	}

	// Get handler for this message type
	handler := service.HandlerForMsgType(pmes.MessageType)
	if handler == nil {
		log.Debug("Got back nil handler from handlerForMsgType")
		return
	}

	// Dispatch handler
	rpmes, err := handler(mPeer, pmes, nil)
	if err != nil {
		log.Debugf("handle message error: %s", err)
		return
	}

	// If nil response, return it before serializing
	if rpmes == nil {
		return
	}

	// Send out response message
	if err := w.WriteMsg(rpmes); err != nil {
		log.Debugf("send response error: %s", err)
		return
	}
}

func (service *OpenBazaarService) SendRequest(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("Sending %s request to %s", pmes.MessageType.String(), p.Pretty())
	s, err := service.host.NewStream(ctx, ProtocolOpenBazaar, p)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	cr := ctxio.NewReader(ctx, s) // Ok to use. We defer close stream in this func.
	cw := ctxio.NewWriter(ctx, s) // Ok to use. We defer close stream in this func.
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(cw)

	if err := w.WriteMsg(pmes); err != nil {
		return nil, err
	}

	rpmes := new(pb.Message)
	if err := r.ReadMsg(rpmes); err != nil {
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
	s, err := service.host.NewStream(ctx, ProtocolOpenBazaar, p)
	if err != nil {
		return err
	}
	defer s.Close()

	cw := ctxio.NewWriter(ctx, s) // Ok to use. We defer close stream in this func.
	w := ggio.NewDelimitedWriter(cw)

	if err := w.WriteMsg(pmes); err != nil {
		return err
	}
	return nil
}
