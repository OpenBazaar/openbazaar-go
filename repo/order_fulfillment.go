package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"math/big"
)

// ToV5OrderFulfillment scans through the order fulfillment looking for any deprecated fields and
// turns them into their v5 counterpart.
func ToV5OrderFulfillment(orderFulfillment *pb.OrderFulfillment) *pb.OrderFulfillment {
	newOrderFulfillment := proto.Clone(orderFulfillment).(*pb.OrderFulfillment)

	if orderFulfillment.Payout.PayoutFeePerByte != 0 && orderFulfillment.Payout.BigPayoutFeePerByte == "" {
		newOrderFulfillment.Payout.BigPayoutFeePerByte = big.NewInt(int64(orderFulfillment.Payout.PayoutFeePerByte)).String()
		newOrderFulfillment.Payout.PayoutFeePerByte = 0
	}
	return newOrderFulfillment
}
