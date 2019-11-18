package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/golang/protobuf/proto"
	"math/big"
)

// ToV5DisputeResolution scans through the dispute resolution looking for any deprecated fields and
// turns them into their v5 counterpart.
func ToV5DisputeResolution(disputeResolution *pb.DisputeResolution) *pb.DisputeResolution {
	newDisputeResolution := proto.Clone(disputeResolution).(*pb.DisputeResolution)
	if disputeResolution.Payout == nil {
		return newDisputeResolution
	}

	if disputeResolution.Payout.BuyerOutput != nil &&
		disputeResolution.Payout.BuyerOutput.Amount != 0 &&
		disputeResolution.Payout.BuyerOutput.BigAmount == "" {
		newDisputeResolution.Payout.BuyerOutput.BigAmount = big.NewInt(int64(disputeResolution.Payout.BuyerOutput.Amount)).String()
		newDisputeResolution.Payout.BuyerOutput.Amount = 0
	}
	if disputeResolution.Payout.VendorOutput != nil &&
		disputeResolution.Payout.VendorOutput.Amount != 0 &&
		disputeResolution.Payout.VendorOutput.BigAmount == "" {
		newDisputeResolution.Payout.VendorOutput.BigAmount = big.NewInt(int64(disputeResolution.Payout.VendorOutput.Amount)).String()
		newDisputeResolution.Payout.VendorOutput.Amount = 0
	}
	if disputeResolution.Payout.ModeratorOutput != nil &&
		disputeResolution.Payout.ModeratorOutput.Amount != 0 &&
		disputeResolution.Payout.ModeratorOutput.BigAmount == "" {
		newDisputeResolution.Payout.ModeratorOutput.BigAmount = big.NewInt(int64(disputeResolution.Payout.ModeratorOutput.Amount)).String()
		newDisputeResolution.Payout.ModeratorOutput.Amount = 0
	}
	return newDisputeResolution
}
