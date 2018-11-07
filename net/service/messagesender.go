package service

import (
	"context"
	"fmt"
	inet "gx/ipfs/QmXfkENeeBvh3zYA51MaSdGUdBjhQ99cP5WQe8zgr6wchG/go-libp2p-net"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"math/rand"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

type messageSender struct {
	s         inet.Stream
	w         ggio.WriteCloser
	r         ggio.ReadCloser
	lk        sync.Mutex
	p         peer.ID
	service   *OpenBazaarService
	singleMes int
	invalid   bool
	requests  map[int32]chan *pb.Message
	requestlk sync.Mutex
}

var (
	ReadMessageTimeout = time.Minute * 5

	ErrContextTimeout  = fmt.Errorf("context timeout exceeded")
	ErrReadTimeout     = fmt.Errorf("timed out reading response")
	ErrRetriesExceeded = fmt.Errorf("maximum retries exceeded")
)

func (service *OpenBazaarService) messageSenderForPeer(p peer.ID) (*messageSender, error) {
	service.senderlk.Lock()
	ms, ok := service.sender[p]
	if ok {
		service.senderlk.Unlock()
		return ms, nil
	}
	log.Debugf("%s creating new sender", p)
	ms = &messageSender{p: p, service: service, requests: make(map[int32]chan *pb.Message, 2)}
	service.sender[p] = ms
	service.senderlk.Unlock()

	if err := ms.prepOrInvalidate(); err != nil {
		service.senderlk.Lock()
		defer service.senderlk.Unlock()

		if msCur, ok := service.sender[p]; ok {
			// Changed. Use the new one, old one is invalid and
			// not in the map so we can just throw it away.
			if ms != msCur {
				return msCur, nil
			}
			// Not changed, remove the now invalid stream from the
			// map.
			delete(service.sender, p)
		}
		// Invalid but not in map. Must have been removed by a disconnect.
		return nil, err
	}
	// All ready to go.
	return ms, nil
}

// invalidate is called before this messageSender is removed from the strmap.
// It prevents the messageSender from being reused/reinitialized and then
// forgotten (leaving the stream open).
func (ms *messageSender) invalidate() {
	ms.invalid = true
	if ms.s != nil {
		ms.s.Reset()
		ms.s = nil
	}
}

func (ms *messageSender) prepOrInvalidate() error {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	if err := ms.prep(); err != nil {
		ms.invalidate()
		return err
	}
	return nil
}

func (ms *messageSender) prep() error {
	if ms.invalid {
		return fmt.Errorf("message sender has been invalidated")
	}
	if ms.s != nil {
		return nil
	}

	log.Debugf("%s creating new stream", ms.p)
	ctx, cancel := ms.service.node.DefaultTimeoutContext(ms.service.ctx)
	defer cancel()
	nstr, err := ms.service.host.NewStream(ctx, ms.p, ProtocolOpenBazaar)
	if err != nil {
		return err
	}
	log.Debugf("%s stream created", ms.p)

	ms.r = ggio.NewDelimitedReader(nstr, inet.MessageSizeMax)
	ms.w = ggio.NewDelimitedWriter(nstr)
	ms.s = nstr

	return nil
}

// streamReuseTries is the number of times we will try to reuse a stream to a
// given peer before giving up and reverting to the old one-message-per-stream
// behavior.
const streamReuseTries = 3

func (ms *messageSender) SendMessage(ctx context.Context, pmes *pb.Message) error {
	log.Debugf("%s sending message", ms.p)
	ms.lk.Lock()
	defer ms.lk.Unlock()
	retry := false

	response := make(chan error, 1)

	go func() {
		for {
			if err := ms.prep(); err != nil {
				response <- err
				return
			}

			log.Debugf("%s writing message", ms.p)
			if err := ms.w.WriteMsg(pmes); err != nil {
				ms.s.Reset()
				ms.s = nil

				if retry {
					response <- err
					return
				} else {
					retry = true
					continue
				}
			}

			if ms.singleMes > streamReuseTries {
				ms.s.Close()
				ms.s = nil
			} else if retry {
				ms.singleMes++
			}

			response <- nil
			return
		}
	}()

	select {
	case r := <-response:
		log.Debugf("%s done sending message", ms.p)
		return r
	case <-ctx.Done():
		log.Debugf("%s timeout sending message", ms.p)
		return ErrContextTimeout
	}
}

func (ms *messageSender) SendRequest(ctx context.Context, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("%s sending request", ms.p)
	pmes.RequestId = rand.Int31()
	returnChan := make(chan *pb.Message)
	ms.requestlk.Lock()
	ms.requests[pmes.RequestId] = returnChan
	ms.requestlk.Unlock()

	defer ms.closeRequest(pmes.RequestId)

	ms.lk.Lock()
	defer ms.lk.Unlock()

	type readResponse struct {
		msg *pb.Message
		err error
	}
	response := make(chan readResponse, 1)
	retry := false

	go func() {
		for {
			if err := ms.prep(); err != nil {
				response <- readResponse{nil, err}
				return
			}

			log.Debugf("%s writing request", ms.p)
			if err := ms.w.WriteMsg(pmes); err != nil {
				ms.s.Reset()
				ms.s = nil

				if retry {
					response <- readResponse{nil, err}
					return
				} else {
					retry = true
					continue
				}
			}

			log.Debugf("%s reading response from request", ms.p)
			mes, err := ms.ctxReadMsg(ctx, returnChan)
			if err != nil {
				ms.s.Reset()
				ms.s = nil
				response <- readResponse{nil, err}
				return
			}

			if ms.singleMes > streamReuseTries {
				ms.s.Close()
				ms.s = nil
			} else if retry {
				ms.singleMes++
			}

			response <- readResponse{mes, nil}
			return
		}
	}()

	select {
	case r := <-response:
		log.Debugf("%s done sending request", ms.p)
		return r.msg, r.err
	case <-ctx.Done():
		log.Debugf("%s timeout sending request", ms.p)
		return nil, ErrContextTimeout
	}
}

// stop listening for responses
func (ms *messageSender) closeRequest(id int32) {
	log.Debugf("%s closing request", ms.p)
	ms.requestlk.Lock()
	ch, ok := ms.requests[id]
	if ok {
		close(ch)
		delete(ms.requests, id)
		log.Debugf("%s closed request", ms.p)
	} else {
		log.Debugf("%s request not found... abort close", ms.p)
	}
	ms.requestlk.Unlock()
}

func (ms *messageSender) ctxReadMsg(ctx context.Context, returnChan chan *pb.Message) (*pb.Message, error) {
	t := time.NewTimer(ReadMessageTimeout)
	defer t.Stop()

	select {
	case mes := <-returnChan:
		log.Debugf("%s context read responded", ms.p)
		return mes, nil
	case <-ctx.Done():
		log.Debugf("%s context read timeout", ms.p)
		return nil, ctx.Err()
	case <-t.C:
		log.Debugf("%s context read timer expire", ms.p)
		return nil, ErrReadTimeout
	}
}
