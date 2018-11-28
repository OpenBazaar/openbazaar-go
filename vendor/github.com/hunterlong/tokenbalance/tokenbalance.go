package tokenbalance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	logFmt "log"
	"math/big"
)

var (
	Geth    *ethclient.Client
	config  *Config
	VERSION string
)

func New(contract, wallet string) (*TokenBalance, error) {
	var err error
	if config == nil || Geth == nil {
		return nil, errors.New("geth server connection has not been created")
	}
	tb := &TokenBalance{
		Contract: common.HexToAddress(contract),
		Wallet:   common.HexToAddress(wallet),
		Decimals: 0,
		Balance:  big.NewInt(0),
		ctx:      context.TODO(),
	}
	err = tb.query()
	return tb, err
}

func log(message string, error bool) {
	if config.Logs {
		if error {
			logFmt.Fatal(message)
			return
		}
		logFmt.Print(message)
	}
}

func (c *Config) Connect() error {
	var err error
	if c.GethLocation == "" {
		return errors.New("geth endpoint has not been set")
	}
	ethConn, err := ethclient.Dial(c.GethLocation)
	if err != nil {
		return err
	}
	block, err := ethConn.BlockByNumber(context.TODO(), nil)
	if block == nil {
		return err
	}
	config = c
	Geth = ethConn
	log(fmt.Sprintf("Connected to Geth at: %v\n", c.GethLocation), false)
	return err
}

func (tb *TokenBalance) ETHString() string {
	return bigIntString(tb.ETH, 18)
}

func (tb *TokenBalance) BalanceString() string {
	if tb.Decimals == 0 {
		return tb.Balance.String()
	}
	return bigIntString(tb.Balance, tb.Decimals)
}

func (tb *TokenBalance) query() error {
	var err error

	token, err := newTokenCaller(tb.Contract, Geth)
	if err != nil {
		log(fmt.Sprintf("Failed to instantiate a token contract: %v\n", err), false)
		return err
	}

	block, err := Geth.BlockByNumber(tb.ctx, nil)
	if err != nil {
		log(fmt.Sprintf("Failed to get current block number: %v\n", err), false)
	}
	tb.Block = block.Number().Int64()

	decimals, err := token.Decimals(nil)
	if err != nil {
		log(fmt.Sprintf("Failed to get decimals from contract: %v \n", tb.Contract.String()), false)
		return err
	}
	tb.Decimals = decimals.Int64()

	tb.ETH, err = Geth.BalanceAt(tb.ctx, tb.Wallet, nil)
	if err != nil {
		log(fmt.Sprintf("Failed to get ethereum balance from address: %v \n", tb.Wallet.String()), false)
	}

	tb.Balance, err = token.BalanceOf(nil, tb.Wallet)
	if err != nil {
		log(fmt.Sprintf("Failed to get balance from contract: %v %v\n", tb.Contract.String(), err), false)
		tb.Balance = big.NewInt(0)
	}

	tb.Symbol, err = token.Symbol(nil)
	if err != nil {
		log(fmt.Sprintf("Failed to get symbol from contract: %v \n", tb.Contract.String()), false)
		tb.Symbol = symbolFix(tb.Contract.String())
	}

	tb.Name, err = token.Name(nil)
	if err != nil {
		log(fmt.Sprintf("Failed to retrieve token name from contract: %v | %v\n", tb.Contract.String(), err), false)
		tb.Name = "MISSING"
	}

	return err
}

func symbolFix(contract string) string {
	switch common.HexToAddress(contract).String() {
	case "0x86Fa049857E0209aa7D9e616F7eb3b3B78ECfdb0":
		return "EOS"
	}
	return "MISSING"
}

func (tb *TokenBalance) ToJSON() string {
	jsonData := tokenBalanceJson{
		Contract: tb.Contract.String(),
		Wallet:   tb.Wallet.String(),
		Name:     tb.Name,
		Symbol:   tb.Symbol,
		Balance:  tb.BalanceString(),
		ETH:      tb.ETHString(),
		Decimals: tb.Decimals,
		Block:    tb.Block,
	}
	d, _ := json.Marshal(jsonData)
	return string(d)
}

func bigIntString(balance *big.Int, decimals int64) string {
	amount := bigIntFloat(balance, decimals)
	deci := fmt.Sprintf("%%0.%vf", decimals)
	return clean(fmt.Sprintf(deci, amount))
}

func bigIntFloat(balance *big.Int, decimals int64) *big.Float {
	if balance.Sign() == 0 {
		return big.NewFloat(0)
	}
	bal := big.NewFloat(0)
	bal.SetInt(balance)
	pow := bigPow(10, decimals)
	p := big.NewFloat(0)
	p.SetInt(pow)
	bal.Quo(bal, p)
	return bal
}

func bigPow(a, b int64) *big.Int {
	r := big.NewInt(a)
	return r.Exp(r, big.NewInt(b), nil)
}

func clean(newNum string) string {
	stringBytes := bytes.TrimRight([]byte(newNum), "0")
	newNum = string(stringBytes)
	if stringBytes[len(stringBytes)-1] == 46 {
		newNum += "0"
	}
	if stringBytes[0] == 46 {
		newNum = "0" + newNum
	}
	return newNum
}
