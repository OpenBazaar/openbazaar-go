package discovery

import (
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

func init() {
	cbor.RegisterCborType(retrievalmarket.RetrievalPeer{})
}

func Multi(r retrievalmarket.PeerResolver) retrievalmarket.PeerResolver { // TODO: actually support multiple mechanisms
	return r
}
