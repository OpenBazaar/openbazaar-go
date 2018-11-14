package quic

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gx/ipfs/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/protocol"

	"testing"
)

func TestQuicGo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QUIC Suite")
}

const (
	versionGQUICFrames = protocol.Version39
	versionIETFFrames  = protocol.VersionTLS
)

var mockCtrl *gomock.Controller

var _ = BeforeSuite(func() {
	Expect(versionGQUICFrames.CryptoStreamID()).To(Equal(protocol.StreamID(1)))
	Expect(versionGQUICFrames.UsesIETFFrameFormat()).To(BeFalse())
	Expect(versionIETFFrames.CryptoStreamID()).To(Equal(protocol.StreamID(0)))
	Expect(versionIETFFrames.UsesIETFFrameFormat()).To(BeTrue())
})

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
