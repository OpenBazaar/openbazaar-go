package reconnect

import (
	"context"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	bhost "gx/ipfs/QmUDTcnDp2WssbmiDLC6aYurUeyt7QeRakHUQMxA2mZ5iB/go-libp2p/p2p/host/basic"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	swarmt "gx/ipfs/QmVHhT8NxtApPTndiZPe4JNGNUxGWtJe3ebyxtRz4HnbEp/go-libp2p-swarm/testing"
	inet "gx/ipfs/QmXuRkCR7BNQa9uqfpTiFWsTQLzmTWYg91Ja1w95gnqb6u/go-libp2p-net"
	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	host "gx/ipfs/QmdJfsSbKSZnMkfZ1kpopiyB9i3Hd6cp8VKWZmtWPa7Moc/go-libp2p-host"
)

var log = logging.Logger("reconnect")

func EchoStreamHandler(stream inet.Stream) {
	c := stream.Conn()
	log.Debugf("%s echoing %s", c.LocalPeer(), c.RemotePeer())
	go func() {
		_, err := io.Copy(stream, stream)
		if err == nil {
			stream.Close()
		} else {
			stream.Reset()
		}
	}()
}

type sendChans struct {
	send   chan struct{}
	sent   chan struct{}
	read   chan struct{}
	close_ chan struct{}
	closed chan struct{}
}

func newSendChans() sendChans {
	return sendChans{
		send:   make(chan struct{}),
		sent:   make(chan struct{}),
		read:   make(chan struct{}),
		close_: make(chan struct{}),
		closed: make(chan struct{}),
	}
}

func newSender() (chan sendChans, func(s inet.Stream)) {
	scc := make(chan sendChans)
	return scc, func(s inet.Stream) {
		sc := newSendChans()
		scc <- sc

		defer func() {
			s.Close()
			sc.closed <- struct{}{}
		}()

		buf := make([]byte, 65536)
		buf2 := make([]byte, 65536)
		u.NewTimeSeededRand().Read(buf)

		for {
			select {
			case <-sc.close_:
				return
			case <-sc.send:
			}

			// send a randomly sized subchunk
			from := rand.Intn(len(buf) / 2)
			to := rand.Intn(len(buf) / 2)
			sendbuf := buf[from : from+to]

			log.Debugf("sender sending %d bytes", len(sendbuf))
			n, err := s.Write(sendbuf)
			if err != nil {
				log.Debug("sender error. exiting:", err)
				return
			}

			log.Debugf("sender wrote %d bytes", n)
			sc.sent <- struct{}{}

			if n, err = io.ReadFull(s, buf2[:len(sendbuf)]); err != nil {
				log.Debug("sender error. failed to read:", err)
				return
			}

			log.Debugf("sender read %d bytes", n)
			sc.read <- struct{}{}
		}
	}
}

// TestReconnect tests whether hosts are able to disconnect and reconnect.
func TestReconnect2(t *testing.T) {
	ctx := context.Background()
	h1 := bhost.New(swarmt.GenSwarm(t, ctx))
	h2 := bhost.New(swarmt.GenSwarm(t, ctx))
	hosts := []host.Host{h1, h2}

	h1.SetStreamHandler(protocol.TestingID, EchoStreamHandler)
	h2.SetStreamHandler(protocol.TestingID, EchoStreamHandler)

	rounds := 8
	if testing.Short() {
		rounds = 4
	}
	for i := 0; i < rounds; i++ {
		log.Debugf("TestReconnect: %d/%d\n", i, rounds)
		SubtestConnSendDisc(t, hosts)
	}
}

// TestReconnect tests whether hosts are able to disconnect and reconnect.
func TestReconnect5(t *testing.T) {
	ctx := context.Background()
	h1 := bhost.New(swarmt.GenSwarm(t, ctx))
	h2 := bhost.New(swarmt.GenSwarm(t, ctx))
	h3 := bhost.New(swarmt.GenSwarm(t, ctx))
	h4 := bhost.New(swarmt.GenSwarm(t, ctx))
	h5 := bhost.New(swarmt.GenSwarm(t, ctx))
	hosts := []host.Host{h1, h2, h3, h4, h5}

	h1.SetStreamHandler(protocol.TestingID, EchoStreamHandler)
	h2.SetStreamHandler(protocol.TestingID, EchoStreamHandler)
	h3.SetStreamHandler(protocol.TestingID, EchoStreamHandler)
	h4.SetStreamHandler(protocol.TestingID, EchoStreamHandler)
	h5.SetStreamHandler(protocol.TestingID, EchoStreamHandler)

	rounds := 4
	if testing.Short() {
		rounds = 2
	}
	for i := 0; i < rounds; i++ {
		log.Debugf("TestReconnect: %d/%d\n", i, rounds)
		SubtestConnSendDisc(t, hosts)
	}
}

func SubtestConnSendDisc(t *testing.T, hosts []host.Host) {

	ctx := context.Background()
	numStreams := 3 * len(hosts)
	numMsgs := 10

	if testing.Short() {
		numStreams = 5 * len(hosts)
		numMsgs = 4
	}

	ss, sF := newSender()

	for _, h1 := range hosts {
		for _, h2 := range hosts {
			if h1.ID() >= h2.ID() {
				continue
			}

			h2pi := h2.Peerstore().PeerInfo(h2.ID())
			log.Debugf("dialing %s", h2pi.Addrs)
			if err := h1.Connect(ctx, h2pi); err != nil {
				t.Fatal("Failed to connect:", err)
			}
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < numStreams; i++ {
		h1 := hosts[i%len(hosts)]
		h2 := hosts[(i+1)%len(hosts)]
		s, err := h1.NewStream(context.Background(), h2.ID(), protocol.TestingID)
		if err != nil {
			t.Error(err)
		}

		wg.Add(1)
		go func(j int) {
			defer wg.Done()

			go sF(s)
			log.Debugf("getting handle %d", j)
			sc := <-ss // wait to get handle.
			log.Debugf("spawning worker %d", j)

			for k := 0; k < numMsgs; k++ {
				sc.send <- struct{}{}
				<-sc.sent
				log.Debugf("%d sent %d", j, k)
				<-sc.read
				log.Debugf("%d read %d", j, k)
			}
			sc.close_ <- struct{}{}
			<-sc.closed
			log.Debugf("closed %d", j)
		}(i)
	}
	wg.Wait()

	for i, h1 := range hosts {
		log.Debugf("host %d has %d conns", i, len(h1.Network().Conns()))
	}

	for _, h1 := range hosts {
		// close connection
		cs := h1.Network().Conns()
		for _, c := range cs {
			if c.LocalPeer() > c.RemotePeer() {
				continue
			}
			log.Debugf("closing: %s", c)
			c.Close()
		}
	}

	<-time.After(20 * time.Millisecond)

	for i, h := range hosts {
		if len(h.Network().Conns()) > 0 {
			t.Fatalf("host %d %s has %d conns! not zero.", i, h.ID(), len(h.Network().Conns()))
		}
	}
}
