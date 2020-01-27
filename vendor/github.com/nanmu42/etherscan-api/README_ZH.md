# etherscan-api

[![Build Status](https://travis-ci.org/nanmu42/etherscan-api.svg?branch=master)](https://travis-ci.org/nanmu42/etherscan-api)
[![Go Report Card](https://goreportcard.com/badge/github.com/nanmu42/etherscan-api)](https://goreportcard.com/report/github.com/nanmu42/etherscan-api)
[![codecov](https://codecov.io/gh/nanmu42/etherscan-api/branch/master/graph/badge.svg)](https://codecov.io/gh/nanmu42/etherscan-api)
[![GoDoc](https://godoc.org/github.com/nanmu42/etherscan-api?status.svg)](https://godoc.org/github.com/nanmu42/etherscan-api)
[English Readme](https://github.com/nanmu42/etherscan-api/blob/master/README.md)

Etherscan.io的Golang实现，
支持几乎所有功能（accounts, transactions, tokens, contracts, blocks, stats），
所有公共网络（Mainnet, Ropsten, Kovan, Rinkby, Tobalaba）。
本项目只依赖于官方库。 :wink:

# Usage

填入网络选项和API Key即可开始使用。 :rocket:

```go
import (
	"github.com/nanmu42/etherscan-api"
	"fmt"
)

func main() {
	// 创建连接指定网络的客户端
	client := etherscan.New(etherscan.Mainnet, "[your API key]")

	// （可选）按需注册钩子函数，例如用于速率控制
	client.BeforeRequest = func(module, action string, param map[string]interface{}) error {
		// ...
	}
	client.AfterRequest = func(module, action string, param map[string]interface{}, outcome interface{}, requestErr error) {
		// ...
	}

	// 查询账户以太坊余额
	balance, err := client.AccountBalance("0x281055afc982d96fab65b3a49cac8b878184cb16")
	if err != nil {
		panic(err)
	}
	// 余额以 *big.Int 的类型呈现，单位为 wei
	fmt.Println(balance.Int())

	// 查询token余额
	tokenBalance, err := client.TokenBalance("contractAddress", "holderAddress")

	// 查询出入指定地址的ERC20转账列表
	transfers, err := client.ERC20Transfers("contractAddress", "address", startBlock, endBlock, page, offset)
}
```

客户端方法列表可在[GoDoc](https://godoc.org/github.com/nanmu42/etherscan-api)查询。

# Etherscan API Key

API Key可以在[etherscan](https://etherscan.io/apis)申请。

Etherscan的API服务是一个公开的社区无偿服务，请避免滥用。
API的调用速率不能高于5次/秒，否则会遭到封禁。

# 利益声明

我和Etherscan没有任何联系。我仅仅是觉得他们的服务很棒，而自己又恰好需要这样一个库。 :smile:

# 许可证

MIT

请自由享受开源，欢迎贡献开源。