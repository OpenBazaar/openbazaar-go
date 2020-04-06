package db

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/wallet-interface"
	"github.com/btcsuite/btcd/btcec"
)

type KeysDB struct {
	modelStore
	coinType wallet.CoinType
}

func NewKeyStore(db *sql.DB, lock *sync.Mutex, coinType wallet.CoinType) repo.KeyStore {
	return &KeysDB{modelStore{db, lock}, coinType}
}

func (k *KeysDB) Put(scriptAddress []byte, keyPath wallet.KeyPath) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	stmt, err := k.PrepareQuery("insert into keys(coin, scriptAddress, purpose, keyIndex, used) values(?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("prepare key sql: %s", err.Error())
	}
	defer stmt.Close()

	_, err = stmt.Exec(k.coinType.CurrencyCode(), hex.EncodeToString(scriptAddress), int(keyPath.Purpose), keyPath.Index, 0)
	if err != nil {
		return fmt.Errorf("commit key: %s", err.Error())
	}
	return nil
}

func (k *KeysDB) ImportKey(scriptAddress []byte, key *btcec.PrivateKey) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	stmt, err := k.PrepareQuery("insert into keys(coin, scriptAddress, purpose, used, key) values(?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("prepare key sql: %s", err.Error())
	}
	defer stmt.Close()
	_, err = stmt.Exec(k.coinType.CurrencyCode(), hex.EncodeToString(scriptAddress), -1, 0, hex.EncodeToString(key.Serialize()))
	if err != nil {
		return fmt.Errorf("commit key: %s", err.Error())
	}
	return nil
}

func (k *KeysDB) MarkKeyAsUsed(scriptAddress []byte) error {
	k.lock.Lock()
	defer k.lock.Unlock()
	stmt, err := k.PrepareQuery("update keys set used=1 where scriptAddress=? and coin=?")
	if err != nil {
		return fmt.Errorf("prepare key sql: %s", err.Error())
	}
	defer stmt.Close()

	_, err = stmt.Exec(hex.EncodeToString(scriptAddress), k.coinType.CurrencyCode())
	if err != nil {
		return fmt.Errorf("commit key update: %s", err.Error())
	}
	return nil
}

func (k *KeysDB) GetLastKeyIndex(purpose wallet.KeyPurpose) (int, bool, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stm := "select keyIndex, used from keys where purpose=? and coin=? order by rowid desc limit 1"
	var index int
	var usedInt int
	err := k.db.QueryRow(stm, strconv.Itoa(int(purpose)), k.coinType.CurrencyCode()).Scan(&index, &usedInt)
	if err != nil {
		return 0, false, err
	}
	var used bool
	if usedInt == 0 {
		used = false
	} else {
		used = true
	}
	return index, used, nil
}

func (k *KeysDB) GetPathForKey(scriptAddress []byte) (wallet.KeyPath, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stmt, err := k.db.Prepare("select purpose, keyIndex from keys where scriptAddress=? and coin=?")
	if err != nil {
		return wallet.KeyPath{}, err
	}
	defer stmt.Close()
	var purpose int
	var index int
	err = stmt.QueryRow(hex.EncodeToString(scriptAddress), k.coinType.CurrencyCode()).Scan(&purpose, &index)
	if err != nil {
		return wallet.KeyPath{}, errors.New("key not found")
	}
	p := wallet.KeyPath{
		Purpose: wallet.KeyPurpose(purpose),
		Index:   index,
	}
	return p, nil
}

func (k *KeysDB) GetKey(scriptAddress []byte) (*btcec.PrivateKey, error) {
	k.lock.Lock()
	defer k.lock.Unlock()

	stmt, err := k.db.Prepare("select key from keys where scriptAddress=? and purpose=-1 and coin=?")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	var keyHex string
	err = stmt.QueryRow(hex.EncodeToString(scriptAddress), k.coinType.CurrencyCode()).Scan(&keyHex)
	if err != nil {
		return nil, errors.New("key not found")
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}
	key, _ := btcec.PrivKeyFromBytes(btcec.S256(), keyBytes)
	return key, nil
}

func (k *KeysDB) GetImported() ([]*btcec.PrivateKey, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var ret []*btcec.PrivateKey
	stm := "select key from keys where purpose=-1 and coin=?"
	rows, err := k.db.Query(stm, k.coinType.CurrencyCode())
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var keyHex []byte
		err = rows.Scan(&keyHex)
		if err != nil {
			return ret, err
		}
		keyBytes, err := hex.DecodeString(string(keyHex))
		if err != nil {
			return ret, err
		}
		priv, _ := btcec.PrivKeyFromBytes(btcec.S256(), keyBytes)
		ret = append(ret, priv)
	}
	return ret, nil
}

func (k *KeysDB) GetUnused(purpose wallet.KeyPurpose) ([]int, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var ret []int
	stm := "select keyIndex from keys where purpose=? and coin=? and used=0 order by rowid asc"
	rows, err := k.db.Query(stm, strconv.Itoa(int(purpose)), k.coinType.CurrencyCode())
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var index int
		err = rows.Scan(&index)
		if err != nil {
			return ret, err
		}
		ret = append(ret, index)
	}
	return ret, nil
}

func (k *KeysDB) GetAll() ([]wallet.KeyPath, error) {
	k.lock.Lock()
	defer k.lock.Unlock()
	var ret []wallet.KeyPath
	stm := "select purpose, keyIndex from keys where coin=?"
	rows, err := k.db.Query(stm, k.coinType.CurrencyCode())
	if err != nil {
		return ret, err
	}
	defer rows.Close()
	for rows.Next() {
		var purpose int
		var index int
		if err := rows.Scan(&purpose, &index); err != nil {
			continue
		}
		p := wallet.KeyPath{
			Purpose: wallet.KeyPurpose(purpose),
			Index:   index,
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func (k *KeysDB) GetLookaheadWindows() map[wallet.KeyPurpose]int {
	k.lock.Lock()
	defer k.lock.Unlock()
	windows := make(map[wallet.KeyPurpose]int)
	for i := 0; i < 2; i++ {
		stm := "select used from keys where purpose=? and coin=? order by rowid desc"
		rows, err := k.db.Query(stm, strconv.Itoa(i), k.coinType.CurrencyCode())
		if err != nil {
			continue
		}
		var unusedCount int
		for rows.Next() {
			var used int
			if err := rows.Scan(&used); err != nil {
				used = 1
			}
			if used == 0 {
				unusedCount++
			} else {
				break
			}
		}
		purpose := wallet.KeyPurpose(i)
		windows[purpose] = unusedCount
		rows.Close()
	}
	return windows
}
