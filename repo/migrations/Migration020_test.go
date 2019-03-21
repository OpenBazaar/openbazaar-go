package migrations_test

import (
	"encoding/hex"
	"encoding/json"
	"gx/ipfs/QmPEpj17FDRpc7K1aArKZp3RsHtzRMKykeK9GVgn4WQGPR/go-ipfs-config"
	"gx/ipfs/QmS73grfbWgWrNztd8Lns9GCG3jjRNDfcPYg2VYQzKDZSt/go-ipfs-ds-help"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	"gx/ipfs/QmfVj3x4D6Jkq9SEoi5n2NmoUomLwoeiwnYz2KQa15wRw6/base32"
	"io/ioutil"
	"os"
	"testing"

	"gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/proto"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	dhtpb "github.com/OpenBazaar/openbazaar-go/repo/migrations/helpers/Migration020"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

func TestMigration020(t *testing.T) {
	var testRepo, err = schema.NewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer testRepo.DestroySchemaDirectories()

	if err = testRepo.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	if err := testRepo.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := testRepo.InitializeIPFSRepo(); err != nil {
		t.Fatal(err)
	}

	var (
		oldIPNSRecordHex = `0a282f69706e732f12201e96ac2751c730f45798aa472e290e987ec08cac23b6ee827aa7e292e0039ef3129c010a342f697066732f516d554e4c4c73504143437a31764c7851566b5871714c5835523158333435717166486273663637687641334e6e1240ce97421affaed4a97b00d212e2c5d73d8b7d6651aa969db5692157aa7757707acfa3933781175bd62ab39e728fac8295379d2b0a47af977b7c3a22b7f4d52b0b1800221e323031392d30332d31395432303a35383a30322e3135303439373036345a28011a2212201e96ac2751c730f45798aa472e290e987ec08cac23b6ee827aa7e292e0039ef3`
		newIPNSRecordHex = `0a342f697066732f516d554e4c4c73504143437a31764c7851566b5871714c5835523158333435717166486273663637687641334e6e1240ce97421affaed4a97b00d212e2c5d73d8b7d6651aa969db5692157aa7757707acfa3933781175bd62ab39e728fac8295379d2b0a47af977b7c3a22b7f4d52b0b1800221e323031392d30332d31395432303a35383a30322e3135303439373036345a2801`
		datastoreSpec    = `{"mounts":[{"mountpoint":"/blocks","path":"blocks","shardFunc":"/repo/flatfs/shard/v1/next-to-last/2","type":"flatfs"},{"mountpoint":"/","path":"datastore","type":"levelds"}],"type":"mount"}`

		configPath        = testRepo.DataPathJoin("config")
		datastoreSpecPath = testRepo.DataPathJoin("datastore_spec")
		repoverPath       = testRepo.DataPathJoin("repover")
		ipfsverPath       = testRepo.DataPathJoin("version")
	)

	config, err := config.Init(os.Stdout, 2048)
	if err != nil {
		t.Fatal(err)
	}

	identity, err := ipfs.IdentityFromKey(testRepo.IdentityKey())
	if err != nil {
		t.Fatal(err)
	}

	config.Identity = identity
	configBytes, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(configPath, configBytes, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(datastoreSpecPath, []byte(datastoreSpec), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	err = fsrepo.Init(testRepo.DataPath(), config)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(ipfsverPath, []byte("7"), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	r, err := fsrepo.Open(testRepo.DataPath())
	if err != nil {
		t.Fatal(err)
	}
	recordBytes, err := hex.DecodeString(oldIPNSRecordHex)
	if err != nil {
		t.Fatal(err)
	}
	peerID, err := peer.IDB58Decode(config.Identity.PeerID)
	if err != nil {
		t.Fatal(err)
	}
	_, ipns := migrations.IPNSKeysForID(peerID)
	if err := r.Datastore().Put(dshelp.NewKeyFromBinary([]byte(ipns)), recordBytes); err != nil {
		t.Fatal(err)
	}
	r.Close()

	if err := ioutil.WriteFile(ipfsverPath, []byte("6"), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(repoverPath, []byte("20"), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	var m migrations.Migration020
	err = m.Up(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	assertCorrectRepoVer(t, repoverPath, "21")
	assertCorrectRepoVer(t, ipfsverPath, "7")

	r, err = fsrepo.Open(testRepo.DataPath())
	if err != nil {
		t.Fatal(err)
	}
	newkey := ds.NewKey("/ipns/" + base32.RawStdEncoding.EncodeToString([]byte(peerID)))
	val, err := r.Datastore().Get(newkey)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()

	if hex.EncodeToString(val) != newIPNSRecordHex {
		t.Error("Record did not return expected value")
	}

	err = m.Down(testRepo.DataPath(), "", true)
	if err != nil {
		t.Fatal(err)
	}

	r, err = fsrepo.Open(testRepo.DataPath())
	if err != nil {
		t.Fatal(err)
	}
	oldkey := dshelp.NewKeyFromBinary([]byte(ipns))
	val, err = r.Datastore().Get(oldkey)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()

	var rec dhtpb.Migration020RecordOldFormat
	err = proto.Unmarshal(val, &rec)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(rec.Value) != newIPNSRecordHex {
		t.Error("Record did not return expected value")
	}

	assertCorrectRepoVer(t, repoverPath, "20")
	assertCorrectRepoVer(t, ipfsverPath, "6")
}
