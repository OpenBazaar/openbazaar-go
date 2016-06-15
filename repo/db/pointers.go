package db

import (
	"time"
	"database/sql"
	"sync"
	"github.com/OpenBazaar/openbazaar-go/ipfs"
	keys "github.com/ipfs/go-ipfs/blocks/key"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
)

type PointersDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (p *PointersDB) Put(pointer ipfs.Pointer) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("insert into pointers(peerID, key, address, purpose, timestamp) values(?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(pointer.Value.ID.Pretty(), pointer.Key.B58String(), pointer.Value.Addrs[0].String(), pointer.Purpose, int(time.Now().Unix()))
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (p *PointersDB) Delete(id peer.ID) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	_, err := p.db.Exec("delete from pointers where peerID=?", id.Pretty())
	if err != nil {
		log.Error(err)
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
		log.Error(err)
		return nil, err
	}
	var ret []ipfs.Pointer
	for rows.Next() {
		var peerID string
		var key string
		var address string
		var purpose int
		var timestamp int
		if err := rows.Scan(&peerID, &key, &address, &purpose, &timestamp); err != nil {
			log.Error(err)
		}
		maAddr, err := ma.NewMultiaddr(address)
		if err != nil {
			return ret, err
		}
		pid, err := peer.IDB58Decode(peerID)
		if err != nil {
			return ret, err
		}
		pointer := ipfs.Pointer{
			Key: keys.B58KeyDecode(key),
			Value: peer.PeerInfo{
				ID: pid,
				Addrs: []ma.Multiaddr{maAddr},
			},
			Purpose: ipfs.Purpose(purpose),
			Timestamp: time.Unix(int64(timestamp), 0),
		}
		ret = append(ret, pointer)
	}
	return ret, nil
}

