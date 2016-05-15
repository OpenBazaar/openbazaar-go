package libbitcoin

import (
	"errors"
	"encoding/binary"
)

var ErrorCodes map[int]error = make(map[int]error)

func init() {
	ErrorCodes[1] = errors.New("service stopped")
	ErrorCodes[2] = errors.New("operationg failed")
	ErrorCodes[3] = errors.New("not found")
	ErrorCodes[4] = errors.New("duplicate")
	ErrorCodes[5] = errors.New("unspent output")
	ErrorCodes[6] = errors.New("unsupported payment type")
	ErrorCodes[7] = errors.New("resolve failed")
	ErrorCodes[8] = errors.New("network unreachable")
	ErrorCodes[9] = errors.New("address in use")
	ErrorCodes[10] = errors.New("listen failed")
	ErrorCodes[11] = errors.New("accept failed")
	ErrorCodes[12] = errors.New("bad stream")
	ErrorCodes[13] = errors.New("channel timeout")
	ErrorCodes[14] = errors.New("blockchain reorganized")
	ErrorCodes[15] = errors.New("pool filled")
	ErrorCodes[16] = errors.New("coinbase transaction")
	ErrorCodes[17] = errors.New("not standard")
	ErrorCodes[18] = errors.New("double spend")
	ErrorCodes[19] = errors.New("input not found")
	ErrorCodes[20] = errors.New("empty transaction")
	ErrorCodes[21] = errors.New("output value overflow")
	ErrorCodes[22] = errors.New("invalid coinbase script size")
	ErrorCodes[23] = errors.New("previous output null")
	ErrorCodes[24] = errors.New("previous block invalid")
	ErrorCodes[25] = errors.New("size limits")
	ErrorCodes[26] = errors.New("proof of work")
	ErrorCodes[27] = errors.New("futuristic timestamp")
	ErrorCodes[28] = errors.New("first not coinbase")
	ErrorCodes[29] = errors.New("extra coinbases")
	ErrorCodes[30] = errors.New("too many sigs")
	ErrorCodes[31] = errors.New("merkle mismatch")
	ErrorCodes[32] = errors.New("incorrect proof of work")
	ErrorCodes[33] = errors.New("timestamp too early")
	ErrorCodes[34] = errors.New("non final transaction")
	ErrorCodes[35] = errors.New("checkpoint failed")
	ErrorCodes[36] = errors.New("old block version")
	ErrorCodes[37] = errors.New("coinbase height mismatch")
	ErrorCodes[38] = errors.New("duplicate or spent")
	ErrorCodes[39] = errors.New("validate inputs failed")
	ErrorCodes[40] = errors.New("fees out of range")
	ErrorCodes[41] = errors.New("coinbase too large")
}

func ParseError(b []byte) error {
	i := int(binary.LittleEndian.Uint32(b))
	if i == 0 {
		return nil
	}
	return ErrorCodes[i]
}
