package stream_test

import (
	. "gx/ipfs/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
	. "gx/ipfs/QmUWtNQd8JdEiYiDqNYTUcaqyteJZ2rTNQLiw3dauLPccy/gomega"

	"testing"
)

func TestGoLibp2pStreamTransport(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GoLibp2pStreamTransport Suite")
}
