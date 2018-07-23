package migrations

import (
	"database/sql"
	"fmt"
	"github.com/OpenBazaar/wallet-interface"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

var WalletCoinType wallet.CoinType

type Migration011 struct{}

func (Migration011) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	var (
		databaseFilePath    string
		repoVersionFilePath = path.Join(repoPath, "repover")
	)
	if testnetEnabled {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}

	const (
		alterKeysSQL                 = "alter table keys add coin text;"
		createKeysIndexSQL           = "create index index_keys on keys (coin);"
		alterUTXOSQL                 = "alter table utxos add coin text;"
		createUTXOIndexSQL           = "create index index_utxos on utxos (coin);"
		alterSTXOSQL                 = "alter table stxos add coin text;"
		createSTXOIndexSQL           = "create index index_stxos on stxos (coin);"
		alterTXNSSQL                 = "alter table txns add coin text;"
		createTXNSIndexSQL           = "create index index_txns on txns (coin);"
		alterWatchedScriptsSQL       = "alter table watchedscripts add coin text;"
		createWatchedScriptsIndexSQL = "create index index_watchedscripts on watchedscripts (coin);"
	)

	db, err := sql.Open("sqlite3", databaseFilePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if databasePassword != "" {
		p := fmt.Sprintf("pragma key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}

	migration := strings.Join([]string{
		alterKeysSQL,
		alterUTXOSQL,
		alterSTXOSQL,
		alterTXNSSQL,
		alterWatchedScriptsSQL,
		createKeysIndexSQL,
		createUTXOIndexSQL,
		createSTXOIndexSQL,
		createTXNSIndexSQL,
		createWatchedScriptsIndexSQL,
	}, " ")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(migration); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	_, err = db.Exec("update keys set coin = ?", WalletCoinType.CurrencyCode())
	if err != nil {
		return err
	}
	_, err = db.Exec("update utxos set coin = ?", WalletCoinType.CurrencyCode())
	if err != nil {
		return err
	}
	_, err = db.Exec("update stxos set coin = ?", WalletCoinType.CurrencyCode())
	if err != nil {
		return err
	}
	_, err = db.Exec("update txns set coin = ?", WalletCoinType.CurrencyCode())
	if err != nil {
		return err
	}
	_, err = db.Exec("update watchedscripts set coin = ?", WalletCoinType.CurrencyCode())
	if err != nil {
		return err
	}

	// Bump schema version
	err = ioutil.WriteFile(repoVersionFilePath, []byte("10"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (Migration011) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	var (
		databaseFilePath    string
		repoVersionFilePath = path.Join(repoPath, "repover")
	)
	if testnetEnabled {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}

	db, err := sql.Open("sqlite3", databaseFilePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if databasePassword != "" {
		p := fmt.Sprintf("pragma key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}

	const (
		alterKeysSQL               = "alter table keys rename to keys_old;"
		createKeysSQL              = "create table keys (scriptAddress text primary key not null, purpose integer, keyIndex integer, used integer, key text);"
		insertKeysSQL              = "insert into keys select scriptAddress, purpose, keyIndex, used, key from keys_old;"
		dropKeysTableSQL           = "drop table keys_old;"
		alterUtxosSQL              = "alter table utxos rename to utxos_old;"
		createUtxosSQL             = "create table utxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer);"
		insertUtxosSQL             = "insert into utxos select outpoint, value, height, scriptPubKey, watchOnly from utxos_old;"
		dropUtxosTableSQL          = "drop table utxos_old;"
		alterStxosSQL              = "alter table stxos rename to stxos_old;"
		createStxosSQL             = "create table stxos (outpoint text primary key not null, value integer, height integer, scriptPubKey text, watchOnly integer, spendHeight integer, spendTxid text);"
		insertStxosSQL             = "insert into stxos select outpoint, value, height, scriptPubKey, watchOnly, spendHeight, spendTxid from stxos_old;"
		dropStxosTableSQL          = "drop table stxos_old;"
		alterTxnsSQL               = "alter table txns rename to txns_old;"
		createTxnsSQL              = "create table txns (txid text primary key not null, value integer, height integer, timestamp integer, watchOnly integer, tx blob);"
		insertTxnsSQL              = "insert into txns select txid, value, height, timestamp, watchOnly, tx from txns_old;"
		dropTxnsTableSQL           = "drop table txns_old;"
		alterWatchedScriptsSQL     = "alter table watchedscripts rename to watchedscripts_old;"
		createWatchedScriptSQL     = "create table watchedscripts (scriptPubKey text primary key not null);"
		insertWatchedScriptsSQL    = "insert into watchedscripts select scriptPubKey from watchedscripts_old;"
		dropWatchedScriptsTableSQL = "drop table watchedscripts_old;"
	)

	migration := strings.Join([]string{
		alterKeysSQL,
		createKeysSQL,
		insertKeysSQL,
		dropKeysTableSQL,
		alterUtxosSQL,
		createUtxosSQL,
		insertUtxosSQL,
		dropUtxosTableSQL,
		alterStxosSQL,
		createStxosSQL,
		insertStxosSQL,
		dropStxosTableSQL,
		alterTxnsSQL,
		createTxnsSQL,
		insertTxnsSQL,
		dropTxnsTableSQL,
		alterWatchedScriptsSQL,
		createWatchedScriptSQL,
		insertWatchedScriptsSQL,
		dropWatchedScriptsTableSQL,
	}, " ")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(migration); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	// Revert schema version
	err = ioutil.WriteFile(repoVersionFilePath, []byte("9"), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
