package failing_before_suite_test

import (
	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
)

var _ = Describe("FailingBeforeSuite", func() {
	It("should run", func() {
		println("A TEST")
	})

	It("should run", func() {
		println("A TEST")
	})
})
