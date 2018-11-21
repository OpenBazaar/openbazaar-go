package migrations

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

var testConfig1 = `{
    "Datastore": {
	    "BloomFilterSize": 0,
	    "GCPeriod": "1h",
	    "HashOnRead": false,
	    "NoSync": false,
	    "Params": null,
	    "Path": "/home/chris/.openbazaar3/datastore",
	    "StorageGCWatermark": 90,
	    "StorageMax": "10GB",
	    "Type": "leveldb"
    },
    "Ipns": {
      "QuerySize": 1,
      "RecordLifetime": "7d",
      "RepublishPeriod": "24h",
      "ResolveCacheSize": 128
   }
}`

func TestMigration001(t *testing.T) {
	f, err := os.Create("./config")
	if err != nil {
		t.Error(err)
	}
	f.Write([]byte(testConfig1))
	f.Close()
	var m Migration001

	// Up
	err = m.Up("./", "", false)
	if err != nil {
		t.Error(err)
	}
	newConfig, err := ioutil.ReadFile("./config")
	if err != nil {
		t.Error(err)
	}
	newConfigCheck := `
	   "Datastore": {
	      "BloomFilterSize": 0,
	      "GCPeriod": "1h",
	      "HashOnRead": false,
	      "Spec": {
		 "mounts": [
		    {
		       "child": {
			  "path": "blocks",
			  "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
			  "sync": true,
			  "type": "flatfs"
		       },
		       "mountpoint": "/blocks",
		       "prefix": "flatfs.datastore",
		       "type": "measure"
		    },
		    {
		       "child": {
			  "compression": "none",
			  "path": "datastore",
			  "type": "levelds"
		       },
		       "mountpoint": "/",
		       "prefix": "leveldb.datastore",
		       "type": "measure"
		    }
		 ],
		 "type": "mount"
	      },
	      "StorageGCWatermark": 90,
	      "StorageMax": "10GB"
	   },
	   "Ipns": {
      "QuerySize": 1,
      "RecordLifetime": "7d",
      "RepublishPeriod": "24h",
      "ResolveCacheSize": 128,
      "UsePersistentCache": true
   }`
	if strings.Contains(string(newConfig), newConfigCheck) {
		t.Error("Failed to write new Datastore")
	}
	repoVer, err := ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "2" {
		t.Error("Failed to write new repo version")
	}

	repoVer, err = ioutil.ReadFile("./version")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "6" {
		t.Error("Failed to write new ipfs repo version")
	}

	spec, err := ioutil.ReadFile("./datastore_spec")
	if err != nil {
		t.Error(err)
	}
	if string(spec) != `{"mounts":[{"mountpoint":"/blocks","path":"blocks","shardFunc":"/repo/flatfs/shard/v1/next-to-last/2","type":"flatfs"},{"mountpoint":"/","path":"datastore","type":"levelds"}],"type":"mount"}` {
		t.Error("Failed to write datastore_spec")
	}

	// Down
	err = m.Down("./", "", false)
	if err != nil {
		t.Error(err)
	}
	newConfig, err = ioutil.ReadFile("./config")
	if err != nil {
		t.Error(err)
	}
	newConfigCheck = `{
   "Datastore": {
      "BloomFilterSize": 0,
      "GCPeriod": "1h",
      "HashOnRead": false,
      "NoSync": false,
      "Params": null,
      "Path": "datastore",
      "StorageGCWatermark": 90,
      "StorageMax": "10GB",
      "Type": "leveldb"
   },
   "Ipns": {
      "QuerySize": 1,
      "RecordLifetime": "7d",
      "RepublishPeriod": "24h",
      "ResolveCacheSize": 128
   }
}`
	if !strings.Contains(string(newConfig), newConfigCheck) {
		t.Error("Failed to write new Datastore")
	}
	repoVer, err = ioutil.ReadFile("./repover")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "1" {
		t.Error("Failed to write new repo version")
	}

	repoVer, err = ioutil.ReadFile("./version")
	if err != nil {
		t.Error(err)
	}
	if string(repoVer) != "5" {
		t.Error("Failed to write new ipfs repo version")
	}

	os.Remove("./config")
	os.Remove("./repover")
	os.Remove("./version")
	os.Remove("./datastore_spec")
}
