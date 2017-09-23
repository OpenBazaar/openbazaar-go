package service

import (
	"context"
	"fmt"
	inet "gx/ipfs/QmNa31VPzC561NWwRsJLE7nGYZYuuD2QfpK2b1q9BK54J1/go-libp2p-net"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	"math/rand"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

type messageSender struct {
	s         inet.Stream
	w         ggio.WriteCloser
	lk        sync.Mutex
	p         peer.ID
	service   *OpenBazaarService
	protoc    protocol.ID
	singleMes int
	requests  map[int32]chan *pb.Message
	requestlk sync.Mutex
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
		ms.lk.Lock()
		if ms.s != nil {
			ms.s.Close()
		}
		ms.s = *s
		ms.w = ggio.NewDelimitedWriter(ms.s)
		ms.lk.Unlock()
	}
	return ms
}

func (service *OpenBazaarService) newMessageSender(p peer.ID) *messageSender {
	return &messageSender{
		p:        p,
		service:  service,
		protoc:   ProtocolOpenBazaar,
		requests: make(map[int32]chan *pb.Message, 2), // low initial capacity
	}
}

func (ms *messageSender) prep() error {
	if ms.s != nil {
		return nil
	}

	nstr, err := ms.service.host.NewStream(ms.service.ctx, ms.p, ms.protoc)
	if err != nil {
		return err
	}
	go ms.service.handleNewMessage(nstr, false)

	ms.w = ggio.NewDelimitedWriter(nstr)
	ms.s = nstr

	return nil
}

// streamReuseTries is the number of times we will try to reuse a stream to a
// given peer before giving up and reverting to the old one-message-per-stream
// behaviour.
const streamReuseTries = 3

func (ms *messageSender) SendMessage(ctx context.Context, pmes *pb.Message) error {
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

func (ms *messageSender) writeMessage(pmes *pb.Message) error {
	err := ms.w.WriteMsg(pmes)
	if err != nil {
		// If the other side isnt expecting us to be reusing streams, we're gonna
		// end up erroring here. To make sure things work seamlessly, lets retry once
		// before continuing

		log.Infof("error writing message: %s", err.Error())
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
	pmes.RequestId = rand.Int31()
	returnChan := make(chan *pb.Message)
	ms.requestlk.Lock()
	ms.requests[pmes.RequestId] = returnChan
	ms.requestlk.Unlock()

	if err := ms.SendMessage(ctx, pmes); err != nil {
		ms.closeRequest(pmes.RequestId)
		return nil, err
	}

	mes, err := ms.ctxReadMsg(ctx, returnChan)
	if err != nil {
		ms.closeRequest(pmes.RequestId)
		ms.s.Close()
		ms.s = nil
		return nil, err
	}
	// no need to close request here, it will have been done in the stream handler

	if ms.singleMes > streamReuseTries {
		ms.s.Close()
		ms.s = nil
	}
	return mes, nil
}

// stop listening for responses
func (ms *messageSender) closeRequest(id int32) {
	ms.requestlk.Lock()
	ch, ok := ms.requests[id]
	if ok {
		close(ch)
		delete(ms.requests, id)
	}
	ms.requestlk.Unlock()
}

func (ms *messageSender) ctxReadMsg(ctx context.Context, returnChan chan *pb.Message) (*pb.Message, error) {
	t := time.NewTimer(ReadMessageTimeout)
	defer t.Stop()

	select {
	case mes := <-returnChan:
		return mes, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.C:
		return nil, ErrReadTimeout
	}
}
