package migrations

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
)

type Migration001 struct{}

func (Migration001) Up(repoPath string, dbPassword string, testnet bool) error {
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	json.Unmarshal(configFile, &cfgIface)
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("Invalid config file")
	}

	cfg["Datastore"] = map[string]interface{}{
		"StorageMax":         "10GB",
		"StorageGCWatermark": 90,
		"GCPeriod":           "1h",
		"BloomFilterSize":    0,
		"HashOnRead":         false,
		"Spec": map[string]interface{}{
			"type": "mount",
			"mounts": []interface{}{
				map[string]interface{}{
					"mountpoint": "/blocks",
					"type":       "measure",
					"prefix":     "flatfs.datastore",
					"child": map[string]interface{}{
						"type":      "flatfs",
						"path":      "blocks",
						"sync":      true,
						"shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
					},
				},
				map[string]interface{}{
					"mountpoint": "/",
					"type":       "measure",
					"prefix":     "leveldb.datastore",
					"child": map[string]interface{}{
						"type":        "levelds",
						"path":        "datastore",
						"compression": "none",
					},
				},
			},
		},
	}

	ipnsIface, ok := cfg["Ipns"]
	if !ok {
		return errors.New("Missing ipns config")
	}
	ipns, ok := ipnsIface.(map[string]interface{})
	if !ok {
		return errors.New("Error parsing ipns config")
	}
	ipns["UsePersistentCache"] = true
	cfg["Ipns"] = ipns

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

	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("2"))
	if err != nil {
		return err
	}
	f1.Close()

	f2, err := os.Create(path.Join(repoPath, "datastore_spec"))
	if err != nil {
		return err
	}
	_, err = f2.Write([]byte(`{"mounts":[{"mountpoint":"/blocks","path":"blocks","shardFunc":"/repo/flatfs/shard/v1/next-to-last/2","type":"flatfs"},{"mountpoint":"/","path":"datastore","type":"levelds"}],"type":"mount"}`))
	if err != nil {
		return err
	}
	f2.Close()

	f3, err := os.Create(path.Join(repoPath, "version"))
	if err != nil {
		return err
	}
	_, err = f3.Write([]byte(`6`))
	if err != nil {
		return err
	}
	f3.Close()
	return nil
}

func (Migration001) Down(repoPath string, dbPassword string, testnet bool) error {
	configFile, err := ioutil.ReadFile(path.Join(repoPath, "config"))
	if err != nil {
		return err
	}
	var cfgIface interface{}
	json.Unmarshal(configFile, &cfgIface)
	cfg, ok := cfgIface.(map[string]interface{})
	if !ok {
		return errors.New("Invalid config file")
	}

	cfg["Datastore"] = map[string]interface{}{
		"BloomFilterSize":    0,
		"GCPeriod":           "1h",
		"HashOnRead":         false,
		"NoSync":             false,
		"Params":             nil,
		"Path":               path.Join(repoPath, "datastore"),
		"StorageGCWatermark": 90,
		"StorageMax":         "10GB",
		"Type":               "leveldb",
	}

	ipnsIface, ok := cfg["Ipns"]
	if !ok {
		return errors.New("Missing ipns config")
	}
	ipns, ok := ipnsIface.(map[string]interface{})
	if !ok {
		return errors.New("Error parsing ipns config")
	}
	delete(ipns, "UsePersistentCache")
	cfg["Ipns"] = ipns

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

	f1, err := os.Create(path.Join(repoPath, "repover"))
	if err != nil {
		return err
	}
	_, err = f1.Write([]byte("1"))
	if err != nil {
		return err
	}
	f1.Close()

	f2, err := os.Create(path.Join(repoPath, "version"))
	if err != nil {
		return err
	}
	_, err = f2.Write([]byte(`5`))
	if err != nil {
		return err
	}
	f2.Close()
	os.RemoveAll(path.Join(repoPath, "datastore_spec"))
	return nil
}
