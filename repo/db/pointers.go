package db

import (
	"database/sql"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	ps "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"strconv"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

type PointersDB struct {
	modelStore
}

func NewPointerStore(db *sql.DB, lock *sync.Mutex) repo.PointerStore {
	return &PointersDB{modelStore{db, lock}}
}

func (p *PointersDB) Put(pointer ipfs.Pointer) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into pointers(pointerID, key, address, cancelID, purpose, timestamp) values(?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	var cancelID string
	if pointer.CancelID != nil {
		cancelID = pointer.CancelID.Pretty()
	}
	_, err = stmt.Exec(pointer.Value.ID.Pretty(), pointer.Cid.String(), pointer.Value.Addrs[0].String(), cancelID, pointer.Purpose, int(time.Now().Unix()))
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (p *PointersDB) Delete(id peer.ID) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, err := p.db.Exec("delete from pointers where pointerID=?", id.Pretty())
	if err != nil {
		return err
	}
	return nil
}

func (p *PointersDB) DeleteAll(purpose ipfs.Purpose) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, err := p.db.Exec("delete from pointers where purpose=?", purpose)
	if err != nil {
		return err
	}
	return nil
}

func (p *PointersDB) GetAll() ([]ipfs.Pointer, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	stm := "select * from pointers"
	rows, err := p.db.Query(stm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ret []ipfs.Pointer
	for rows.Next() {
		var pointerID string
		var key string
		var address string
		var purpose int
		var timestamp int
		var cancelID string
		if err := rows.Scan(&pointerID, &key, &address, &cancelID, &purpose, &timestamp); err != nil {
			return ret, err
		}
		maAddr, err := ma.NewMultiaddr(address)
		if err != nil {
			return ret, err
		}
		pid, err := peer.IDB58Decode(pointerID)
		if err != nil {
			return ret, err
		}
		k, err := cid.Decode(key)
		if err != nil {
			return ret, err
		}
		var canID *peer.ID
		if cancelID != "" {
			c, err := peer.IDB58Decode(cancelID)
			if err != nil {
				return ret, err
			}
			canID = &c
		}
		pointer := ipfs.Pointer{
			Cid: k,
			Value: ps.PeerInfo{
				ID:    pid,
				Addrs: []ma.Multiaddr{maAddr},
			},
			CancelID:  canID,
			Purpose:   ipfs.Purpose(purpose),
			Timestamp: time.Unix(int64(timestamp), 0),
		}
		ret = append(ret, pointer)
	}
	return ret, nil
}

func (p *PointersDB) GetByPurpose(purpose ipfs.Purpose) ([]ipfs.Pointer, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	stm := "select * from pointers where purpose=" + strconv.Itoa(int(purpose))
	rows, err := p.db.Query(stm)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ret []ipfs.Pointer
	for rows.Next() {
		var pointerID string
		var key string
		var address string
		var purpose int
		var timestamp int
		var cancelID string
		if err := rows.Scan(&pointerID, &key, &address, &cancelID, &purpose, &timestamp); err != nil {
			return ret, err
		}
		maAddr, err := ma.NewMultiaddr(address)
		if err != nil {
			return ret, err
		}
		pid, err := peer.IDB58Decode(pointerID)
		if err != nil {
			return ret, err
		}
		k, err := cid.Decode(key)
		if err != nil {
			return ret, err
		}
		var canID *peer.ID
		if cancelID != "" {
			c, err := peer.IDB58Decode(cancelID)
			if err != nil {
				return ret, err
			}
			canID = &c
		}
		pointer := ipfs.Pointer{
			Cid: k,
			Value: ps.PeerInfo{
				ID:    pid,
				Addrs: []ma.Multiaddr{maAddr},
			},
			CancelID:  canID,
			Purpose:   ipfs.Purpose(purpose),
			Timestamp: time.Unix(int64(timestamp), 0),
		}
		ret = append(ret, pointer)
	}
	return ret, nil
}

func (p *PointersDB) Get(id peer.ID) (ipfs.Pointer, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	stm := "select * from pointers where pointerID=?"
	row := p.db.QueryRow(stm, id.Pretty())
	var pointer ipfs.Pointer

	var pointerID string
	var key string
	var address string
	var purpose int
	var timestamp int
	var cancelID string
	if err := row.Scan(&pointerID, &key, &address, &cancelID, &purpose, &timestamp); err != nil {
		return pointer, err
	}
	maAddr, err := ma.NewMultiaddr(address)
	if err != nil {
		return pointer, err
	}
	pid, err := peer.IDB58Decode(pointerID)
	if err != nil {
		return pointer, err
	}
	k, err := cid.Decode(key)
	if err != nil {
		return pointer, err
	}
	var canID *peer.ID
	if cancelID != "" {
		c, err := peer.IDB58Decode(cancelID)
		if err != nil {
			return pointer, err
		}
		canID = &c
	}
	pointer = ipfs.Pointer{
		Cid: k,
		Value: ps.PeerInfo{
			ID:    pid,
			Addrs: []ma.Multiaddr{maAddr},
		},
		CancelID:  canID,
		Purpose:   ipfs.Purpose(purpose),
		Timestamp: time.Unix(int64(timestamp), 0),
	}
	return pointer, nil
}
