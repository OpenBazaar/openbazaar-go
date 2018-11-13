package stream_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	smux "gx/ipfs/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
	insecure "gx/ipfs/QmZ3XKH272gU9px86XqWYeZHU65ayHxWs6Wbswvdj2VqVK/go-conn-security/insecure"
	manet "gx/ipfs/Qmaabb1tJZ2CX5cp6MuuiGgns71NYoxdgQP6Xdid1dVceC/go-multiaddr-net"
	mplex "gx/ipfs/QmaveCPGVaKJU57tBErGCDjzLaqEMZkFygoiv4BhYwWUGc/go-smux-multiplex"
	tpt "gx/ipfs/QmbCkisBsdejwSzusQcdbYjpSX3yvUw1ek2YSsJ89QbZYX/go-libp2p-transport"
	st "gx/ipfs/QmeUjhpfGkrMtE6s7JjU2xLhfzprSDh16QUxGVr3wTrKSx/go-libp2p-transport-upgrader"

	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
	. "gx/ipfs/QmUWtNQd8JdEiYiDqNYTUcaqyteJZ2rTNQLiw3dauLPccy/gomega"
)

// negotiatingMuxer sets up a new mplex connection
// It makes sure that this happens at the same time for client and server.
type negotiatingMuxer struct{}

func (m *negotiatingMuxer) NewConn(c net.Conn, isServer bool) (smux.Conn, error) {
	var err error
	// run a fake muxer negotiation
	if isServer {
		_, err = c.Write([]byte("setup"))
	} else {
		_, err = c.Read(make([]byte, 5))
	}
	if err != nil {
		return nil, err
	}
	return mplex.DefaultTransport.NewConn(c, isServer)
}

// blockingMuxer blocks the muxer negotiation until the contain chan is closed
type blockingMuxer struct {
	unblock chan struct{}
}

var _ smux.Transport = &blockingMuxer{}

func newBlockingMuxer() *blockingMuxer { return &blockingMuxer{unblock: make(chan struct{})} }
func (m *blockingMuxer) NewConn(c net.Conn, isServer bool) (smux.Conn, error) {
	<-m.unblock
	return (&negotiatingMuxer{}).NewConn(c, isServer)
}
func (m *blockingMuxer) Unblock() { close(m.unblock) }

// errorMuxer is a muxer that errors while setting up
type errorMuxer struct{}

var _ smux.Transport = &errorMuxer{}

func (m *errorMuxer) NewConn(c net.Conn, isServer bool) (smux.Conn, error) {
	return nil, errors.New("mux error")
}

