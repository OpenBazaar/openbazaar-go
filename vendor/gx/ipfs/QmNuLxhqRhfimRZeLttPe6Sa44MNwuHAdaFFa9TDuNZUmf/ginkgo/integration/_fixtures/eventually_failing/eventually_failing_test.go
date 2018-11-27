package eventually_failing_test

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
)

var _ = Describe("EventuallyFailing", func() {
	It("should fail on the third try", func() {
		time.Sleep(time.Second)
		files, err := ioutil.ReadDir(".")
		Ω(err).ShouldNot(HaveOccurred())

		numRuns := 1
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "counter") {
				numRuns++
			}
		}

		Ω(numRuns).Should(BeNumerically("<", 3))
		ioutil.WriteFile(fmt.Sprintf("./counter-%d", numRuns), []byte("foo"), 0777)
	})
})
