package libp2pquic

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ic "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	tpt "gx/ipfs/QmbCkisBsdejwSzusQcdbYjpSX3yvUw1ek2YSsJ89QbZYX/go-libp2p-transport"
)

var _ = Describe("Listener", func() {
	var t tpt.Transport

	BeforeEach(func() {
		rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).ToNot(HaveOccurred())
		key, err := ic.UnmarshalRsaPrivateKey(x509.MarshalPKCS1PrivateKey(rsaKey))
		Expect(err).ToNot(HaveOccurred())
		t, err = NewTransport(key)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("listening on the right address", func() {
		It("returns the address it is listening on", func() {
			localAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
			ln, err := t.Listen(localAddr)
			Expect(err).ToNot(HaveOccurred())
			netAddr := ln.Addr()
			Expect(netAddr).To(BeAssignableToTypeOf(&net.UDPAddr{}))
			port := netAddr.(*net.UDPAddr).Port
			Expect(port).ToNot(BeZero())
			Expect(ln.Multiaddr().String()).To(Equal(fmt.Sprintf("/ip4/127.0.0.1/udp/%d/quic", port)))
		})

		It("returns the address it is listening on, for listening on IPv4", func() {
			localAddr, err := ma.NewMultiaddr("/ip4/0.0.0.0/udp/0/quic")
			Expect(err).ToNot(HaveOccurred())
			ln, err := t.Listen(localAddr)
			Expect(err).ToNot(HaveOccurred())
			netAddr := ln.Addr()
			Expect(netAddr).To(BeAssignableToTypeOf(&net.UDPAddr{}))
			port := netAddr.(*net.UDPAddr).Port
			Expect(port).ToNot(BeZero())
			Expect(ln.Multiaddr().String()).To(Equal(fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", port)))
		})

		It("returns the address it is listening on, for listening on IPv6", func() {
			localAddr, err := ma.NewMultiaddr("/ip6/::/udp/0/quic")
			Expect(err).ToNot(HaveOccurred())
			ln, err := t.Listen(localAddr)
			Expect(err).ToNot(HaveOccurred())
			netAddr := ln.Addr()
			Expect(netAddr).To(BeAssignableToTypeOf(&net.UDPAddr{}))
			port := netAddr.(*net.UDPAddr).Port
			Expect(port).ToNot(BeZero())
			Expect(ln.Multiaddr().String()).To(Equal(fmt.Sprintf("/ip6/::/udp/%d/quic", port)))
		})
	})

	Context("accepting connections", func() {
		var localAddr ma.Multiaddr

		BeforeEach(func() {
			var err error
			localAddr, err = ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns Accept when it is closed", func() {
			addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
			Expect(err).ToNot(HaveOccurred())
			ln, err := t.Listen(addr)
			Expect(err).ToNot(HaveOccurred())
			done := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				ln.Accept()
				close(done)
			}()
			Consistently(done).ShouldNot(BeClosed())
			Expect(ln.Close()).To(Succeed())
			Eventually(done).Should(BeClosed())
		})

		It("doesn't accept Accept calls after it is closed", func() {
			ln, err := t.Listen(localAddr)
			Expect(err).ToNot(HaveOccurred())
			Expect(ln.Close()).To(Succeed())
			_, err = ln.Accept()
			Expect(err).To(HaveOccurred())
		})
	})
})
