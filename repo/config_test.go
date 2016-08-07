package repo

import (
	"reflect"
	"testing"
)

const testConfigPath = "testdata/config"
const nonexistentTestConfigPath = "testdata/nonexistent"

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

	username, pw, err = GetAPIUsernameAndPw(nonexistentTestConfigPath)
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

func TestGetAPIHeaders(t *testing.T) {
	headers, err := GetAPIHeaders(testConfigPath)
	if reflect.ValueOf(headers).Kind() != reflect.Map {
		t.Error("headers is not a map")
	}
	if err != nil {
		t.Error("GetAPIHeaders threw an unexpected error")
	}

	headers, err = GetAPIHeaders(nonexistentTestConfigPath)
	if reflect.ValueOf(headers).Kind() != reflect.Map {
		t.Error("headers is not a map")
	}
	if err == nil {
		t.Error("GetAPIHeaders didn`t throw an error")
	}
}

func TestGetAPIEnabled(t *testing.T) {
	enabled, err := GetAPIEnabled(testConfigPath)
	if enabled != true {
		t.Error("enabled is not true")
	}
	if err != nil {
		t.Error("GetAPIEnabled threw an unexpected error")
	}

	enabled, err = GetAPIEnabled(nonexistentTestConfigPath)
	if enabled != false {
		t.Error("enabled is not false when path to config file is nonexistent")
	}
	if err == nil {
		t.Error("GetAPIEnabled didn`t throw an error")
	}
}

func TestGetAPICORS(t *testing.T) {
	cors, err := GetAPICORS(testConfigPath)
	if cors != true {
		t.Error("cors is not true")
	}
	if err != nil {
		t.Error("GetAPICORS threw an unexpected error")
	}

	cors, err = GetAPICORS(nonexistentTestConfigPath)
	if cors != false {
		t.Error("cors is not false when path to config file is nonexistent")
	}
	if err == nil {
		t.Error("GetAPICORS didn`t throw an error")
	}
}
