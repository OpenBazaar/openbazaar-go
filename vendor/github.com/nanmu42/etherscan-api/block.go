/*
 * Copyright (c) 2018 LI Zhennan
 *
 * Use of this work is governed by a MIT License.
 * You may find a license copy in project root.
 */

package etherscan

// BlockReward gets block and uncle rewards by block number
func (c *Client) BlockReward(blockNum int) (rewards BlockRewards, err error) {
	param := M{
		"blockno": blockNum,
	}

	err = c.call("block", "getblockreward", param, &rewards)
	return
}
