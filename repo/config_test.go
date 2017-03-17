package repo

import (
	"reflect"
	"testing"

	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"os"
	"path/filepath"
)

const testConfigFolder = "testdata"
const testConfigPath = "testdata/config"
const nonexistentTestConfigPath = "testdata/nonexistent"

func TestGetApiConfig(t *testing.T) {
	config, err := GetAPIConfig(testConfigPath)
	if config.Username != "TestUsername" {
		t.Error("Expected TestUsername, got ", config.Username)
	}
	if config.Password != "TestPassword" {
		t.Error("Expected TestPassword, got ", config.Password)
	}
	if !config.Authenticated {
		t.Error("Expected Authenticated = true")
	}
	if len(config.AllowedIPs) != 1 || config.AllowedIPs[0] != "127.0.0.1" {
		t.Error("Expected AllowedIPs = [127.0.0.1]")
	}
	if config.CORS == nil {
		t.Error("Cors is not set")
	}
	if reflect.ValueOf(config.HTTPHeaders).Kind() != reflect.Map {
		t.Error("Headers is not a map")
	}
	if config.Enabled != true {
		t.Error("Enabled is not true")
	}
	if !config.SSL {
		t.Error("Expected SSL = true")
	}
	if config.SSLCert == "" {
		t.Error("Expected test SSL cert, got ", config.SSLCert)
	}
	if config.SSLKey == "" {
		t.Error("Expected test SSL key, got ", config.SSLKey)
	}
	if err != nil {
		t.Error("GetAPIAuthentication threw an unexpected error")
	}

	_, err = GetAPIConfig(nonexistentTestConfigPath)
	if err == nil {
		t.Error("GetAPIAuthentication didn`t throw an error")
	}
}

func TestGetWalletConfig(t *testing.T) {
	config, err := GetWalletConfig(testConfigPath)
	if config.FeeAPI != "https://bitcoinfees.21.co/api/v1/fees/recommended" {
		t.Error("FeeApi does not equal expected value")
	}
	if config.TrustedPeer != "127.0.0.1:8333" {
		t.Error("TrustedPeer does not equal expected value")
	}
	if config.Type != "spvwallet" {
		t.Error("Type does not equal expected value")
	}
	if config.RPCUser != "username" {
		t.Error("RPC user does not equal expected value")
	}
	if config.RPCPassword != "password" {
		t.Error("RPC password does not equal expected value")
	}
	if config.Binary != "/path/to/bitcoind" {
		t.Error("Binary does not equal expected value")
	}
	if config.LowFeeDefault != 20 {
		t.Error("Expected low to be 20, got ", config.LowFeeDefault)
	}
	if config.MediumFeeDefault != 40 {
		t.Error("Expected medium to be 40, got ", config.MediumFeeDefault)
	}
	if config.HighFeeDefault != 60 {
		t.Error("Expected high to be 60, got ", config.HighFeeDefault)
	}
	if config.MaxFee != 2000 {
		t.Error("Expected maxFee to be 2000, got ", config.MaxFee)
	}
	if err != nil {
		t.Error("GetFeeAPI threw an unexpected error")
	}

	_, err = GetWalletConfig(nonexistentTestConfigPath)
	if err == nil {
		t.Error("GetFeeAPI didn't throw an error")
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

func TestExtendConfigFile(t *testing.T) {
	r, err := fsrepo.Open(testConfigFolder)
	if err != nil {
		t.Error("fsrepo.Open threw an unexpected error", err)
		return
	}
	config, _ := GetWalletConfig(testConfigPath)
	originalMaxFee := config.MaxFee
	newMaxFee := config.MaxFee + 1
	if err := extendConfigFile(r, "Wallet.MaxFee", newMaxFee); err != nil {
		t.Error("extendConfigFile threw an unexpected error ", err)
		return
	}
	config, _ = GetWalletConfig(testConfigPath)
	if config.MaxFee != newMaxFee {
		t.Errorf("Expected maxFee to be %v, got %v", newMaxFee, config.MaxFee)
		return
	}
	// Reset maxFee to original value
	extendConfigFile(r, "Wallet.MaxFee", originalMaxFee)

	// Teardown
	os.RemoveAll(filepath.Join(testConfigFolder, "datastore"))
	os.RemoveAll(filepath.Join(testConfigFolder, "repo.lock"))
}

func TestInitConfig(t *testing.T) {
	config, err := InitConfig(testConfigFolder)
	if config == nil {
		t.Error("config empty", err)
	}
	if err != nil {
		t.Error("InitConfig threw an unexpected error")
	}
	if config.Addresses.Gateway != "/ip4/127.0.0.1/tcp/4002" {
		t.Error("config.Addresses.Gateway is not set")
	}
}
