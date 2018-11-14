package gexec_test

import (
	"bytes"

	. "gx/ipfs/QmUWtNQd8JdEiYiDqNYTUcaqyteJZ2rTNQLiw3dauLPccy/gomega/gexec"

	. "github.com/onsi/ginkgo"
	. "gx/ipfs/QmUWtNQd8JdEiYiDqNYTUcaqyteJZ2rTNQLiw3dauLPccy/gomega"
)

var _ = Describe("PrefixedWriter", func() {
	var buffer *bytes.Buffer
	var writer *PrefixedWriter
	BeforeEach(func() {
		buffer = &bytes.Buffer{}
		writer = NewPrefixedWriter("[p]", buffer)
	})

	It("should emit the prefix on newlines", func() {
		writer.Write([]byte("abc"))
		writer.Write([]byte("def\n"))
		writer.Write([]byte("hij\n"))
		writer.Write([]byte("\n\n"))
		writer.Write([]byte("klm\n\nnop"))
		writer.Write([]byte(""))
		writer.Write([]byte("qrs"))
		writer.Write([]byte("\ntuv\nwx"))
		writer.Write([]byte("yz\n\n"))

		Expect(buffer.String()).Should(Equal(`[p]abcdef
[p]hij
[p]
[p]
[p]klm
[p]
[p]nopqrs
[p]tuv
[p]wxyz
[p]
`))
	})
})
