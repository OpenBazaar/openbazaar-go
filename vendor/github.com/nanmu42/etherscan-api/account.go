/*
 * Copyright (c) 2018 LI Zhennan
 *
 * Use of this work is governed by a MIT License.
 * You may find a license copy in project root.
 */

package etherscan

// AccountBalance gets ether balance for a single address
func (c *Client) AccountBalance(address string) (balance *BigInt, err error) {
	param := M{
		"tag":     "latest",
		"address": address,
	}
	balance = new(BigInt)
	err = c.call("account", "balance", param, balance)
	return
}

// MultiAccountBalance gets ether balance for multiple addresses in a single call
func (c *Client) MultiAccountBalance(addresses ...string) (balances []AccountBalance, err error) {
	param := M{
		"tag":     "latest",
		"address": addresses,
	}
	balances = make([]AccountBalance, 0, len(addresses))
	err = c.call("account", "balancemulti", param, &balances)
	return
}

// NormalTxByAddress gets a list of "normal" transactions by address
//
// startBlock and endBlock can be nil
//
// if desc is true, result will be sorted in blockNum descendant order.
func (c *Client) NormalTxByAddress(address string, startBlock *int, endBlock *int, page int, offset int, desc bool) (txs []NormalTx, err error) {
	param := M{
		"address": address,
		"page":    page,
		"offset":  offset,
	}
	compose(param, "startblock", startBlock)
	compose(param, "endblock", endBlock)
	if desc {
		param["sort"] = "desc"
	} else {
		param["sort"] = "asc"
	}

	err = c.call("account", "txlist", param, &txs)
	return
}

// InternalTxByAddress gets a list of "internal" transactions by address
//
// startBlock and endBlock can be nil
//
// if desc is true, result will be sorted in descendant order.
func (c *Client) InternalTxByAddress(address string, startBlock *int, endBlock *int, page int, offset int, desc bool) (txs []InternalTx, err error) {
	param := M{
		"address": address,
		"page":    page,
		"offset":  offset,
	}
	compose(param, "startblock", startBlock)
	compose(param, "endblock", endBlock)
	if desc {
		param["sort"] = "desc"
	} else {
		param["sort"] = "asc"
	}

	err = c.call("account", "txlistinternal", param, &txs)
	return
}

// ERC20Transfers get a list of "erc20 - token transfer events" by
// contract address and/or from/to address.
//
// leave undesired condition to nil.
//
// Note on a Etherscan bug:
// Some ERC20 contract does not have valid decimals information in Etherscan.
// When that happens, TokenName, TokenSymbol are empty strings,
// and TokenDecimal is 0.
//
// More information can be found at:
// https://github.com/nanmu42/etherscan-api/issues/8
func (c *Client) ERC20Transfers(contractAddress, address *string, startBlock *int, endBlock *int, page int, offset int, desc bool) (txs []ERC20Transfer, err error) {
	param := M{
		"page":   page,
		"offset": offset,
	}
	compose(param, "contractaddress", contractAddress)
	compose(param, "address", address)
	compose(param, "startblock", startBlock)
	compose(param, "endblock", endBlock)

	if desc {
		param["sort"] = "desc"
	} else {
		param["sort"] = "asc"
	}

	err = c.call("account", "tokentx", param, &txs)
	return
}

// BlocksMinedByAddress gets list of blocks mined by address
func (c *Client) BlocksMinedByAddress(address string, page int, offset int) (mined []MinedBlock, err error) {
	param := M{
		"address":   address,
		"blocktype": "blocks",
		"page":      page,
		"offset":    offset,
	}

	err = c.call("account", "getminedblocks", param, &mined)
	return
}

// UnclesMinedByAddress gets list of uncles mined by address
func (c *Client) UnclesMinedByAddress(address string, page int, offset int) (mined []MinedBlock, err error) {
	param := M{
		"address":   address,
		"blocktype": "uncles",
		"page":      page,
		"offset":    offset,
	}

	err = c.call("account", "getminedblocks", param, &mined)
	return
}

// TokenBalance get erc20-token account balance of address for contractAddress
func (c *Client) TokenBalance(contractAddress, address string) (balance *BigInt, err error) {
	param := M{
		"contractaddress": contractAddress,
		"address":         address,
		"tag":             "latest",
	}

	err = c.call("account", "tokenbalance", param, &balance)
	return
}
