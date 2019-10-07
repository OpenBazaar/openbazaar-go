package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"testing"
)

func TestToV5DisputeResolution(t *testing.T) {
	var (
		disputeResolution = &pb.DisputeResolution{
			Payout: &pb.DisputeResolution_Payout{
				BuyerOutput: &pb.DisputeResolution_Payout_Output{
					Amount: 10000,
				},
				VendorOutput: &pb.DisputeResolution_Payout_Output{
					Amount: 10000,
				},
				ModeratorOutput: &pb.DisputeResolution_Payout_Output{
					Amount: 10000,
				},
			},
		}
		newDisputeResolution = ToV5DisputeResolution(disputeResolution)
		expected             = "10000"
	)

	if newDisputeResolution.Payout.BuyerOutput.BigAmount != expected {
		t.Errorf("Expected BigAmount of %s got %s", expected, newDisputeResolution.Payout.BuyerOutput.BigAmount)
	}
	if newDisputeResolution.Payout.VendorOutput.BigAmount != expected {
		t.Errorf("Expected BigAmount of %s got %s", expected, newDisputeResolution.Payout.VendorOutput.BigAmount)
	}
	if newDisputeResolution.Payout.ModeratorOutput.BigAmount != expected {
		t.Errorf("Expected BigAmount of %s got %s", expected, newDisputeResolution.Payout.ModeratorOutput.BigAmount)
	}

	if newDisputeResolution.Payout.BuyerOutput.Amount != 0 {
		t.Errorf("Expected Amount of 0, got %d", newDisputeResolution.Payout.BuyerOutput.Amount)
	}
	if newDisputeResolution.Payout.VendorOutput.Amount != 0 {
		t.Errorf("Expected Amount of 0, got %d", newDisputeResolution.Payout.VendorOutput.Amount)
	}
	if newDisputeResolution.Payout.ModeratorOutput.Amount != 0 {
		t.Errorf("Expected Amount of 0, got %d", newDisputeResolution.Payout.ModeratorOutput.Amount)
	}
}
