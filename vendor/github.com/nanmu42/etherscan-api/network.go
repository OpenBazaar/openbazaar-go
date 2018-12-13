/*
 * Copyright (c) 2018 LI Zhennan
 *
 * Use of this work is governed by a MIT License.
 * You may find a license copy in project root.
 */

package etherscan

const (
	//// Ethereum public networks

	// Mainnet Ethereum mainnet for production
	Mainnet Network = "api"
	// Ropsten Testnet(POW)
	Ropsten Network = "api-ropsten"
	// Kovan Testnet(POA)
	Kovan Network = "api-kovan"
	// Rinkby Testnet(CLIQUE)
	Rinkby Network = "api-rinkeby"
	// Tobalaba Testnet
	Tobalaba Network = "api-tobalaba"
)

// Network is ethereum network type (mainnet, ropsten, etc)
type Network string

// SubDomain returns the subdomain of  etherscan API
// via n provided.
func (n Network) SubDomain() (sub string)  {
	return string(n)
}