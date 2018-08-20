package params

import (
	"github.com/btcsuite/btcd/chaincfg"
	l "github.com/ltcsuite/ltcd/chaincfg"
)

func init() {
	l.MainNetParams.ScriptHashAddrID = 0x05
}

func ConvertParams(params *chaincfg.Params) l.Params {
	switch params.Name {
	case chaincfg.MainNetParams.Name:
		return l.MainNetParams
	case chaincfg.TestNet3Params.Name:
		return l.TestNet4Params
	default:
		return l.RegressionNetParams
	}
}
