/*
 * Copyright (c) 2018 LI Zhennan
 *
 * Use of this work is governed by a MIT License.
 * You may find a license copy in project root.
 */

package etherscan

// ContractABI gets contract abi for verified contract source codes
func (c *Client) ContractABI(address string) (abi string, err error) {
	param := M{
		"address": address,
	}

	err = c.call("contract", "getabi", param, &abi)
	return
}

// ContractSource gets contract source code for verified contract source codes
func (c *Client) ContractSource(address string) (source []ContractSource, err error) {
	param := M{
		"address": address,
	}

	err = c.call("contract", "getsourcecode", param, &source)
	return
}
