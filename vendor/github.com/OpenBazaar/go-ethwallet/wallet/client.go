package wallet

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/OpenBazaar/go-ethwallet/util"
)

// EthClient represents the eth client
type EthClient struct {
	*ethclient.Client
}

var txns []wi.Txn

// NewEthClient returns a new eth client
func NewEthClient(url string) (*EthClient, error) {
	var conn *ethclient.Client
	var err error
	if conn, err = ethclient.Dial(url); err != nil {
		return nil, err
	}
	return &EthClient{
		Client: conn,
	}, nil

}

// Transfer will transfer eth from this user account to dest address
func (client *EthClient) Transfer(from *Account, destAccount common.Address, value *big.Int) (common.Hash, error) {
	var err error
	fromAddress := from.Address()
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return common.BytesToHash([]byte{}), err
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return common.BytesToHash([]byte{}), err
	}

	msg := ethereum.CallMsg{From: fromAddress, Value: value}
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		return common.BytesToHash([]byte{}), err
	}

	rawTx := types.NewTransaction(nonce, destAccount, value, gasLimit, gasPrice, nil)
	signedTx, err := from.SignTransaction(types.HomesteadSigner{}, rawTx)
	if err != nil {
		return common.BytesToHash([]byte{}), err
	}
	txns = append(txns, wi.Txn{
		Txid:      signedTx.Hash().Hex(),
		Value:     value.Int64(),
		Height:    0,
		Timestamp: time.Now(),
		WatchOnly: false,
		Bytes:     rawTx.Data()})

	// this for debug only
	fmt.Println("Txn ID : ", signedTx.Hash().Hex())

	return signedTx.Hash(), client.SendTransaction(context.Background(), signedTx)
}

// GetBalance - returns the balance for this account
func (client *EthClient) GetBalance(destAccount common.Address) (*big.Int, error) {
	return client.BalanceAt(context.Background(), destAccount, nil)
}

// GetUnconfirmedBalance - returns the unconfirmed balance for this account
func (client *EthClient) GetUnconfirmedBalance(destAccount common.Address) (*big.Int, error) {
	return client.PendingBalanceAt(context.Background(), destAccount)
}

// GetTransaction - returns a eth txn for the specified hash
func (client *EthClient) GetTransaction(hash common.Hash) (*types.Transaction, bool, error) {
	return client.TransactionByHash(context.Background(), hash)
}

// GetLatestBlock - returns the latest block
func (client *EthClient) GetLatestBlock() (uint32, string, error) {
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, "", err
	}
	return uint32(header.Number.Int64()), header.Hash().String(), nil
}

// EstimateTxnGas - returns estimated gas
func (client *EthClient) EstimateTxnGas(from, to common.Address, value *big.Int) (*big.Int, error) {
	gas := big.NewInt(0)
	if !(util.IsValidAddress(from.String()) && util.IsValidAddress(to.String())) {
		return gas, errors.New("invalid address")
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return gas, err
	}
	msg := ethereum.CallMsg{From: from, To: &to, Value: value}
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		return gas, err
	}
	return gas.Mul(big.NewInt(int64(gasLimit)), gasPrice), nil
}

// EstimateGasSpend - returns estimated gas
func (client *EthClient) EstimateGasSpend(from common.Address, value *big.Int) (*big.Int, error) {
	gas := big.NewInt(0)
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return gas, err
	}
	msg := ethereum.CallMsg{From: from, Value: value}
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		return gas, err
	}
	return gas.Mul(big.NewInt(int64(gasLimit)), gasPrice), nil
}

/*
func getClient() (*ethclient.Client, error) {
	client, err := ethclient.Dial("https://mainnet.infura.io")
	if err != nil {
		log.Info("error initializing client")
	}

	return client, err
}
*/

func init() {
	txns = []wi.Txn{}
}
