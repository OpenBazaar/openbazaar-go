/*
 * Copyright (c) 2018 LI Zhennan
 *
 * Use of this work is governed by a MIT License.
 * You may find a license copy in project root.
 */

package etherscan

import "errors"

// ErrPreByzantiumTx transaction before 4,370,000 does not support receipt status check
var ErrPreByzantiumTx = errors.New("pre-byzantium transaction does not support receipt status check")

// ExecutionStatus checks contract execution status (if there was an error during contract execution)
//
// note on IsError: 0 = pass, 1 = error
func (c *Client) ExecutionStatus(txHash string) (status ExecutionStatus, err error) {
	param := M{
		"txhash": txHash,
	}

	err = c.call("transaction", "getstatus", param, &status)
	return
}

// ReceiptStatus checks transaction receipt status
//
// only applicable for post byzantium fork transactions, i.e. after block 4,370,000
//
// An special err ErrPreByzantiumTx raises for the transaction before byzantium fork.
//
// Note: receiptStatus: 0 = Fail, 1 = Pass.
func (c *Client) ReceiptStatus(txHash string) (receiptStatus int, err error) {
	param := M{
		"txhash": txHash,
	}

	var rawStatus = struct {
		Status string `json:"status"`
	}{}

	err = c.call("transaction", "gettxreceiptstatus", param, &rawStatus)
	if err != nil {
		return
	}

	switch rawStatus.Status {
	case "0":
		receiptStatus = 0
	case "1":
		receiptStatus = 1
	default:
		receiptStatus = -1
		err = ErrPreByzantiumTx
	}

	return
}
