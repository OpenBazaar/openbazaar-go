package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"math/big"
)

// ToV5Order scans through the order looking for any deprecated fields and turns them into their v5 counterpart.
func ToV5Order(order *pb.Order, lookupFunc func(currencyCode string) (CurrencyDefinition, error)) (*pb.Order, error) {
	newOrder := proto.Clone(order).(*pb.Order)

	if order.RefundFee != 0 && order.BigRefundFee == "" {
		newOrder.BigRefundFee = big.NewInt(int64(order.RefundFee)).String()
		newOrder.RefundFee = 0
	}

	for i, item := range order.Items {
		if item.Quantity != 0 && item.BigQuantity == "" {
			newOrder.Items[i].BigQuantity = big.NewInt(int64(item.Quantity)).String()
			newOrder.Items[i].Quantity = 0
		}

		if item.Quantity64 != 0 && item.BigQuantity == "" {
			newOrder.Items[i].BigQuantity = big.NewInt(int64(item.Quantity64)).String()
			newOrder.Items[i].Quantity64 = 0
		}
	}

	if order.Payment.Amount != 0 && order.Payment.BigAmount == "" {
		newOrder.Payment.BigAmount = big.NewInt(int64(order.Payment.Amount)).String()
		newOrder.Payment.Amount = 0
	}

	if order.Payment.Coin != "" && order.Payment.AmountCurrency == nil {
		def, err := lookupFunc(order.Payment.Coin)
		if err != nil {
			return nil, err
		}
		newOrder.Payment.AmountCurrency = &pb.CurrencyDefinition{
			Code:         def.Code.String(),
			Divisibility: uint32(def.Divisibility),
		}
		newOrder.Payment.Coin = ""
	}
	return newOrder, nil
}
