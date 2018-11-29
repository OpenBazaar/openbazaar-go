package model

type Status struct {
	Info Info `json:"info"`
}

type Info struct {
	Version         int         `json:"version"`
	ProtocolVersion int         `json:"protocolversion"`
	Blocks          int         `json:"blocks"`
	TimeOffset      int         `json:"timeoffset"`
	Connections     int         `json:"connections"`
	DifficultyIface interface{} `json:"difficulty"`
	Difficulty      float64     `json:"-"`
	Testnet         bool        `json:"testnet"`
	RelayFeeIface   interface{} `json:"relayfee"`
	RelayFee        float64     `json:"-"`
	Errors          string      `json:"errors"`
	Network         string      `json:"network"`
}

func (i Info) IsEqual(other Info) bool {
	if i.Version != other.Version {
		return false
	}
	if i.ProtocolVersion != other.ProtocolVersion {
		return false
	}
	if i.Blocks != other.Blocks {
		return false
	}
	if i.TimeOffset != other.TimeOffset {
		return false
	}
	if i.Connections != other.Connections {
		return false
	}
	if i.Difficulty != other.Difficulty {
		return false
	}
	if i.Testnet != other.Testnet {
		return false
	}
	if i.RelayFee != other.RelayFee {
		return false
	}
	if i.Errors != other.Errors {
		return false
	}
	if i.Network != other.Network {
		return false
	}
	return true
}

type BlockList struct {
	Blocks     []Block    `json:"blocks"`
	Length     int        `json:"length"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Next      string `json:"next"`
	Prev      string `json:"prev"`
	CurrentTs int    `json:"currentTs"`
	Current   string `json:"current"`
	IsToday   bool   `json:"isToday"`
	More      bool   `json:"more"`
	MoreTs    int    `json:"moreTs"`
}

type Block struct {
	Hash              string    `json:"hash"`
	Size              int       `json:"size"`
	Height            int       `json:"height"`
	Version           int       `json:"version"`
	MerkleRoot        string    `json:"merkleroot"`
	Tx                []string  `json:"tx"`
	Time              int64     `json:"time"`
	Nonce             string    `json:"nonce"`
	Solution          string    `json:"solution"`
	Bits              string    `json:"bits"`
	Difficulty        float64   `json:"difficulty"`
	Chainwork         string    `json:"chainwork"`
	Confirmations     int       `json:"confirmations"`
	PreviousBlockhash string    `json:"previousblockhash"`
	NextBlockhash     string    `json:"nextblockhash"`
	Reward            float64   `json:"reward"`
	IsMainChain       bool      `json:"isMainChain"`
	PoolInfo          *PoolInfo `json:"poolinfo"`
}

type PoolInfo struct {
	PoolName string `json:"poolName"`
	URL      string `json:"url"`
}

type Utxo struct {
	Address       string      `json:"address"`
	Txid          string      `json:"txid"`
	Vout          int         `json:"vout"`
	ScriptPubKey  string      `json:"scriptPubKey"`
	AmountIface   interface{} `json:"amount"`
	Amount        float64
	Satoshis      int64 `json:"satoshis"`
	Confirmations int   `json:"confirmations"`
}

type TransactionList struct {
	TotalItems int           `json:"totalItems"`
	From       int           `json:"from"`
	To         int           `json:"to"`
	Items      []Transaction `json:"items"`
}

type Transaction struct {
	Txid          string   `json:"txid"`
	Version       int      `json:"version"`
	Locktime      int      `json:"locktime"`
	Inputs        []Input  `json:"vin"`
	Outputs       []Output `json:"vout"`
	BlockHash     string   `json:"blockhash"`
	BlockHeight   int      `json:"blockheight"`
	Confirmations int      `json:"confirmations"`
	Time          int64    `json:"time"`
	BlockTime     int64    `json:"blocktime"`
	RawBytes      []byte   `json:"rawbytes"`
}

type RawTxResponse struct {
	RawTx string `json:"rawtx"`
}

type Input struct {
	Txid            string      `json:"txid"`
	Vout            int         `json:"vout"`
	Sequence        uint32      `json:"sequence"`
	N               int         `json:"n"`
	ScriptSig       Script      `json:"scriptSig"`
	Addr            string      `json:"addr"`
	Satoshis        int64       `json:"valueSat"`
	ValueIface      interface{} `json:"value"`
	Value           float64
	DoubleSpentTxid string `json:"doubleSpentTxID"`
}

type Output struct {
	ValueIface   interface{} `json:"value"`
	Value        float64
	N            int       `json:"n"`
	ScriptPubKey OutScript `json:"scriptPubKey"`
	SpentTxid    string    `json:"spentTxId"`
	SpentIndex   int       `json:"spentIndex"`
	SpentHeight  int       `json:"spentHeight"`
}

type Script struct {
	Hex string `json:"hex"`
	Asm string `json:"asm"`
}

type OutScript struct {
	Script
	Addresses []string `json:"addresses"`
	Type      string   `json:"type"`
}

type AddressTxid struct {
	Address string `json:"address"`
	Txid    string `json:"txid"`
}
