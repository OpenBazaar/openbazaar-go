package net

import (
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
)

// TestEnsureNoOmissionsInMessageProcessingOrder ensures that
// new message types are either included in MessageProcessingOrder
// or added to the BlackList, indicating it should not be added.
func TestEnsureNoOmissionsInMessageProcessingOrder(t *testing.T) {
	messages := make(map[string]int32, len(pb.Message_MessageType_value))
	for k, v := range pb.Message_MessageType_value {
		messages[k] = v
	}

	// Add deliberate omissions to this list
	blackList := map[pb.Message_MessageType]struct{}{
		pb.Message_PING:          {},
		pb.Message_OFFLINE_RELAY: {},
		pb.Message_STORE:         {},
		pb.Message_BLOCK:         {},
		pb.Message_ERROR:         {},
	}

	// Inclusion check
	for _, msgType := range MessageProcessingOrder {
		delete(messages, msgType.String())
	}
	// BlackList check
	for msgType := range blackList {
		delete(messages, msgType.String())
	}

	if l := len(messages); l > 0 {
		t.Errorf("found %d unexpected message types which are not considered in MessageProcessingOrder: %v", l, messages)
	}
}
