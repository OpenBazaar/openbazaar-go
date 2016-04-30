package net

import (
	"github.com/ipfs/go-ipfs/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/ipfs/go-ipfs/commands"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	protocol "gx/ipfs/QmYgaiNVVL7f2nydijAwpDRunRkmxfu3PoK87Y3pH84uAW/go-libp2p/p2p/protocol"
	peer "gx/ipfs/QmZwZjMVGss5rqYsJVGy18gNbkTJffFyq2x1uJ4e4p3ZAt/go-libp2p-peer"
	host "gx/ipfs/QmYgaiNVVL7f2nydijAwpDRunRkmxfu3PoK87Y3pH84uAW/go-libp2p/p2p/host"
	inet "gx/ipfs/QmYgaiNVVL7f2nydijAwpDRunRkmxfu3PoK87Y3pH84uAW/go-libp2p/p2p/net"
	ctxio "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-context/io"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
)

var ProtocolOpenBazaar protocol.ID = "/app/openbazaar"

type OpenBazaarService struct {
	host      host.Host
	self      peer.ID
	peerstore peer.Peerstore
	cmdCtx    commands.Context
	ctx       context.Context
}

var OBService *OpenBazaarService

func SetupOpenBazaarService(node *core.IpfsNode, ctx commands.Context) *OpenBazaarService {
	OBService = &OpenBazaarService {
		host: node.PeerHost.(host.Host),
		self: node.Identity,
		peerstore: node.PeerHost.Peerstore(),
		cmdCtx: ctx,
		ctx: node.Context(),
	}
	node.PeerHost.SetStreamHandler(ProtocolOpenBazaar, OBService.handleNewStream)
	log.Infof("OpenBazaar service running at %s", ProtocolOpenBazaar)
	return OBService
}

func (service *OpenBazaarService) handleNewStream(s inet.Stream) {
	go service.handleNewMessage(s)
}

func (service *OpenBazaarService) handleNewMessage(s inet.Stream) {
	cr := ctxio.NewReader(service.ctx, s)
	cw := ctxio.NewWriter(service.ctx, s)
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(cw)
	mPeer := s.Conn().RemotePeer()

	// receive msg
	defer s.Close()
	pmes := new(pb.Message)
	if err := r.ReadMsg(pmes); err != nil {
		log.Errorf("Error unmarshaling data: %s", err)
	}

	// get handler for this msg type.
	handler := service.handlerForMsgType(pmes.MessageType)
	if handler == nil {
		log.Debug("Got back nil handler from handlerForMsgType")
		return
	}

	// dispatch handler.
	rpmes, err := handler(mPeer, pmes)
	if err != nil {
		log.Debugf("handle message error: %s", err)
		return
	}

	// if nil response, return it before serializing
	if rpmes == nil {
		log.Debug("Got back nil response from request.")
		return
	}

	// send out response msg
	if err := w.WriteMsg(rpmes); err != nil {
		log.Debugf("send response error: %s", err)
		return
	}
}

func (service *OpenBazaarService) sendRequest(ctx context.Context, p peer.ID, pmes *pb.Message) (*pb.Message, error) {
	// TODO: build this out
	return pmes, nil
}

func (service *OpenBazaarService) sendMessage(ctx context.Context, p peer.ID, pmes *pb.Message) error {
	// TODO: build this out
	return nil
}
