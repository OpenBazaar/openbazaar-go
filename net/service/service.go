package service

import (
	"errors"
	"github.com/ipfs/go-ipfs/core"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/ipfs/go-ipfs/commands"
	"github.com/op/go-logging"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	protocol "gx/ipfs/QmVL44QeoQDTYK8RVdpkyja7uYcK3WDNoBNHVLonf9YDtm/go-libp2p/p2p/protocol"
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	host "gx/ipfs/QmVL44QeoQDTYK8RVdpkyja7uYcK3WDNoBNHVLonf9YDtm/go-libp2p/p2p/host"
	inet "gx/ipfs/QmVL44QeoQDTYK8RVdpkyja7uYcK3WDNoBNHVLonf9YDtm/go-libp2p/p2p/net"
	ctxio "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-context/io"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
)

var log = logging.MustGetLogger("service")

var ProtocolOpenBazaar protocol.ID = "/app/openbazaar"

type OpenBazaarService struct {
	host       host.Host
	self       peer.ID
	peerstore  peer.Peerstore
	cmdCtx     commands.Context
	ctx        context.Context
	broadcast  chan []byte
	datastore  repo.Datastore
}

var OBService *OpenBazaarService

func SetupOpenBazaarService(node *core.IpfsNode, broadcast chan []byte, ctx commands.Context, datastore repo.Datastore) *OpenBazaarService {
	OBService = &OpenBazaarService {
		host: node.PeerHost.(host.Host),
		self: node.Identity,
		peerstore: node.PeerHost.Peerstore(),
		cmdCtx: ctx,
		ctx: node.Context(),
		broadcast: broadcast,
		datastore: datastore,
	}
	node.PeerHost.SetStreamHandler(ProtocolOpenBazaar, OBService.HandleNewStream)
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
		return
	}

	// send out response msg
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

	cr := ctxio.NewReader(ctx, s) // ok to use. we defer close stream in this func
	cw := ctxio.NewWriter(ctx, s) // ok to use. we defer close stream in this func
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
	// TODO: build this out
	return nil
}
