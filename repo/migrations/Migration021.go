package migrations

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

// Migration021 migrates the config file to set the Swarm: EnableAutoRelay option
// to true.
type Migration021 struct{}

func (Migration021) Up(repoPath, dbPassword string, testnet bool) error {
	var (
		configMap        = map[string]interface{}{}
		configBytes, err = ioutil.ReadFile(path.Join(repoPath, "config"))
	)
	if err != nil {
		return fmt.Errorf("reading config: %s", err.Error())
	}

	if err = json.Unmarshal(configBytes, &configMap); err != nil {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}

	iSwarm, ok := configMap["Swarm"]
	if !ok {
		return fmt.Errorf("unmarshal config: missing swarm key")
	}

	swarm, ok := iSwarm.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unmarshal config: error type asserting swarm")
	}

	swarm["EnableAutoRelay"] = true
	swarm["EnableAutoNATService"] = false
	configMap["Swarm"] = swarm

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 22); err != nil {
		return fmt.Errorf("bumping repover to 18: %s", err.Error())
	}
	return nil
}

func (Migration021) Down(repoPath, dbPassword string, testnet bool) error {
	var (
		configMap        = map[string]interface{}{}
		configBytes, err = ioutil.ReadFile(path.Join(repoPath, "config"))
	)
	if err != nil {
		return fmt.Errorf("reading config: %s", err.Error())
	}

	if err = json.Unmarshal(configBytes, &configMap); err != nil {
		return fmt.Errorf("unmarshal config: %s", err.Error())
	}

	iSwarm, ok := configMap["Swarm"]
	if !ok {
		return fmt.Errorf("unmarshal config: missing swarm key")
	}

	swarm, ok := iSwarm.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unmarshal config: error type asserting swarm")
	}

	delete(swarm, "EnableAutoRelay")
	delete(swarm, "EnableAutoNATService")
	configMap["Swarm"] = swarm

	newConfigBytes, err := json.MarshalIndent(configMap, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal migrated config: %s", err.Error())
	}

	if err := ioutil.WriteFile(path.Join(repoPath, "config"), newConfigBytes, os.ModePerm); err != nil {
		return fmt.Errorf("writing migrated config: %s", err.Error())
	}

	if err := writeRepoVer(repoPath, 21); err != nil {
		return fmt.Errorf("dropping repover to 16: %s", err.Error())
	}
	return nil
}