var _ = Describe("Listener", func() {
	var (
		defaultUpgrader = &st.Upgrader{
			Secure: insecure.New(peer.ID(1)),
			Muxer:  &negotiatingMuxer{},
		}
	)

	testConn := func(clientConn, serverConn tpt.Conn) {
		cstr, err := clientConn.OpenStream()
		ExpectWithOffset(0, err).ToNot(HaveOccurred())
		_, err = cstr.Write([]byte("foobar"))
		ExpectWithOffset(0, err).ToNot(HaveOccurred())
		sstr, err := serverConn.AcceptStream()
		ExpectWithOffset(0, err).ToNot(HaveOccurred())
		b := make([]byte, 6)
		_, err = sstr.Read(b)
		ExpectWithOffset(0, err).ToNot(HaveOccurred())
		ExpectWithOffset(0, b).To(Equal([]byte("foobar")))
	}

	createListener := func(upgrader *st.Upgrader) tpt.Listener {
		addr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		ExpectWithOffset(0, err).ToNot(HaveOccurred())
		ln, err := manet.Listen(addr)
		ExpectWithOffset(0, err).ToNot(HaveOccurred())
		return upgrader.UpgradeListener(nil, ln)
	}

	dial := func(upgrader *st.Upgrader, raddr ma.Multiaddr, p peer.ID) (tpt.Conn, error) {
		macon, err := manet.Dial(raddr)
		if err != nil {
			return nil, err
		}
		return upgrader.UpgradeOutbound(context.Background(), nil, macon, p)
	}

	BeforeEach(func() {
		tpt.AcceptTimeout = time.Hour
	})

	It("accepts a single connection", func() {
		ln := createListener(defaultUpgrader)
		cconn, err := dial(defaultUpgrader, ln.Multiaddr(), peer.ID(1))
		Expect(err).ToNot(HaveOccurred())
		sconn, err := ln.Accept()
		Expect(err).ToNot(HaveOccurred())
		testConn(cconn, sconn)
	})

	It("accepts multiple connections", func() {
		ln := createListener(defaultUpgrader)
		const num = 10
		for i := 0; i < 10; i++ {
			cconn, err := dial(defaultUpgrader, ln.Multiaddr(), peer.ID(1))
			Expect(err).ToNot(HaveOccurred())
			sconn, err := ln.Accept()
			Expect(err).ToNot(HaveOccurred())
			testConn(cconn, sconn)
		}
	})

	It("closes connections if they are not accepted", func() {
		const timeout = 200 * time.Millisecond
		tpt.AcceptTimeout = timeout
		ln := createListener(defaultUpgrader)
		conn, err := dial(defaultUpgrader, ln.Multiaddr(), peer.ID(2))
		Expect(err).ToNot(HaveOccurred())
		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			str, err := conn.OpenStream()
			Expect(err).ToNot(HaveOccurred())
			// start a Read. It will block until the connection is closed
			str.Read([]byte{0})
			close(done)
		}()
		Consistently(done, timeout/2).ShouldNot(BeClosed())
		Eventually(done, timeout).Should(BeClosed())
	})

	It("doesn't accept connections that fail to setup", func() {
		upgrader := &st.Upgrader{
			Secure: insecure.New(peer.ID(1)),
			Muxer:  &errorMuxer{},
		}
		ln := createListener(upgrader)
		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			_, _ = ln.Accept()
			close(done)
		}()
		_, _ = dial(defaultUpgrader, ln.Multiaddr(), peer.ID(2))
		Consistently(done).ShouldNot(BeClosed())
		// make the goroutine return
		ln.Close()
		Eventually(done).Should(BeClosed())
	})

	Context("concurrency", func() {
		It("sets up connections concurrently", func() {
			num := 3 * st.AcceptQueueLength
			bm := newBlockingMuxer()
			upgrader := &st.Upgrader{
				Secure: insecure.New(peer.ID(1)),
				Muxer:  bm,
			}
			ln := createListener(upgrader)
			accepted := make(chan tpt.Conn, num)
			go func() {
				defer GinkgoRecover()
				for {
					conn, err := ln.Accept()
					if err != nil {
						return
					}
					accepted <- conn
				}
			}()
			var wg sync.WaitGroup
			// start num dials, which all block while setting up the muxer
			for i := 0; i < num; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					_, err := dial(defaultUpgrader, ln.Multiaddr(), peer.ID(2))
					Expect(err).ToNot(HaveOccurred())
					wg.Done()
				}()
			}
			// the dials are still blocked, so we shouldn't have any connection available yet
			Consistently(accepted).Should(BeEmpty())
			bm.Unblock() // make all dials succeed
			Eventually(accepted).Should(HaveLen(num))
			wg.Wait()
		})

		It("stops setting up when the more than AcceptQueueLength connections are waiting to get accepted", func() {
			ln := createListener(defaultUpgrader)
			// setup AcceptQueueLength connections, but don't accept any of them
			dialed := make(chan struct{}, 10*st.AcceptQueueLength) // used as a thread-safe counter
			for i := 0; i < st.AcceptQueueLength; i++ {
				go func() {
					defer GinkgoRecover()
					_, err := dial(defaultUpgrader, ln.Multiaddr(), peer.ID(2))
					Expect(err).ToNot(HaveOccurred())
					dialed <- struct{}{}
				}()
			}
			Eventually(dialed).Should(HaveLen(st.AcceptQueueLength))
			// dial a new connection. This connection should not complete setup, since the queue is full
			go func() {
				defer GinkgoRecover()
				_, err := dial(defaultUpgrader, ln.Multiaddr(), peer.ID(2))
				Expect(err).ToNot(HaveOccurred())
				dialed <- struct{}{}
			}()
			Consistently(dialed).Should(HaveLen(st.AcceptQueueLength))
			// accept a single connection. Now the new connection should be set up, and fill the queue again
			_, err := ln.Accept()
			Expect(err).ToNot(HaveOccurred())
			Eventually(dialed).Should(HaveLen(st.AcceptQueueLength + 1))
		})
	})

	Context("closing", func() {
		It("unblocks Accept when it is closed", func() {
			ln := createListener(defaultUpgrader)
			done := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				_, err := ln.Accept()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("use of closed network connection"))
				close(done)
			}()
			Consistently(done).ShouldNot(BeClosed())
			Expect(ln.Close()).To(Succeed())
			Eventually(done).Should(BeClosed())
		})

		It("doesn't accept new connections when it is closed", func() {
			ln := createListener(defaultUpgrader)
			Expect(ln.Close()).To(Succeed())
			_, err := dial(defaultUpgrader, ln.Multiaddr(), peer.ID(1))
			Expect(err).To(HaveOccurred())
		})

		It("closes incoming connections that have not yet been accepted", func() {
			ln := createListener(defaultUpgrader)
			conn, err := dial(defaultUpgrader, ln.Multiaddr(), peer.ID(2))
			Expect(conn.IsClosed()).To(BeFalse())
			Expect(err).ToNot(HaveOccurred())
			Expect(ln.Close()).To(Succeed())
			Eventually(conn.IsClosed).Should(BeTrue())
		})
	})
})
