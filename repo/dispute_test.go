package repo

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"testing"
)

func TestToV5Dispute(t *testing.T) {
	var (
		dispute = &pb.Dispute{
			Outpoints: []*pb.Outpoint{
				{
					Value: 10000,
				},
			},
		}
		newDispute = ToV5Dispute(dispute)
		expected   = "10000"
	)

	if newDispute.Outpoints[0].BigValue != expected {
		t.Errorf("Expected BigValue of %s got %s", expected, newDispute.Outpoints[0].BigValue)
	}

	if newDispute.Outpoints[0].Value != 0 {
		t.Errorf("Expected Value of 0, got %d", newDispute.Outpoints[0].Value)
	}
}
