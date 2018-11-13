package nested

import (
	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("something less important", func() {

		whatever := &UselessStruct{}
		GinkgoT().Fail(whatever.ImportantField != "SECRET_PASSWORD")
	})
})
