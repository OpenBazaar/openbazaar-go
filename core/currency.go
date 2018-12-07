package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	wallet "github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcutil"
)

// DefaultCurrencyDivisibility is the Divisibility of the Currency if not
// defined otherwise
const DefaultCurrencyDivisibility uint32 = 1e8

type SpendRequest struct {
	decodedAddress btcutil.Address

	Address                string `json:"address"`
	Amount                 int64  `json:"amount"`
	FeeLevel               string `json:"feeLevel"`
	Memo                   string `json:"memo"`
	OrderID                string `json:"orderId"`
	RequireAssociatedOrder bool   `json:"requireOrder"`
	Wallet                 string `json:"wallet"`
}

type SpendResponse struct {
	Amount             int64     `json:"amount"`
	ConfirmedBalance   int64     `json:"confirmedBalance"`
	Memo               string    `json:"memo"`
	OrderID            string    `json:"orderId"`
	Timestamp          time.Time `json:"timestamp"`
	Txid               string    `json:"txid"`
	UnconfirmedBalance int64     `json:"unconfirmedBalance"`
}

// Spend will attempt to move funds from the node to the destination address described in the
// SpendRequest for the amount indicated.
func (n *OpenBazaarNode) Spend(args *SpendRequest) (*SpendResponse, error) {
	var feeLevel wallet.FeeLevel

	wal, err := n.Multiwallet.WalletForCurrencyCode(args.Wallet)
	if err != nil {
		return nil, ErrUnknownWallet
	}

	addr, err := wal.DecodeAddress(args.Address)
	if err != nil {
		return nil, ErrInvalidSpendAddress
	}
	args.decodedAddress = addr

	contract, err := n.getOrderContractBySpendRequest(args)
	if err != nil && args.RequireAssociatedOrder {
		return nil, ErrOrderNotFound
	}

	switch strings.ToUpper(args.FeeLevel) {
	case "PRIORITY":
		feeLevel = wallet.PRIOIRTY
	case "NORMAL":
		feeLevel = wallet.NORMAL
	case "ECONOMIC":
		feeLevel = wallet.ECONOMIC
	default:
		feeLevel = wallet.NORMAL
	}

	txid, err := wal.Spend(args.Amount, addr, feeLevel, args.OrderID)
	if err != nil {
		switch {
		case err == wallet.ErrorInsuffientFunds:
			return nil, ErrInsufficientFunds
		case err == wallet.ErrorDustAmount:
			return nil, ErrSpendAmountIsDust
		default:
			return nil, err
		}
	}

	var (
		thumbnail string
		title     string
		memo      = args.Memo
	)
	if contract != nil {
		if contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
			thumbnail = contract.VendorListings[0].Item.Images[0].Tiny
			title = contract.VendorListings[0].Item.Title
		}
	}
	if memo == "" && title != "" {
		memo = title
	}

	if err := n.Datastore.TxMetadata().Put(repo.Metadata{
		Txid:       txid.String(),
		Address:    args.Address,
		Memo:       memo,
		OrderId:    args.OrderID,
		Thumbnail:  thumbnail,
		CanBumpFee: false,
	}); err != nil {
		return nil, fmt.Errorf("failed persisting transaction metadata: %s", err)
	}

	confirmed, unconfirmed := wal.Balance()
	txn, err := wal.GetTransaction(*txid)
	if err != nil {
		return nil, fmt.Errorf("failed retrieving new wallet balance: %s", err)
	}

	return &SpendResponse{
		Txid:               txid.String(),
		ConfirmedBalance:   confirmed,
		UnconfirmedBalance: unconfirmed,
		Amount:             -(txn.Value),
		Timestamp:          txn.Timestamp,
		Memo:               memo,
		OrderID:            args.OrderID,
	}, nil
}

func (n *OpenBazaarNode) getOrderContractBySpendRequest(args *SpendRequest) (*pb.RicardianContract, error) {
	var errorStr = "unable to find order from order id or spend address"
	if args.OrderID != "" {
		contract, _, _, _, _, err := n.Datastore.Purchases().GetByOrderId(args.OrderID)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", errorStr, err)
		}
		if contract != nil {
			return contract, nil
		}
	}

	if args.decodedAddress != nil {
		contract, _, _, _, err := n.Datastore.Purchases().GetByPaymentAddress(args.decodedAddress)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", errorStr, err)
		}
		if contract != nil {
			return contract, nil
		}
	}

	return nil, errors.New(errorStr)
}
