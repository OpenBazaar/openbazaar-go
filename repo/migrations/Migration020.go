package migrations

import (
	"bytes"
	"database/sql"
	"fmt"
	"path"

	"github.com/OpenBazaar/openbazaar-go/ipfs"

	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	"gx/ipfs/QmS73grfbWgWrNztd8Lns9GCG3jjRNDfcPYg2VYQzKDZSt/go-ipfs-ds-help"
	"gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	"gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/proto"
	"gx/ipfs/QmfVj3x4D6Jkq9SEoi5n2NmoUomLwoeiwnYz2KQa15wRw6/base32"

	dhtpb "github.com/OpenBazaar/openbazaar-go/repo/migrations/helpers/Migration020"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

// Migration020 runs an IPFS migration which migrates the IPNS records in the
// datastore.
type Migration020 struct{}

func (Migration020) Up(repoPath string, dbPassword string, testnet bool) error {
	// IPFS code errors if it detects an invalid version so we'll migrate the version first.
	if err := writeIPFSVer(repoPath, 7); err != nil {
		return fmt.Errorf("bumping IPFS version to 21: %s", err.Error())
	}

	// Open the repo
	r, err := fsrepo.Open(repoPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Load the config
	cfg, err := r.Config()
	if err != nil {
		return err
	}

	// Open our OpenBazaar db and grab the identity key
	var databaseFilePath string
	if testnet {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	db, err := sql.Open("sqlite3", databaseFilePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if dbPassword != "" {
		p := fmt.Sprintf("PRAGMA key = '%s';", dbPassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}
	stmt, err := db.Prepare("select value from config where key=?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	var identityKey []byte
	err = stmt.QueryRow("identityKey").Scan(&identityKey)
	if err != nil {
		return err
	}
	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}

	// Set our key in IPFS config
	cfg.Identity = identity

	// Migrate record(s)
	ks := r.Keystore()
	keys, err := ks.List()
	if err != nil {
		return err
	}

	dstore := r.Datastore()

	sk, err := myKey(r)
	if err != nil {
		return err
	}

	err = applyForKey(dstore, sk)
	if err != nil {
		return err
	}

	for _, keyName := range keys {
		k, err := ks.Get(keyName)
		if err != nil {
			return err
		}
		err = applyForKey(dstore, k)
		if err != nil {
			return err
		}
	}

	// Migrate OpenBazaar repover
	if err := writeRepoVer(repoPath, 21); err != nil {
		return fmt.Errorf("bumping repover to 21: %s", err.Error())
	}
	return nil
}

func (Migration020) Down(repoPath string, dbPassword string, testnet bool) error {
	// We're downgrading from version 7.
	fsrepo.RepoVersion = 7
	r, err := fsrepo.Open(repoPath)
	fsrepo.RepoVersion = 6
	if err != nil {
		return err
	}
	defer r.Close()

	// Load the config
	cfg, err := r.Config()
	if err != nil {
		return err
	}

	// Open our OpenBazaar db and grab the identity key
	var databaseFilePath string
	if testnet {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}
	db, err := sql.Open("sqlite3", databaseFilePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if dbPassword != "" {
		p := fmt.Sprintf("PRAGMA key = '%s';", dbPassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}
	stmt, err := db.Prepare("select value from config where key=?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	var identityKey []byte
	err = stmt.QueryRow("identityKey").Scan(&identityKey)
	if err != nil {
		return err
	}
	identity, err := ipfs.IdentityFromKey(identityKey)
	if err != nil {
		return err
	}

	// Set our key in IPFS config
	cfg.Identity = identity

	// Migrate the record(s)
	sk, err := myKey(r)
	if err != nil {
		return err
	}

	ks := r.Keystore()
	keys, err := ks.List()
	if err != nil {
		return err
	}

	dstore := r.Datastore()

	revertForKey(dstore, sk, sk)

	for _, keyName := range keys {
		k, err := ks.Get(keyName)
		if err != nil {
			return err
		}
		revertForKey(dstore, sk, k)
	}

	// Migrate the OpenBazaar and IPFS repo versions
	if err := writeRepoVer(repoPath, 20); err != nil {
		return fmt.Errorf("bumping repover to 20: %s", err.Error())
	}
	if err := writeIPFSVer(repoPath, 6); err != nil {
		return fmt.Errorf("bumping IPFS version to 6: %s", err.Error())
	}
	return nil
}

func myKey(r repo.Repo) (ci.PrivKey, error) {
	cfg, err := r.Config()
	if err != nil {
		return nil, err
	}

	sk, err := cfg.Identity.DecodePrivateKey("passphrase todo!")
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		return nil, err
	}
	idCfg, err := peer.IDB58Decode(cfg.Identity.PeerID)
	if err != nil {
		return nil, err
	}

	if pid != idCfg {
		return nil, fmt.Errorf(
			"private key in config does not match id: %s != %s",
			pid,
			idCfg,
		)
	}
	return sk, nil
}

func applyForKey(dstore ds.Datastore, k ci.PrivKey) error {
	id, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %s", err)
	}
	_, ipns := IPNSKeysForID(id)
	recordbytes, err := dstore.Get(dshelp.NewKeyFromBinary([]byte(ipns)))
	if err == ds.ErrNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("datastore error: %s", err)
	}

	dhtrec := new(dhtpb.Migration020RecordOldFormat)
	err = proto.Unmarshal(recordbytes, dhtrec)
	if err != nil {
		return fmt.Errorf("failed to decode DHT record: %s", err)
	}

	val := dhtrec.GetValue()
	newkey := ds.NewKey("/ipns/" + base32.RawStdEncoding.EncodeToString([]byte(id)))
	err = dstore.Put(newkey, val)
	if err != nil {
		return fmt.Errorf("failed to write new IPNS record: %s", err)
	}
	return nil
}

func revertForKey(dstore ds.Datastore, sk ci.PrivKey, k ci.PrivKey) error {
	id, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %s", err)
	}

	_, ipns := IPNSKeysForID(id)

	newkey := ds.NewKey("/ipns/" + base32.RawStdEncoding.EncodeToString([]byte(id)))
	value, err := dstore.Get(newkey)
	if err == ds.ErrNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("datastore error: %s", err)
	}

	dhtrec, err := MakePutRecord(sk, ipns, value, true)
	if err != nil {
		return fmt.Errorf("failed to create DHT record: %s", err)
	}

	data, err := proto.Marshal(dhtrec)
	if err != nil {
		return fmt.Errorf("failed to marshal DHT record: %s", err)
	}

	err = dstore.Put(dshelp.NewKeyFromBinary([]byte(ipns)), data)
	if err != nil {
		return fmt.Errorf("failed to write DHT record: %s", err)
	}
	return nil
}

// MakePutRecord creates and signs a dht record for the given key/value pair
func MakePutRecord(sk ci.PrivKey, key string, value []byte, sign bool) (*dhtpb.Migration020RecordOldFormat, error) {
	record := new(dhtpb.Migration020RecordOldFormat)

	record.Key = proto.String(key)
	record.Value = value

	pkb, err := sk.GetPublic().Bytes()
	if err != nil {
		return nil, err
	}

	pkh := u.Hash(pkb)

	record.Author = proto.String(string(pkh))
	if sign {
		blob := RecordBlobForSig(record)

		sig, err := sk.Sign(blob)
		if err != nil {
			return nil, err
		}

		record.Signature = sig
	}
	return record, nil
}

// RecordBlobForSig returns the blob protected by the record signature
func RecordBlobForSig(r *dhtpb.Migration020RecordOldFormat) []byte {
	k := []byte(r.GetKey())
	v := r.GetValue()
	a := []byte(r.GetAuthor())
	return bytes.Join([][]byte{k, v, a}, []byte{})
}

func IPNSKeysForID(id peer.ID) (name, ipns string) {
	namekey := "/pk/" + string(id)
	ipnskey := "/ipns/" + string(id)

	return namekey, ipnskey
}
