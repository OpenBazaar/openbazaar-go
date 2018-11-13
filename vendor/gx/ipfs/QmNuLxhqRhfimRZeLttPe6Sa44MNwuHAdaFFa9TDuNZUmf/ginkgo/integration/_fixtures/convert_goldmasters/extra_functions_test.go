package tmp

import (
	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("something less important", func() {

		strp := "hello!"
		somethingImportant(GinkgoT(), &strp)
	})
})

func somethingImportant(t GinkgoTInterface, message *string) {
	t.Log("Something important happened in a test: " + *message)
}
