![TokenBalance](http://i.imgur.com/43Blvht.jpg)


# TokenBalance API [![Build Status](https://travis-ci.org/hunterlong/tokenbalance.svg?branch=master)](https://travis-ci.org/hunterlong/tokenbalance) [![Docker Build Status](https://img.shields.io/docker/build/hunterlong/tokenbalance.svg)](https://hub.docker.com/r/hunterlong/tokenbalance/) [![Coverage Status](https://coveralls.io/repos/github/hunterlong/tokenbalance/badge.svg?branch=master)](https://coveralls.io/github/hunterlong/tokenbalance?branch=master) [![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](https://godoc.org/github.com/hunterlong/tokenbalance)
TokenBalance is an easy to use public API and application that will output your [ERC20 Token](https://github.com/ConsenSys/Tokens/blob/master/Token_Contracts/contracts/Token.sol) balance without any troubles. You can run TokenBalance on your local computer or you can use api.tokenbalance.com to easily parse your erc20 token balances.
Connects to your local geth IPC and prints out a simple JSON response for ethereum token balances. Runs on port *8080* by default if you wish to run locally.

<p align="center">
<a href="https://github.com/hunterlong/balancebadge"><img src="https://img.balancebadge.io/eth/0x004F3E7fFA2F06EA78e14ED2B13E87d710e8013F.svg?color=green"></a> <a href="https://github.com/hunterlong/balancebadge"><img src="https://img.balancebadge.io/token/0x86fa049857e0209aa7d9e616f7eb3b3b78ecfdb0/0x3bf19a5b8b8cacda07c0ad46c18b27d999c15d0f.svg?color=purple"></a> <a href="https://github.com/hunterlong/balancebadge"><img src="https://img.balancebadge.io/token/0xd26114cd6EE289AccF82350c8d8487fedB8A0C07/0x12a99eb147af05a68ff900d8e9f55d855a41dda7.svg?color=cyan"></a> <a href="https://github.com/hunterlong/balancebadge"><img src="https://img.balancebadge.io/token/0xe41d2489571d322189246dafa5ebde1f4699f498/0xb15bb8a5c133aeb874c2b61252b7f01128574332.svg?color=red"></a>
</p>

## Server Status and Uptime
You can view the current status of Token Balance along with API latency information on our status page. This status page logs the Ethereum Mainnet, Ropsten testnet, and Rinkeby testnet.

[https://status.tokenbalance.com](https://status.tokenbalance.com)

## Installing Token Balance
You don't need to compile Token Balance from source anymore! All you need to do is go to [Releases](https://github.com/hunterlong/tokenbalance/releases/latest) in this repo and download the binary that is built for your OS.
Once you've downloaded, rename the file to `tokenbalance` for ease if use. On Mac or Linux move this file with the command `mv tokenbalance /usr/local/bin/tokenbalance`, you should be able to run the application from anywhere now.

## Token Balance and Token Info (/token)
To fetch information about your balance, token details, and ETH balance use the follow API call in a simple HTTP GET or CURL. The response is in JSON so you can easily parse what you need. Replace TOKEN_ADDRESS with the contract address of the ERC20 token, and replace ETH_ADDRESS with your address.

###### Ethereum Mainnet
```bash
https://api.tokenbalance.com/token/TOKEN_ADDRESS/ETH_ADDRESS
```

###### Ethereum Ropsten Testnet
```bash
https://test.tokenbalance.com/token/TOKEN_ADDRESS/ETH_ADDRESS
```

###### Ethereum Rinkeby Testnet
```bash
https://rinkeby.tokenbalance.com/token/TOKEN_ADDRESS/ETH_ADDRESS
```

- ###### Response (JSON)
```bash
{
    "name": "Kin",
    "wallet": "0x393c82c7Ae55B48775f4eCcd2523450d291f2418",
    "symbol": "KIN",
    "decimals": 18,
    "balance": "15788648",
    "eth_balance": "0.217960852347180212",
    "block": 4274167
}
```

## Only Token Balance (/balance)
This API response will only show you the ERC20 token balance in plain text. Perfect for ultra simple parsing.

```bash
https://api.tokenbalance.com/balance/TOKEN_ADDRESS/ETH_ADDRESS
```
- ###### Response (PLAIN TEXT)
```bash
1022.503000
```

## Examples

- [jsFiddle AJAX Example](https://jsfiddle.net/hunterlong/nkqr6064/)
- [Fetch Balance and Token Details for Status Coin](https://api.tokenbalance.com/token/0x744d70FDBE2Ba4CF95131626614a1763DF805B9E/0x242f3f8cffecc870bdb30165a0cb3c1f06f32949)
- [Fetch Balance and Token Details for Gnosis](https://api.tokenbalance.com/token/0x6810e776880c02933d47db1b9fc05908e5386b96/0x97b47ffde901107303a53630d28105c6a7af1c3e)
- [Fetch Balance and Token Details for Storj](https://api.tokenbalance.com/token/0xb64ef51c888972c908cfacf59b47c1afbc0ab8ac/0x29b532092fd5031b9ee1e5fe07d627abedd5eda8)
- [Only Token Balance for Augur](https://api.tokenbalance.com/balance/0x48c80F1f4D53D5951e5D5438B54Cba84f29F32a5/0x90fbfc09db2f4b6e8b65b7a237e15bba9dc5db0c)
- [Only Token Balance for Golem](https://api.tokenbalance.com/balance/0xa74476443119A942dE498590Fe1f2454d7D4aC0d/0xe42b94dc4b02edef833556ede32757cf2b6cc455)

# Run with Docker
You can easily start [Token Balance with Docker](https://hub.docker.com/r/hunterlong/tokenbalance/builds/). Register for a free [Infura.io API Key](https://infura.io/signup) to use Token Balance without downloading the ethereum blockchain.
```
docker run -p 8080:8080 -e GETH_SERVER=https://mainnet.infura.io/APIKEY -d hunterlong/tokenbalance
```

# Use as Golang Package
You can use Token Balance as a typical Go Language package if you you like to implement ERC20 functionality into your own application.

```bash
go get github.com/hunterlong/tokenbalance
```

###### First you'll want to connect to your Geth server or IPC
```go
import (
    "github.com/hunterlong/tokenbalance"
)

func main() {
	// connect to your Geth Server
    configs = &tokenbalance.Config{
         GethLocation: "https://eth.coinapp.io",
         Logs:         true,
    }
    configs.Connect()

    // insert a Token Contract address and Wallet address
    contract := "0x86fa049857e0209aa7d9e616f7eb3b3b78ecfdb0"
    wallet := "0xbfaa1a1ea534d35199e84859975648b59880f639"

    // query the blockchain and wallet details
    token, err := tokenbalance.New(contract, wallet)

    // Token Balance will respond back useful things
    token.BalanceString()  // "600000.0"
    token.ETHString()      // "1.020095885777777767"
    token.Name             // "OmiseGO"
    token.Symbol           // "OMG"
    token.Decimals         // 18
    token.Balance          // big.Int() (token balance)
    token.ETH              // big.Int() (ether balance)
}
```

# Implement in Google Sheets
If your familiar with Google Sheets, you can easily fetch all of your cryptocurrency balances within 1 spreadsheet. All you need to do is make a cell with the value below.
```
=ImportData("https://api.tokenbalance.com/balance/0xd26114cd6EE289AccF82350c8d8487fedB8A0C07/0xf9578adc61d07671f536d50afc5800232fc9fd86")
```
Simple as that! Get creative an use Coin Market Cap's API to fetch the price and multiply with your balance to make a portfolio of your cryptocurrencies!

# Implement in your App
Feel free to use the TokenBalance API server to fetch ERC20 token balances and details. We do have a header set that will allow you to call the API via AJAX. `Access-Control-Allow-Origin "*"` The server may limit your requests if you do more than 60 hits per minute.

# Run Your Own Server
TokenBalance isn't just an API, it's an opensource HTTP server that you can run on your own computer or server.

<p align="center"><img width="85%" src="https://img.cjx.io/tokenbalanceunix.gif">
<img width="85%" src="https://img.cjx.io/tokenbalancewindows.gif">
</p>

## Installation
##### Ubuntu 16.04
```bash
git clone https://github.com/hunterlong/tokenbalance
cd tokenbalance
go get && go build ./cmd
```

## Start TokenBalance Server
```bash
tokenbalance start --geth="/ethereum/geth.ipc"
```
This will create a light weight HTTP server will respond balance information about a ethereum contract token.

## Optional Config
```bash
tokenbalance start --geth="/ethereum/geth.ipc" --port 8080 --ip 127.0.0.1
```

#### CURL Request
```bash
CONTRACT=0xa74476443119A942dE498590Fe1f2454d7D4aC0d
ETH_ADDRESS=0xda0aed568d9a2dbdcbafc1576fedc633d28eee9a

curl https://api.tokenbalance.com/token/$CONTRACT/$ETH_ADDRESS
```

#### Response
```bash
{
    "name": "Kin",
    "wallet": "0x393c82c7Ae55B48775f4eCcd2523450d291f2418",
    "symbol": "KIN",
    "decimals": 18,
    "balance": "15788648",
    "eth_balance": "0.217960852347180212",
    "block": 4274167
}
```
