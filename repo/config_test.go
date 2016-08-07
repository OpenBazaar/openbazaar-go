package repo

import (
	"testing"
)

const testConfigPath = "testdata/config"

func TestGetAPIUsernameAndPw(t *testing.T) {
	username, pw, err := GetAPIUsernameAndPw(testConfigPath)
	if username != "TestUsername" {
		t.Error("Expected TestUsername, got ", username)
	}
	if pw != "TestPassword" {
		t.Error("Expected TestPassword, got ", pw)
	}
	if err != nil {
		t.Error("GetAPIUsernameAndPw threw an unexpected error")
	}

	username, pw, err = GetAPIUsernameAndPw("testdata/nonexistent")
	if username != "" {
		t.Error("Expected empty string, got ", username)
	}
	if pw != "" {
		t.Error("Expected empty string, got ", pw)
	}
	if err == nil {
		t.Error("GetAPIUsernameAndPw didn`t throw an error")
	}
}
