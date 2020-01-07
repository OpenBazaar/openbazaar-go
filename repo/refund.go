package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"math/big"
)

// ToV5Refund scans through the refund looking for any deprecated fields and
// turns them into their v5 counterpart.
func ToV5Refund(refund *pb.Refund) *pb.Refund {
	newRefund := proto.Clone(refund).(*pb.Refund)

	if refund.RefundTransaction.Value != 0 && refund.RefundTransaction.BigValue == "" {
		newRefund.RefundTransaction.BigValue = big.NewInt(int64(refund.RefundTransaction.Value)).String()
		newRefund.RefundTransaction.Value = 0
	}
	return newRefund
}
