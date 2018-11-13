package crypto

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gx/ipfs/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/protocol"
)

var _ = Describe("NullAEAD", func() {
	It("selects the right FVN variant", func() {
		connID := protocol.ConnectionID([]byte{0x42, 0, 0, 0, 0, 0, 0, 0})
		Expect(NewNullAEAD(protocol.PerspectiveClient, connID, protocol.Version39)).To(Equal(&nullAEADFNV128a{
			perspective: protocol.PerspectiveClient,
		}))
		Expect(NewNullAEAD(protocol.PerspectiveClient, connID, protocol.VersionTLS)).To(BeAssignableToTypeOf(&aeadAESGCM{}))
	})
})
