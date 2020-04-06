package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"math/big"
)

// ToV5OrderConfirmation scans through the order confirmation looking for any deprecated fields and
// turns them into their v5 counterpart.
func ToV5OrderConfirmation(orderConfirmation *pb.OrderConfirmation) *pb.OrderConfirmation {
	newOrderConfirmation := proto.Clone(orderConfirmation).(*pb.OrderConfirmation)

	if orderConfirmation.RequestedAmount != 0 && orderConfirmation.BigRequestedAmount == "" {
		newOrderConfirmation.BigRequestedAmount = big.NewInt(int64(orderConfirmation.RequestedAmount)).String()
		newOrderConfirmation.RequestedAmount = 0
	}
	return newOrderConfirmation
}
