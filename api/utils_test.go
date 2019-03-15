package api

import (
	"testing"
)

func Test_extractModeratorChanges(t *testing.T) {
	currentModList := []string{"a", "b", "c"}
	newModList := []string{"a", "x", "c"}

	toAdd, toDelete := extractModeratorChanges(newModList, &currentModList)
	if len(toAdd) != 1 {
		t.Errorf("Test_extractModeratorChanges returned incorrect number of additions: expected %d got %d", 1, len(toAdd))
	}
	if len(toDelete) != 1 {
		t.Errorf("Test_extractModeratorChanges returned incorrect number of deletions: expected %d got %d", 1, len(toDelete))
	}

	if toAdd[0] != "x" {
		t.Errorf("Test_extractModeratorChanges returned incorrect addition: expected a got %s", toAdd[0])
	}
	if toDelete[0] != "b" {
		t.Errorf("Test_extractModeratorChanges returned incorrect deletion: expected b got %s", toDelete[0])
	}
}
