package muxcodec

import (
	mc "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec"
	cbor "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec/cbor"
	json "gx/ipfs/QmZb2Hc6sQeFpmnSuFLYH2eWjaMcPPtzDzXY1PkMM1sjnP/go-multicodec/json"
)

func StandardMux() *Multicodec {
	return MuxMulticodec([]mc.Multicodec{
		cbor.Multicodec(),
		json.Multicodec(false),
		json.Multicodec(true),
	}, SelectFirst)
}
