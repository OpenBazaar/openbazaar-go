package service

import (
	"context"
	"fmt"
	inet "gx/ipfs/QmNa31VPzC561NWwRsJLE7nGYZYuuD2QfpK2b1q9BK54J1/go-libp2p-net"
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
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

var ReadMessageTimeout = time.Minute
var ErrReadTimeout = fmt.Errorf("timed out reading response")

func (service *OpenBazaarService) messageSenderForPeer(p peer.ID) (*messageSender, error) {
	service.senderlk.Lock()
	ms, ok := service.sender[p]
	if ok {
		service.senderlk.Unlock()
		return ms, nil
	}
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

func (service *OpenBazaarService) newMessageSender(p peer.ID) *messageSender {
	return &messageSender{
		p:        p,
		service:  service,
		requests: make(map[int32]chan *pb.Message, 2), // low initial capacity
	}
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

	nstr, err := ms.service.host.NewStream(ms.service.ctx, ms.p, ProtocolOpenBazaar)
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

func (ms *messageSender) SendMessage(ctx context.Context, pmes *pb.Message) error {
	ms.lk.Lock()
	defer ms.lk.Unlock()
	retry := false
	for {
		if err := ms.prep(); err != nil {
			return err
		}

		if err := ms.w.WriteMsg(pmes); err != nil {
			ms.s.Reset()
			ms.s = nil

			if retry {
				return err
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

		return nil
	}
}

func (ms *messageSender) SendRequest(ctx context.Context, pmes *pb.Message) (*pb.Message, error) {
	pmes.RequestId = rand.Int31()
	returnChan := make(chan *pb.Message)
	ms.requestlk.Lock()
	ms.requests[pmes.RequestId] = returnChan
	ms.requestlk.Unlock()

	ms.lk.Lock()
	defer ms.lk.Unlock()
	retry := false
	for {
		if err := ms.prep(); err != nil {
			return nil, err
		}

		if err := ms.w.WriteMsg(pmes); err != nil {
			ms.s.Reset()
			ms.s = nil

			if retry {
				return nil, err
			} else {
				retry = true
				continue
			}
		}

		mes, err := ms.ctxReadMsg(ctx, returnChan);
		if err != nil {
			ms.s.Reset()
			ms.s = nil
			return nil, err
		}

		if ms.singleMes > streamReuseTries {
			ms.s.Close()
			ms.s = nil
		} else if retry {
			ms.singleMes++
		}

		return mes, nil
	}
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
