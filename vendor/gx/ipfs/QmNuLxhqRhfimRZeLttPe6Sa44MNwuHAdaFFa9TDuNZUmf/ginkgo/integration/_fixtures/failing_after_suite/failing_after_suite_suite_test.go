package failing_before_suite_test

import (
	. "github.com/onsi/gomega"
	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"

	"testing"
)

func TestFailingAfterSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FailingAfterSuite Suite")
}

var _ = BeforeSuite(func() {
	println("BEFORE SUITE")
})

var _ = AfterSuite(func() {
	println("AFTER SUITE")
	panic("BAM!")
})
