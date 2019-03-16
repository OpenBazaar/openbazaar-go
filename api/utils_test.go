package api

import (
	"testing"
)

func Test_extractModeratorChanges(t *testing.T) {
	currentModList := []string{"a", "b", "c"}
	newModList := []string{"a", "x", "c"}

	toAdd, toDelete := extractModeratorChanges(newModList, &currentModList)
	if len(toAdd) != 1 {
		t.Errorf("Returned incorrect number of additions: expected %d got %d", 1, len(toAdd))
	}
	if len(toDelete) != 1 {
		t.Errorf("Returned incorrect number of deletions: expected %d got %d", 1, len(toDelete))
	}

	if toAdd[0] != "x" {
		t.Errorf("Returned incorrect addition: expected a got %s", toAdd[0])
	}
	if toDelete[0] != "b" {
		t.Errorf("Returned incorrect deletion: expected b got %s", toDelete[0])
	}
}
