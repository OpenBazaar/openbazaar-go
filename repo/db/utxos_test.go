package db

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"sync"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
	"github.com/OpenBazaar/wallet-interface"
)

func buildNewUnspentTransactionOutputStore() (repo.UnspentTransactionOutputStore, func(), error) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		return nil, nil, err
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		return nil, nil, err
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		return nil, nil, err
	}
	return NewUnspentTransactionStore(database, new(sync.Mutex), wallet.Bitcoin), appSchema.DestroySchemaDirectories, nil
}

func newPopulatedUtxoStore() (repo.UnspentTransactionOutputStore, wallet.Utxo, func(), error) {
	var utxoDB, teardown, err = buildNewUnspentTransactionOutputStore()
	utxo := factory.NewUtxo()
	if err != nil {
		return nil, utxo, teardown, err
	}
	return utxoDB, utxo, teardown, utxoDB.Put(utxo)
}

func TestUtxoPut(t *testing.T) {
	var utxoDB, utxo, teardown, err = newPopulatedUtxoStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	stmt, _ := utxoDB.PrepareQuery("select outpoint, value, height, scriptPubKey from utxos where outpoint=?")
	defer stmt.Close()

	var outpoint string
	var value string
	var height int
	var scriptPubkey string
	o := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	err = stmt.QueryRow(o).Scan(&outpoint, &value, &height, &scriptPubkey)
	if err != nil {
		t.Error(err)
	}
	if outpoint != o {
		t.Error("Utxo db returned wrong outpoint")
	}
	if value != utxo.Value {
		t.Error("Utxo db returned wrong value")
	}
	if height != int(utxo.AtHeight) {
		t.Error("Utxo db returned wrong height")
	}
	if scriptPubkey != hex.EncodeToString(utxo.ScriptPubkey) {
		t.Error("Utxo db returned wrong scriptPubKey")
	}
}

func TestUtxoGetAll(t *testing.T) {
	var utxoDB, utxo, teardown, err = newPopulatedUtxoStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	utxos, err := utxoDB.GetAll()
	if err != nil {
		t.Error(err)
	}
	if utxos[0].Op.Hash.String() != utxo.Op.Hash.String() {
		t.Error("Utxo db returned wrong outpoint hash")
	}
	if utxos[0].Op.Index != utxo.Op.Index {
		t.Error("Utxo db returned wrong outpoint index")
	}
	if utxos[0].Value != utxo.Value {
		t.Error("Utxo db returned wrong value")
	}
	if utxos[0].AtHeight != utxo.AtHeight {
		t.Error("Utxo db returned wrong height")
	}
	if !bytes.Equal(utxos[0].ScriptPubkey, utxo.ScriptPubkey) {
		t.Error("Utxo db returned wrong scriptPubKey")
	}
}

func TestSetWatchOnlyUtxo(t *testing.T) {
	var utxoDB, utxo, teardown, err = newPopulatedUtxoStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	err = utxoDB.SetWatchOnly(utxo)
	if err != nil {
		t.Error(err)
	}
	stmt, _ := utxoDB.PrepareQuery("select watchOnly from utxos where outpoint=?")
	defer stmt.Close()

	var watchOnlyInt int
	o := utxo.Op.Hash.String() + ":" + strconv.Itoa(int(utxo.Op.Index))
	err = stmt.QueryRow(o).Scan(&watchOnlyInt)
	if err != nil {
		t.Error(err)
	}
	if watchOnlyInt != 1 {
		t.Error("Utxo freeze failed")
	}

}

func TestDeleteUtxo(t *testing.T) {
	var utxoDB, utxo, teardown, err = newPopulatedUtxoStore()
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()
	err = utxoDB.Delete(utxo)
	if err != nil {
		t.Error(err)
	}
	utxos, err := utxoDB.GetAll()
	if err != nil {
		t.Error(err)
	}
	if len(utxos) != 0 {
		t.Error("Utxo db delete failed")
	}
}
