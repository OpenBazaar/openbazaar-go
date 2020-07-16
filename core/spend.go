package core

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcutil"
)

// DefaultCurrencyDivisibility is the Divisibility of the Currency if not
// defined otherwise
const DefaultCurrencyDivisibility uint = 8

type SpendRequest struct {
	decodedAddress btcutil.Address

	Amount                 string                   `json:"amount"`
	Currency               *repo.CurrencyDefinition `json:"currency"`
	CurrencyCode           string                   `json:"currencyCode"`
	Address                string                   `json:"address"`
	FeeLevel               string                   `json:"feeLevel"`
	Memo                   string                   `json:"memo"`
	OrderID                string                   `json:"orderId"`
	RequireAssociatedOrder bool                     `json:"requireOrder"`
	SpendAll               bool                     `json:"spendAll"`
}

type SpendResponse struct {
	Amount             string                   `json:"amount"`
	ConfirmedBalance   string                   `json:"confirmedBalance"`
	UnconfirmedBalance string                   `json:"unconfirmedBalance"`
	Currency           *repo.CurrencyDefinition `json:"currency"`
	Memo               string                   `json:"memo"`
	OrderID            string                   `json:"orderId"`
	Timestamp          time.Time                `json:"timestamp"`
	Txid               string                   `json:"txid"`
	PeerID             string                   `json:"-"`
	ConsumedInput      bool                     `json:"-"`
}

// Spend will attempt to move funds from the node to the destination address described in the
// SpendRequest for the amount indicated.
func (n *OpenBazaarNode) Spend(args *SpendRequest) (*SpendResponse, error) {
	var (
		feeLevel wallet.FeeLevel
		peerID   string

		amt        = new(big.Int)
		lookupCode = args.CurrencyCode
	)

	if lookupCode == "" && args.Currency != nil {
		lookupCode = args.Currency.Code.String()
	}
	var currencyDef, err = n.LookupCurrency(lookupCode)
	if err != nil {
		return nil, repo.ErrCurrencyDefinitionUndefined
	}
	if args.Currency != nil && currencyDef.Divisibility != args.Currency.Divisibility {
		currencyDef.Divisibility = args.Currency.Divisibility
		if err := currencyDef.Valid(); err != nil {
			return nil, err
		}
	}

	amt, ok := amt.SetString(args.Amount, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}

	wal, err := n.Multiwallet.WalletForCurrencyCode(lookupCode)
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
	case "SUPER_ECONOMIC":
		feeLevel = wallet.SUPER_ECONOMIC
	default:
		feeLevel = wallet.ECONOMIC
	}

	txid, err := wal.Spend(*amt, addr, feeLevel, args.OrderID, args.SpendAll)
	if err != nil {
		switch {
		case err == wallet.ErrInsufficientFunds:
			return nil, ErrInsufficientFunds
		case err == wallet.ErrorDustAmount:
			return nil, ErrSpendAmountIsDust
		default:
			return nil, err
		}
	}

	txn, err := wal.GetTransaction(*txid)
	if err != nil {
		log.Errorf("get txn failed : %v", err.Error())
		return nil, fmt.Errorf("failed retrieving new wallet balance: %s", err)
	}

	var (
		thumbnail string
		title     string
		memo      = args.Memo
		toAddress = args.Address
	)

	if txn.ToAddress != "" {
		toAddress = txn.ToAddress
	}

	if contract != nil && contract.VendorListings[0] != nil {
		if contract.VendorListings[0].Item != nil && len(contract.VendorListings[0].Item.Images) > 0 {
			thumbnail = contract.VendorListings[0].Item.Images[0].Tiny
			title = contract.VendorListings[0].Item.Title
		}
		if contract.VendorListings[0].VendorID != nil {
			peerID = contract.VendorListings[0].VendorID.PeerID
		}
	}
	if memo == "" && title != "" {
		memo = title
	}

	if err := n.Datastore.TxMetadata().Put(repo.Metadata{
		Txid:       txid.String(),
		Address:    toAddress,
		Memo:       memo,
		OrderId:    args.OrderID,
		Thumbnail:  thumbnail,
		CanBumpFee: false,
	}); err != nil {
		return nil, fmt.Errorf("failed persisting transaction metadata: %s", err)
	}

	confirmed, unconfirmed := wal.Balance()
	defn, err := n.LookupCurrency(wal.CurrencyCode())
	if err != nil {
		return nil, fmt.Errorf("wallet currency not found in dictionary")
	}

	return &SpendResponse{
		Txid:               txid.String(),
		ConfirmedBalance:   confirmed.Value.String(),
		UnconfirmedBalance: unconfirmed.Value.String(),
		Currency:           &defn,
		Amount:             strings.TrimPrefix(txn.Value, "-"),
		Timestamp:          txn.Timestamp,
		Memo:               memo,
		OrderID:            args.OrderID,
		PeerID:             peerID,
	}, nil
}

func (n *OpenBazaarNode) getOrderContractBySpendRequest(args *SpendRequest) (*pb.RicardianContract, error) {
	var errorStr = "unable to find order from order id or spend address"
	if args.OrderID != "" {
		contract, _, _, _, _, _, err := n.Datastore.Purchases().GetByOrderId(args.OrderID)
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
