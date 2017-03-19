package ping

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	inet "gx/ipfs/QmVtMT3fD7DzQNW7hdm6Xe6KPstzcggrhNpeVZ4422UpKK/go-libp2p-net"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	host "gx/ipfs/QmXzeAcmKDTfNZQBiyF22hQKuTK7P5z6MBBQLTk9bbiSUc/go-libp2p-host"
	u "gx/ipfs/QmZuY8aV7zbNXVy6DyN9SmnuH3o9nG852F4aTiSBpts8d1/go-ipfs-util"
)

var log = logging.Logger("ping")

const PingSize = 32

const ID = "/ipfs/ping/1.0.0"

const pingTimeout = time.Second * 60

type PingService struct {
	Host host.Host
}

func NewPingService(h host.Host) *PingService {
	ps := &PingService{h}
	h.SetStreamHandler(ID, ps.PingHandler)
	return ps
}

func (p *PingService) PingHandler(s inet.Stream) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	buf := make([]byte, PingSize)

	timer := time.NewTimer(pingTimeout)
	defer timer.Stop()

	go func() {
		select {
		case <-timer.C:
		case <-ctx.Done():
		}

		s.Close()
	}()

	for {
		_, err := io.ReadFull(s, buf)
		if err != nil {
			log.Debug(err)
			return
		}

		_, err = s.Write(buf)
		if err != nil {
			log.Debug(err)
			return
		}

		timer.Reset(pingTimeout)
	}
}

func (ps *PingService) Ping(ctx context.Context, p peer.ID) (<-chan time.Duration, error) {
	s, err := ps.Host.NewStream(ctx, p, ID)
	if err != nil {
		return nil, err
	}

	out := make(chan time.Duration)
	go func() {
		defer close(out)
		defer s.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				t, err := ping(s)
				if err != nil {
					log.Debugf("ping error: %s", err)
					return
				}

				ps.Host.Peerstore().RecordLatency(p, t)
				select {
				case out <- t:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

func ping(s inet.Stream) (time.Duration, error) {
	buf := make([]byte, PingSize)
	u.NewTimeSeededRand().Read(buf)

	before := time.Now()
	_, err := s.Write(buf)
	if err != nil {
		return 0, err
	}

	rbuf := make([]byte, PingSize)
	_, err = io.ReadFull(s, rbuf)
	if err != nil {
		return 0, err
	}

	if !bytes.Equal(buf, rbuf) {
		return 0, errors.New("ping packet was incorrect!")
	}

	return time.Since(before), nil
}
