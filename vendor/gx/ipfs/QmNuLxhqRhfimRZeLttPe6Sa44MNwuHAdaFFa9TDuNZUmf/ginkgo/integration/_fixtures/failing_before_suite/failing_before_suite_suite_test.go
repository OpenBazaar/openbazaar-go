package failing_before_suite_test

import (
	. "github.com/onsi/gomega"
	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"

	"testing"
)

func TestFailing_before_suite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Failing_before_suite Suite")
}

var _ = BeforeSuite(func() {
	println("BEFORE SUITE")
	panic("BAM!")
})

var _ = AfterSuite(func() {
	println("AFTER SUITE")
})
