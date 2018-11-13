package handshake

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gx/ipfs/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/protocol"
)

var _ = Describe("Ephermal KEX", func() {
	It("has a consistent KEX", func() {
		kex1, err := getEphermalKEX()
		Expect(err).ToNot(HaveOccurred())
		Expect(kex1).ToNot(BeNil())
		kex2, err := getEphermalKEX()
		Expect(err).ToNot(HaveOccurred())
		Expect(kex2).ToNot(BeNil())
		Expect(kex1).To(Equal(kex2))
	})

	It("changes KEX", func() {
		kexLifetime = 10 * time.Millisecond
		defer func() {
			kexLifetime = protocol.EphermalKeyLifetime
		}()
		kex, err := getEphermalKEX()
		Expect(err).ToNot(HaveOccurred())
		Expect(kex).ToNot(BeNil())
		time.Sleep(kexLifetime)
		kex2, err := getEphermalKEX()
		Expect(err).ToNot(HaveOccurred())
		Expect(kex2).ToNot(Equal(kex))
	})
})
