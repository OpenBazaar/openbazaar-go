package notifications

import "testing"

func TestSerialization(t *testing.T) {
	notif := StatusNotification{"some status string"}
	val := string(Serialize(notif))
	str := `{"status":"some status string"}`
	if val != str {
		t.Error("Incorrect serialization")
	}
}
