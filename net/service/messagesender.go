package service

import (
	"context"
	"fmt"
	inet "gx/ipfs/QmQx1dHDDYENugYgqA22BaBrRfuv1coSsuPiM7rYh1wwGH/go-libp2p-net"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	ggio "github.com/gogo/protobuf/io"
	proto "github.com/gogo/protobuf/proto"
)

type messageSender struct {
	s         inet.Stream
	r         ggio.ReadCloser
	w         ggio.WriteCloser
	lk        sync.Mutex
	p         peer.ID
	service   *OpenBazaarService
	protoc    protocol.ID
	singleMes int
}

var ReadMessageTimeout = time.Minute
var ErrReadTimeout = fmt.Errorf("timed out reading response")

func (service *OpenBazaarService) messageSenderForPeer(p peer.ID, s *inet.Stream) *messageSender {
	service.senderlk.Lock()
	defer service.senderlk.Unlock()

	ms, ok := service.sender[p]
	if !ok {
		ms = service.newMessageSender(p)
		service.sender[p] = ms
	}
	if s != nil {
		// replace old stream
		if ms.s != nil {
			ms.s.Close()
		}
		ms.s = *s
		ms.r = ggio.NewDelimitedReader(ms.s, inet.MessageSizeMax)
		ms.w = ggio.NewDelimitedWriter(ms.s)
	}
	return ms
}

func (service *OpenBazaarService) newMessageSender(p peer.ID) *messageSender {
	return &messageSender{p: p, service: service, protoc: ProtocolOpenBazaar}
}

func (ms *messageSender) prep() error {
	if ms.s != nil {
		return nil
	}

	nstr, err := ms.service.host.NewStream(ms.service.ctx, ms.p, ms.protoc)
	if err != nil {
		return err
	}

	ms.r = ggio.NewDelimitedReader(nstr, inet.MessageSizeMax)
	ms.w = ggio.NewDelimitedWriter(nstr)
	ms.s = nstr

	return nil
}

// streamReuseTries is the number of times we will try to reuse a stream to a
// given peer before giving up and reverting to the old one-message-per-stream
// behaviour.
const streamReuseTries = 3

func (ms *messageSender) SendMessage(ctx context.Context, pmes proto.Message) error {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	if err := ms.prep(); err != nil {
		return err
	}

	if err := ms.writeMessage(pmes); err != nil {
		return err
	}

	if ms.singleMes > streamReuseTries {
		ms.s.Close()
		ms.s = nil
	}

	return nil
}

func (ms *messageSender) writeMessage(pmes proto.Message) error {
	err := ms.w.WriteMsg(pmes)
	if err != nil {
		// If the other side isnt expecting us to be reusing streams, we're gonna
		// end up erroring here. To make sure things work seamlessly, lets retry once
		// before continuing

		log.Infof("error writing message: ", err)
		ms.s.Close()
		ms.s = nil
		if err := ms.prep(); err != nil {
			return err
		}

		if err := ms.w.WriteMsg(pmes); err != nil {
			return err
		}

		// keep track of this happening. If it happens a few times, its
		// likely we can assume the otherside will never support stream reuse
		ms.singleMes++
	}
	return nil
}

func (ms *messageSender) SendRequest(ctx context.Context, pmes *pb.Message) (*pb.Message, error) {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	if err := ms.prep(); err != nil {
		return nil, err
	}

	if err := ms.writeMessage(pmes); err != nil {
		return nil, err
	}

	mes := new(pb.Message)
	if err := ms.ctxReadMsg(ctx, mes); err != nil {
		ms.s.Close()
		ms.s = nil
		return nil, err
	}

	if ms.singleMes > streamReuseTries {
		ms.s.Close()
		ms.s = nil
	}

	return mes, nil
}

func (ms *messageSender) ctxReadMsg(ctx context.Context, mes *pb.Message) error {
	errc := make(chan error, 1)
	go func(r ggio.ReadCloser) {
		errc <- r.ReadMsg(mes)
	}(ms.r)

	t := time.NewTimer(ReadMessageTimeout)
	defer t.Stop()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return ErrReadTimeout
	}
}
