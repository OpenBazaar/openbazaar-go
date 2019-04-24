package basemux

import (
	mc "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec"
	mux "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec/mux"

	b64 "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec/base/b64"
	bin "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec/base/bin"
	hex "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec/base/hex"
)

func AllBasesMux() *mux.Multicodec {
	m := mux.MuxMulticodec([]mc.Multicodec{
		hex.Multicodec(),
		b64.Multicodec(),
		bin.Multicodec(),
	}, mux.SelectFirst)
	m.Wrap = false
	return m
}
