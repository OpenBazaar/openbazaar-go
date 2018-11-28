// Package tokenbalance is used to fetch the latest token balance for any Ethereum address and ERC20 token. You can install
// this package/CLI or you can use basic HTTP GET on the public TokenBalance server.
//
// Mainnet API Endpoint:
// https://api.tokenbalance.com
//
// Example: https://api.tokenbalance.com/balance/0xa74476443119A942dE498590Fe1f2454d7D4aC0d/0xda0aed568d9a2dbdcbafc1576fedc633d28eee9a
// Response: `5401731.086778292432427406`
//
// Example: https://api.tokenbalance.com/token/0xa74476443119A942dE498590Fe1f2454d7D4aC0d/0xda0aed568d9a2dbdcbafc1576fedc633d28eee9a
// Response:
// ```
// {
// "token": "0xa74476443119A942dE498590Fe1f2454d7D4aC0d",
// "wallet": "0xda0AEd568D9A2dbDcBAFC1576fedc633d28EEE9a",
// "name": "Golem Network token",
// "symbol": "GNT",
// "balance": "5401731.086778292432427406",
// "eth_balance": "0.985735366999999973",
// "decimals": 18,
// "block": 6461672
// }
// ```
//
// Ropsten Testnet API Endpoint:
// https://test.tokenbalance.com
//
// Rinkeby Testnet API Endpoint:
// https://rinkeby.tokenbalance.com
//
// More info on: https://github.com/hunterlong/tokenbalance
package tokenbalance
