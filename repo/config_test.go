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
		t.Error("GetAPIHeaders didn't throw an error")
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
		t.Error("GetAPIEnabled didn't throw an error")
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
		t.Error("GetAPICORS didn't throw an error")
	}
}

func TestGetFeeAPI(t *testing.T) {
	feeApi, err := GetFeeAPI(testConfigPath)
	if feeApi != "https://bitcoinfees.21.co/api/v1/fees/recommended" {
		t.Error("feeApi does not equal expected value")
	}
	if err != nil {
		t.Error("GetFeeAPI threw an unexpected error")
	}

	feeApi, err = GetFeeAPI(nonexistentTestConfigPath)
	if feeApi != "" {
		t.Error("Expected empty string, got ", feeApi)
	}
	if err == nil {
		t.Error("GetFeeAPI didn't throw an error")
	}
}

func TestGetDefaultFees(t *testing.T) {
	low, medium, high, err := GetDefaultFees(testConfigPath)
	if low != 20 {
		t.Error("Expected low to be 20, got ", low)
	}
	if medium != 40 {
		t.Error("Expected medium to be 40, got ", medium)
	}
	if high != 60 {
		t.Error("Expected high to be 60, got ", high)
	}
	if err != nil {
		t.Error("GetDefaultFees threw an unexpected error")
	}

	low, medium, high, err = GetDefaultFees(nonexistentTestConfigPath)
	if low != 0 {
		t.Error("Expected low to be 0, got ", low)
	}
	if medium != 0 {
		t.Error("Expected medium to be 0, got ", medium)
	}
	if high != 0 {
		t.Error("Expected high to be 0, got ", high)
	}
	if err == nil {
		t.Error("GetDefaultFees didn't throw an error")
	}
}

func TestGetMaxFee(t *testing.T) {
	maxFee, err := GetMaxFee(testConfigPath)
	if maxFee != 2000 {
		t.Error("Expected maxFee to be 2000, got ", maxFee)
	}
	if err != nil {
		t.Error("GetMaxFee threw an unexpected error")
	}

	maxFee, err = GetMaxFee(nonexistentTestConfigPath)
	if maxFee != 0 {
		t.Error("Expected maxFee to be 0, got ", maxFee)
	}
	if err == nil {
		t.Error("GetMaxFee didn't throw an error")
	}
}

func TestGetDropboxApiToken(t *testing.T) {
	dropboxApiToken, err := GetDropboxApiToken(testConfigPath)
	if dropboxApiToken != "dropbox123" {
		t.Error("dropboxApiToken does not equal expected value")
	}
	if err != nil {
		t.Error("GetDropboxApiToken threw an unexpected error")
	}

	dropboxApiToken, err = GetDropboxApiToken(nonexistentTestConfigPath)
	if dropboxApiToken != "" {
		t.Error("Expected empty string, got ", dropboxApiToken)
	}
	if err == nil {
		t.Error("GetDropboxApiToken didn't throw an error")
	}
}

func TestGetResolverUrl(t *testing.T) {
	resolverUrl, err := GetResolverUrl(testConfigPath)
	if resolverUrl != "https://resolver.onename.com/" {
		t.Error("resolverUrl does not equal expected value")
	}
	if err != nil {
		t.Error("GetResolverUrl threw an unexpected error")
	}

	resolverUrl, err = GetResolverUrl(nonexistentTestConfigPath)
	if resolverUrl != "" {
		t.Error("Expected empty string, got ", resolverUrl)
	}
	if err == nil {
		t.Error("GetResolverUrl didn't throw an error")
	}
}
