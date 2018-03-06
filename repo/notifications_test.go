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
