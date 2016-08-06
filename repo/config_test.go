package repo

import (
	"testing"
)

func TestGetAPIUsernameAndPw(t *testing.T) {
	var path = "testdata/config"
	username, pw, err := GetAPIUsernameAndPw(path)
	if username != "TestUsername" {
		t.Error("Expected TestUsername, got ", username)
	}
	if pw != "TestPassword" {
		t.Error("Expected TestPassword, got ", pw)
	}
	if err != nil {
		t.Error("GetAPIUsernameAndPw threw an unexpected error")
	}

	path = "testdata/nonexistent"
	username, pw, err = GetAPIUsernameAndPw(path)
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