package libp2pquic

import (
	"net"

	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("QUIC Multiaddr", func() {
	It("converts a net.Addr to a QUIC Multiaddr", func() {
		addr := &net.UDPAddr{IP: net.IPv4(192, 168, 0, 42), Port: 1337}
		maddr, err := toQuicMultiaddr(addr)
		Expect(err).ToNot(HaveOccurred())
		Expect(maddr.String()).To(Equal("/ip4/192.168.0.42/udp/1337/quic"))
	})

	It("converts a QUIC Multiaddr to a net.Addr", func() {
		maddr, err := ma.NewMultiaddr("/ip4/192.168.0.42/udp/1337/quic")
		Expect(err).ToNot(HaveOccurred())
		addr, err := fromQuicMultiaddr(maddr)
		Expect(err).ToNot(HaveOccurred())
		Expect(addr).To(BeAssignableToTypeOf(&net.UDPAddr{}))
		udpAddr := addr.(*net.UDPAddr)
		Expect(udpAddr.IP).To(Equal(net.IPv4(192, 168, 0, 42)))
		Expect(udpAddr.Port).To(Equal(1337))
	})
})
