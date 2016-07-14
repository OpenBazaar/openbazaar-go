package db

import (
	"database/sql"
	"sync"
	"strconv"
	"encoding/hex"
	"strings"
	"github.com/OpenBazaar/spvwallet"
	"github.com/btcsuite/btcd/wire"
)

type StxoDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (s *StxoDB) Put(stxo spvwallet.Stxo) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	tx, _ := s.db.Begin()
	stmt, err := tx.Prepare("insert or replace into stxos(outpoint, value, height, scriptPubKey, spendHeight, spendTxid) values(?,?,?,?,?,?)")
	defer stmt.Close()
	if err != nil {
		tx.Rollback()
		return err
	}
	outpoint := stxo.Utxo.Op.Hash.String() + ":" + strconv.Itoa(int(stxo.Utxo.Op.Index))
	_, err = stmt.Exec(outpoint, int(stxo.Utxo.Value), int(stxo.Utxo.AtHeight), hex.EncodeToString(stxo.Utxo.ScriptPubkey), int(stxo.SpendHeight), stxo.SpendTxid.String())
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (s *StxoDB) GetAll() ([]spvwallet.Stxo, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	var ret []spvwallet.Stxo
	stm := "select outpoint, value, height, scriptPubKey, spendHeight, spendTxid from stxos"
	rows, err := s.db.Query(stm)
	defer rows.Close()
	if err != nil {
		return ret, err
	}
	for rows.Next() {
		var outpoint string
		var value int
		var height int
		var scriptPubKey string
		var spendHeight int
		var spendTxid string
		if err := rows.Scan(&outpoint, &value, &height, &scriptPubKey, &spendHeight, &spendTxid); err != nil {
			continue
		}
		s := strings.Split(outpoint, ":")
		if err != nil {
			continue
		}
		shaHash, err := wire.NewShaHashFromStr(s[0])
		if err != nil {
			continue
		}
		index, err := strconv.Atoi(s[1])
		if err != nil {
			continue
		}
		scriptBytes, err := hex.DecodeString(scriptPubKey)
		if err != nil {
			continue
		}
		spentHash, err := wire.NewShaHashFromStr(spendTxid)
		if err != nil {
			continue
		}
		utxo := spvwallet.Utxo{
			Op: *wire.NewOutPoint(shaHash, uint32(index)),
			AtHeight: int32(height),
			Value: int64(value),
			ScriptPubkey: scriptBytes,
		}
		ret = append(ret, spvwallet.Stxo{
			Utxo: utxo,
			SpendHeight: int32(spendHeight),
			SpendTxid: *spentHash,
		})
	}
	return ret, nil
}

func (s *StxoDB) Delete(stxo spvwallet.Stxo) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	outpoint := stxo.Utxo.Op.Hash.String() + ":" + strconv.Itoa(int(stxo.Utxo.Op.Index))
	_, err := s.db.Exec("delete from stxos where outpoint=?", outpoint)
	if err != nil {
		return err
	}
	return nil
}