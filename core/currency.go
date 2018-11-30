package core

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	wallet "github.com/OpenBazaar/wallet-interface"
)

// DefaultCurrencyDivisibility is the Divisibility of the Currency if not
// defined otherwise
const DefaultCurrencyDivisibility uint32 = 1e8

type SpendRequest struct {
	Wallet                 string `json:"wallet"`
	Address                string `json:"address"`
	Amount                 int64  `json:"amount"`
	FeeLevel               string `json:"feeLevel"`
	Memo                   string `json:"memo"`
	OrderID                string `json:"orderID"`
	RequireAssociatedOrder bool   `json:"requireOrder"`
}

type SpendResponse struct {
	Txid               string    `json:"txid"`
	Amount             int64     `json:"amount"`
	ConfirmedBalance   int64     `json:"confirmedBalance"`
	UnconfirmedBalance int64     `json:"unconfirmedBalance"`
	Timestamp          time.Time `json:"timestamp"`
	Memo               string    `json:"memo"`
	OrderID            string    `json:"orderID"`
}

func (n *OpenBazaarNode) Spend(args *SpendRequest) (*SpendResponse, error) {
	var (
		feeLevel wallet.FeeLevel
		contract *pb.RicardianContract
	)

	wal, err := n.Multiwallet.WalletForCurrencyCode(args.Wallet)
	if err != nil {
		return nil, ErrUnknownWallet
	}

	addr, err := wal.DecodeAddress(args.Address)
	if err != nil {
		return nil, ErrInvalidSpendAddress
	}

	if args.RequireAssociatedOrder {
		var err error
		contract, err = n.getOrderContractBySpendRequest(args)
		if err != nil {
			return nil, ErrOrderNotFound
		}
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
	if args.RequireAssociatedOrder {
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
	if args.OrderID != "" {
		contract, _, _, _, _, err := n.Datastore.Purchases().GetByOrderId(args.OrderID)
		if err == nil && contract != nil {
			return contract, nil
		}
	}
	contract, _, _, _, _, err := n.Datastore.Purchases().GetByOrderId(args.Address)
	if err == nil && contract != nil {
		return contract, nil
	}

	return nil, errors.New("unable to find order from order id or spend address")
}
