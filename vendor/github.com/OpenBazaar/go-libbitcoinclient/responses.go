package libbitcoin

import (
	"github.com/btcsuite/btcutil"
)

type FetchHistory2Resp struct {
	IsSpend  bool
	TxHash   string
	Index    uint32
	Height   uint32
	Value    uint64
}

type SubscribeResp struct{
	Address   string
	Height    uint32
	Block     string
	Tx        btcutil.Tx
}

