package repo_test

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
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
		newDisputeResolution = repo.ToV5DisputeResolution(disputeResolution)
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

func TestToV5DisputeResolutionHandlesMissingOutputs(t *testing.T) {
	var (
		examples = []*pb.DisputeResolution{
			{ // missing BuyerOutput
				Payout: &pb.DisputeResolution_Payout{
					BuyerOutput: nil,
					VendorOutput: &pb.DisputeResolution_Payout_Output{
						Amount: 10000,
					},
					ModeratorOutput: &pb.DisputeResolution_Payout_Output{
						Amount: 10000,
					},
				},
			},
			{ // missing VendorOutput
				Payout: &pb.DisputeResolution_Payout{
					BuyerOutput: &pb.DisputeResolution_Payout_Output{
						Amount: 10000,
					},
					VendorOutput: nil,
					ModeratorOutput: &pb.DisputeResolution_Payout_Output{
						Amount: 10000,
					},
				},
			},
			{ // missing ModeratorOutput
				Payout: &pb.DisputeResolution_Payout{
					BuyerOutput: &pb.DisputeResolution_Payout_Output{
						Amount: 10000,
					},
					VendorOutput: &pb.DisputeResolution_Payout_Output{
						Amount: 10000,
					},
					ModeratorOutput: nil,
				},
			},
		}
	)

	for _, e := range examples {
		repo.ToV5DisputeResolution(e)
	}
}

func TestToV5DisputeResolutionHandlesMissingPayout(t *testing.T) {
	var example = &pb.DisputeResolution{Payout: nil}
	repo.ToV5DisputeResolution(example)
}
