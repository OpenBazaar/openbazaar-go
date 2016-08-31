package core

import (
	"errors"
	"github.com/OpenBazaar/openbazaar-go/pb"
)

func validateOrderConfirmation(conf *pb.OrderConfirmation, order *pb.Order) error {
	orderID, err := calcOrderId(order)
	if err != nil {
		return err
	}
	if conf.OrderID != orderID {
		return errors.New("Vendor's response contained invalid order ID")
	}
	if conf.RequestedAmount != order.Payment.Amount {
		return errors.New("Vendor requested an amount different from what we calculated")
	}
	//TODO: validate rating signature
	return nil
}
