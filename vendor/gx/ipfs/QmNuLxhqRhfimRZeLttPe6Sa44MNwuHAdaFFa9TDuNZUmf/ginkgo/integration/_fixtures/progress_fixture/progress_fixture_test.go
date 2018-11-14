package progress_fixture_test

import (
	"fmt"

	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
)

var _ = Describe("ProgressFixture", func() {
	BeforeEach(func() {
		fmt.Fprintln(GinkgoWriter, ">outer before<")
	})

	JustBeforeEach(func() {
		fmt.Fprintln(GinkgoWriter, ">outer just before<")
	})

	AfterEach(func() {
		fmt.Fprintln(GinkgoWriter, ">outer after<")
	})

	Context("Inner Context", func() {
		BeforeEach(func() {
			fmt.Fprintln(GinkgoWriter, ">inner before<")
		})

		JustBeforeEach(func() {
			fmt.Fprintln(GinkgoWriter, ">inner just before<")
		})

		AfterEach(func() {
			fmt.Fprintln(GinkgoWriter, ">inner after<")
		})

		When("Inner When", func() {
			BeforeEach(func() {
				fmt.Fprintln(GinkgoWriter, ">inner before<")
			})

			It("should emit progress as it goes", func() {
				fmt.Fprintln(GinkgoWriter, ">it<")
			})
		})
	})

	Specify("should emit progress as it goes", func() {
		fmt.Fprintln(GinkgoWriter, ">specify<")
	})
})
