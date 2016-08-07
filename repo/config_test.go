package repo

import (
	"testing"
)

const testConfigPath = "testdata/"

func TestCredentials(t *testing.T) {
	username, pw, err := Credentials(testConfigPath + "configExample.json")
	if username != "TestUsername" {
		t.Error("Expected TestUsername, got ", username)
	}
	if pw != "TestPassword" {
		t.Error("Expected TestPassword, got ", pw)
	}
	if err != nil {
		t.Error("Credentials threw an unexpected error")
	}

	username, pw, err = Credentials("testdata/nonexistent")
	if username != "" {
		t.Error("Expected empty string, got ", username)
	}
	if pw != "" {
		t.Error("Expected empty string, got ", pw)
	}
	if err == nil {
		t.Error("Credentials didn't throw an error")
	}
}

func TestCredentialsMissingPassword(t *testing.T) {
    username, password, err := Credentials(testConfigPath + "configCredentialsMissingPassword.json")

    if err != nil {
        t.Error("No error expected")
    }
    if username != "duosearch" {
        t.Error("Username improperly parsed")
    }
    if password != "" {
        t.Error("Password improperly parsed")
    }
}
