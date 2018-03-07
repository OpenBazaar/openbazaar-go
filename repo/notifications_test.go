package repo

import (
	"encoding/json"
	"testing"
)

func TestSerialization(t *testing.T) {
	notif := StatusNotification{"some status string"}
	val := string(Serialize(notif))
	j, err := json.MarshalIndent(notif, "", "    ")
	if err != nil {
		t.Error(err)
	}
	if string(val) != string(j) {
		t.Error("Incorrect serialization")
	}
}

type simpleNotifier struct {
	ID, Type string
}

func (n *simpleNotifier) GetNotificationID() string   { return n.ID }
func (n *simpleNotifier) GetNotificationType() string { return n.Type }

func TestGetDisputeCaseID(t *testing.T) {
	negativeCase := simpleNotifier{
		ID:   "test",
		Type: "test",
	}
	_, err := GetDisputeCaseID(&negativeCase)
	if err == nil {
		t.Error("Expected simpleNotifier to return error when getting dispute-specific CaseID")
	}

	positiveCase := DisputeNotification{
		ID:     "test",
		Type:   "test",
		CaseID: "expectedID",
	}
	actualID, err := GetDisputeCaseID(&positiveCase)
	if err != nil {
		t.Error(err)
	}
	if actualID != positiveCase.CaseID {
		t.Error("Expected the returned CaseID to match")
	}
}
