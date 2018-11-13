package libp2pquic

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"time"

	ic "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	tpt "gx/ipfs/QmbCkisBsdejwSzusQcdbYjpSX3yvUw1ek2YSsJ89QbZYX/go-libp2p-transport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Connection", func() {
	var (
		serverKey, clientKey ic.PrivKey
		serverID, clientID   peer.ID
	)

	createPeer := func() (peer.ID, ic.PrivKey) {
		key, err := rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).ToNot(HaveOccurred())
		priv, err := ic.UnmarshalRsaPrivateKey(x509.MarshalPKCS1PrivateKey(key))
		Expect(err).ToNot(HaveOccurred())
		id, err := peer.IDFromPrivateKey(priv)
		Expect(err).ToNot(HaveOccurred())
		return id, priv
	}

	runServer := func(tr tpt.Transport, multiaddr string) (ma.Multiaddr, <-chan tpt.Conn) {
		addrChan := make(chan ma.Multiaddr)
		connChan := make(chan tpt.Conn)
		go func() {
			defer GinkgoRecover()
			addr, err := ma.NewMultiaddr(multiaddr)
			Expect(err).ToNot(HaveOccurred())
			ln, err := tr.Listen(addr)
			Expect(err).ToNot(HaveOccurred())
			addrChan <- ln.Multiaddr()
			conn, err := ln.Accept()
			Expect(err).ToNot(HaveOccurred())
			connChan <- conn
		}()
		return <-addrChan, connChan
	}

	// modify the cert chain such that verificiation will fail
	invalidateCertChain := func(tlsConf *tls.Config) {
		tlsConf.Certificates[0].Certificate = [][]byte{tlsConf.Certificates[0].Certificate[0]}
	}

	BeforeEach(func() {
		serverID, serverKey = createPeer()
		clientID, clientKey = createPeer()
	})

	It("handshakes on IPv4", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		serverConn := <-serverConnChan
		Expect(conn.LocalPeer()).To(Equal(clientID))
		Expect(conn.LocalPrivateKey()).To(Equal(clientKey))
		Expect(conn.RemotePeer()).To(Equal(serverID))
		Expect(conn.RemotePublicKey()).To(Equal(serverKey.GetPublic()))
		Expect(serverConn.LocalPeer()).To(Equal(serverID))
		Expect(serverConn.LocalPrivateKey()).To(Equal(serverKey))
		Expect(serverConn.RemotePeer()).To(Equal(clientID))
		Expect(serverConn.RemotePublicKey()).To(Equal(clientKey.GetPublic()))
	})

	It("handshakes on IPv6", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip6/::1/udp/0/quic")

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		serverConn := <-serverConnChan
		Expect(conn.LocalPeer()).To(Equal(clientID))
		Expect(conn.LocalPrivateKey()).To(Equal(clientKey))
		Expect(conn.RemotePeer()).To(Equal(serverID))
		Expect(conn.RemotePublicKey()).To(Equal(serverKey.GetPublic()))
		Expect(serverConn.LocalPeer()).To(Equal(serverID))
		Expect(serverConn.LocalPrivateKey()).To(Equal(serverKey))
		Expect(serverConn.RemotePeer()).To(Equal(clientID))
		Expect(serverConn.RemotePublicKey()).To(Equal(clientKey.GetPublic()))
	})

	It("opens and accepts streams", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		serverConn := <-serverConnChan

		str, err := conn.OpenStream()
		Expect(err).ToNot(HaveOccurred())
		_, err = str.Write([]byte("foobar"))
		Expect(err).ToNot(HaveOccurred())
		str.Close()
		sstr, err := serverConn.AcceptStream()
		Expect(err).ToNot(HaveOccurred())
		data, err := ioutil.ReadAll(sstr)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal([]byte("foobar")))
	})

	It("fails if the peer ID doesn't match", func() {
		thirdPartyID, _ := createPeer()

		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		// dial, but expect the wrong peer ID
		_, err = clientTransport.Dial(context.Background(), serverAddr, thirdPartyID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TLS handshake error: bad certificate"))
		Consistently(serverConnChan).ShouldNot(Receive())
	})

	It("fails if the client presents an invalid cert chain", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		clientTransport, err := NewTransport(clientKey)
		invalidateCertChain(clientTransport.(*transport).tlsConf)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool { return conn.IsClosed() }).Should(BeTrue())
		Consistently(serverConnChan).ShouldNot(Receive())
	})

	It("fails if the server presents an invalid cert chain", func() {
		serverTransport, err := NewTransport(serverKey)
		invalidateCertChain(serverTransport.(*transport).tlsConf)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		_, err = clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TLS handshake error: bad certificate"))
		Consistently(serverConnChan).ShouldNot(Receive())
	})

	It("keeps accepting connections after a failed connection attempt", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		// first dial with an invalid cert chain
		clientTransport1, err := NewTransport(clientKey)
		invalidateCertChain(clientTransport1.(*transport).tlsConf)
		Expect(err).ToNot(HaveOccurred())
		_, err = clientTransport1.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		Consistently(serverConnChan).ShouldNot(Receive())

		// then dial with a valid client
		clientTransport2, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		_, err = clientTransport2.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		Eventually(serverConnChan).Should(Receive())
	})

	It("dials to two servers at the same time", func() {
		serverID2, serverKey2 := createPeer()

		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")
		serverTransport2, err := NewTransport(serverKey2)
		Expect(err).ToNot(HaveOccurred())
		serverAddr2, serverConnChan2 := runServer(serverTransport2, "/ip4/127.0.0.1/udp/0/quic")

		data := bytes.Repeat([]byte{'a'}, 5*1<<20) // 5 MB
		// wait for both servers to accept a connection
		// then send some data
		go func() {
			for _, c := range []tpt.Conn{<-serverConnChan, <-serverConnChan2} {
				go func(conn tpt.Conn) {
					defer GinkgoRecover()
					str, err := conn.OpenStream()
					Expect(err).ToNot(HaveOccurred())
					defer str.Close()
					_, err = str.Write(data)
					Expect(err).ToNot(HaveOccurred())
				}(c)
			}
		}()

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		c1, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		c2, err := clientTransport.Dial(context.Background(), serverAddr2, serverID2)
		Expect(err).ToNot(HaveOccurred())

		done := make(chan struct{}, 2)
		// receive the data on both connections at the same time
		for _, c := range []tpt.Conn{c1, c2} {
			go func(conn tpt.Conn) {
				defer GinkgoRecover()
				str, err := conn.AcceptStream()
				Expect(err).ToNot(HaveOccurred())
				str.Close()
				d, err := ioutil.ReadAll(str)
				Expect(err).ToNot(HaveOccurred())
				Expect(d).To(Equal(data))
				conn.Close()
				done <- struct{}{}
			}(c)
		}

		Eventually(done, 5*time.Second).Should(Receive())
		Eventually(done, 5*time.Second).Should(Receive())
	})
})
