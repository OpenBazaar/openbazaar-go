package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

// Migration018 migrates the config file to be compatible with the latest version of IPFS.
// We've also removed the resolvers config as we aren't using that any more and added
// an IpnsExtra config which contains fields we previously patched into the IPNS config.
type Migration018 struct{}

func (Migration018) Up(repoPath string, dbPassword string, testnet bool) error {
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	if err := json.Unmarshal(configFile, &cfgIface); err != nil {
		return err
	}
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("invalid config file")
	}

	// Update API address to use nil instead of ""
	addrsIface, ok := cfg["Addresses"]
	if !ok {
		return errors.New("missing addresses config")
	}
	addrs, ok := addrsIface.(map[string]interface{})
	if !ok {
		return errors.New("error parsing addresses config")
	}
	apiAddr, ok := addrs["API"]
	if !ok {
		return errors.New("missing API address")
	}
	apiAddrStr, ok := apiAddr.(string)
	if !ok {
		return errors.New("error parsing API addr")
	}
	if apiAddrStr == "" {
		addrs["API"] = nil
	}

	// Delete resolvers
	delete(cfg, "Resolvers")

	// Add APICommands to gateway
	gatewayIface, ok := cfg["Gateway"]
	if !ok {
		return errors.New("missing gateway config")
	}
	gatewayCfg, ok := gatewayIface.(map[string]interface{})
	if !ok {
		return errors.New("error parsing gateway config")
	}
	gatewayCfg["APICommands"] = nil

	// Modify IPNS config
	ipnsIface, ok := cfg["Ipns"]
	if !ok {
		return errors.New("missing ipns config")
	}
	ipnsCfg, ok := ipnsIface.(map[string]interface{})
	if !ok {
		return errors.New("error parsing ipns config")
	}
	ipnsCfg["RecordLifetime"] = "168h"
	delete(ipnsCfg, "BackUpAPI")
	delete(ipnsCfg, "UsePersistentCache")
	delete(ipnsCfg, "QuerySize")

	cfg["Ipns"] = ipnsCfg

	// Add Pubsub config
	cfg["Pubsub"] = map[string]interface{}{
		"DisableSigning":              false,
		"Router":                      "",
		"StrictSignatureVerification": false,
	}

	// Modify Experimental config
	expIface, ok := cfg["Experimental"]
	if !ok {
		return errors.New("missing experimental config")
	}
	expCfg, ok := expIface.(map[string]interface{})
	if !ok {
		return errors.New("error parsing experimental config")
	}
	expCfg["P2pHttpProxy"] = false
	expCfg["QUIC"] = false
	expCfg["UrlstoreEnabled"] = false

	cfg["Experimental"] = expCfg

	// Add IPNSExtra config
	cfg["IpnsExtra"] = map[string]interface{}{
		"DHTQuorumSize": 1,
		"FallbackAPI":   "https://gateway.ob1.io",
	}

	out, err := json.MarshalIndent(cfg, "", "   ")
	if err != nil {
		return err
	}
	f, err := os.Create(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	_, err = f.Write(out)
	if err != nil {
		return err
	}
	f.Close()
	if err := writeRepoVer(repoPath, 19); err != nil {
		return fmt.Errorf("bumping repover to 19: %s", err.Error())
	}
	if err := writeIPFSVer(repoPath, 7); err != nil {
		return fmt.Errorf("bumping repover to 19: %s", err.Error())
	}
	return nil
}

func (Migration018) Down(repoPath string, dbPassword string, testnet bool) error {
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	if err := json.Unmarshal(configFile, &cfgIface); err != nil {
		return err
	}
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("invalid config file")
	}

	// Update API address to use "" instead of nil
	addrsIface, ok := cfg["Addresses"]
	if !ok {
		return errors.New("missing addresses config")
	}
	addrs, ok := addrsIface.(map[string]interface{})
	if !ok {
		return errors.New("error parsing addresses config")
	}
	apiAddr, ok := addrs["API"]
	if !ok {
		return errors.New("missing API address")
	}
	if apiAddr == nil {
		addrs["API"] = ""
	}

	// Add resolvers
	cfg["Resolvers"] = map[string]interface{}{
		".eth": "",
		".id":  "https://resolver.onename.com/",
	}

	// Delete APICommands to gateway
	gatewayIface, ok := cfg["Gateway"]
	if !ok {
		return errors.New("missing gateway config")
	}
	gatewayCfg, ok := gatewayIface.(map[string]interface{})
	if !ok {
		return errors.New("error parsing gateway config")
	}
	delete(gatewayCfg, "APICommands")

	// Modify IPNS config
	ipnsIface, ok := cfg["Ipns"]
	if !ok {
		return errors.New("missing ipns config")
	}
	ipnsCfg, ok := ipnsIface.(map[string]interface{})
	if !ok {
		return errors.New("error parsing ipns config")
	}
	ipnsCfg["RecordLifetime"] = "7d"
	ipnsCfg["BackUpAPI"] = "https://gateway.ob1.io"
	ipnsCfg["UsePersistentCache"] = true
	ipnsCfg["QuerySize"] = 1

	cfg["Ipns"] = ipnsCfg

	// Delete Pubsub config
	delete(cfg, "Pubsub")

	// Modify Experimental config
	expIface, ok := cfg["Experimental"]
	if !ok {
		return errors.New("missing experimental config")
	}
	expCfg, ok := expIface.(map[string]interface{})
	if !ok {
		return errors.New("error parsing experimental config")
	}
	delete(expCfg, "P2pHttpProxy")
	delete(expCfg, "QUIC")
	delete(expCfg, "UrlstoreEnabled")

	cfg["Experimental"] = expCfg

	// Delete IPNSExtra config
	delete(cfg, "IpnsExtra")

	out, err := json.MarshalIndent(cfg, "", "   ")
	if err != nil {
		return err
	}
	f, err := os.Create(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	_, err = f.Write(out)
	if err != nil {
		return err
	}
	f.Close()
	if err := writeRepoVer(repoPath, 18); err != nil {
		return fmt.Errorf("downgrading repover to 18: %s", err.Error())
	}
	if err := writeIPFSVer(repoPath, 6); err != nil {
		return fmt.Errorf("bumping repover to 19: %s", err.Error())
	}
	return nil
}
