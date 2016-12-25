package spvwallet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/btcsuite/btcd/wire"
	"math/big"
	"path"
	"sort"
	"sync"
)

const MAX_HEADERS = 2000

// Database interface for storing block headers
type Headers interface {
	// Put a block header to the database
	// Total work and height are required to be calculated prior to insertion
	// If this is the new best header, the chain tip should also be updated
	Put(header StoredHeader, newBestHeader bool) error

	// Delete all headers after the MAX_HEADERS most recent
	Prune() error

	// Returns all information about the previous header
	GetPreviousHeader(header wire.BlockHeader) (StoredHeader, error)

	// Retreive the best header from the database
	GetBestHeader() (StoredHeader, error)

	// Get the height of chain
	Height() (uint32, error)

	// Cleanly close the db
	Close()

	// Print all headers
	Print()
}

type StoredHeader struct {
	header    wire.BlockHeader
	height    uint32
	totalWork *big.Int
}

// HeaderDB implements Headers using bolt DB
type HeaderDB struct {
	lock     *sync.Mutex
	db       *bolt.DB
	filePath string
}

var (
	BKTHeaders  = []byte("Headers")
	BKTChainTip = []byte("ChainTip")
	KEYChainTip = []byte("ChainTip")
)

func NewHeaderDB(filePath string) *HeaderDB {
	h := new(HeaderDB)
	db, _ := bolt.Open(path.Join(filePath, "headers.bin"), 0644, &bolt.Options{InitialMmapSize: 5000000})
	h.db = db
	h.lock = new(sync.Mutex)
	h.filePath = filePath

	db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists(BKTHeaders)
		if err != nil {
			return err
		}
		_, err = btx.CreateBucketIfNotExists(BKTChainTip)
		if err != nil {
			return err
		}
		return nil
	})
	return h
}

func (h *HeaderDB) Put(sh StoredHeader, newBestHeader bool) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	return h.db.Update(func(btx *bolt.Tx) error {
		hdrs := btx.Bucket(BKTHeaders)
		ser, err := serializeHeader(sh)
		if err != nil {
			return err
		}
		hash := sh.header.BlockHash()
		err = hdrs.Put(hash.CloneBytes(), ser)
		if err != nil {
			return err
		}
		if newBestHeader {
			tip := btx.Bucket(BKTChainTip)
			err = tip.Put(KEYChainTip, ser)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (h *HeaderDB) Prune() error {
	h.lock.Lock()
	defer h.lock.Unlock()
	return h.db.Update(func(btx *bolt.Tx) error {
		hdrs := btx.Bucket(BKTHeaders)
		numHeaders := hdrs.Stats().KeyN
		tip := btx.Bucket(BKTChainTip)
		b := tip.Get(KEYChainTip)
		if b == nil {
			return errors.New("ChainTip not set")
		}
		sh, err := deserializeHeader(b)
		if err != nil {
			return err
		}
		height := sh.height
		if numHeaders > MAX_HEADERS {
			var toDelete [][]byte
			pruneHeight := height - 2000
			err := hdrs.ForEach(func(k, v []byte) error {
				sh, err := deserializeHeader(v)
				if err != nil {
					return err
				}
				if sh.height <= pruneHeight {
					toDelete = append(toDelete, k)
				}
				return nil
			})
			if err != nil {
				return err
			}
			for _, k := range toDelete {
				err := hdrs.Delete(k)
				if err != nil {
					return err
				}
			}

		}
		return nil
	})
}

func (h *HeaderDB) GetPreviousHeader(header wire.BlockHeader) (sh StoredHeader, err error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	err = h.db.View(func(btx *bolt.Tx) error {
		hdrs := btx.Bucket(BKTHeaders)
		hash := header.PrevBlock
		b := hdrs.Get(hash.CloneBytes())
		if b == nil {
			return errors.New("Header does not exist in database")
		}
		sh, err = deserializeHeader(b)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return sh, err
	}
	return sh, nil
}

func (h *HeaderDB) GetBestHeader() (sh StoredHeader, err error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	err = h.db.View(func(btx *bolt.Tx) error {
		tip := btx.Bucket(BKTChainTip)
		b := tip.Get(KEYChainTip)
		if b == nil {
			return errors.New("ChainTip not set")
		}
		sh, err = deserializeHeader(b)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return sh, err
	}
	return sh, nil
}

func (h *HeaderDB) Height() (uint32, error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	var height uint32
	err := h.db.View(func(btx *bolt.Tx) error {
		tip := btx.Bucket(BKTChainTip)
		sh, err := deserializeHeader(tip.Get(KEYChainTip))
		if err != nil {
			return err
		}
		height = sh.height
		return nil
	})
	if err != nil {
		return height, err
	}
	return height, nil
}

func (h *HeaderDB) Print() {
	h.lock.Lock()
	defer h.lock.Unlock()
	m := make(map[uint32]string)
	h.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		bkt := tx.Bucket(BKTHeaders)
		bkt.ForEach(func(k, v []byte) error {
			sh, _ := deserializeHeader(v)
			m[sh.height] = sh.header.BlockHash().String()
			return nil
		})

		return nil
	})
	var keys []int
	for k := range m {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Println(k, m[uint32(k)])
	}
}

func (h *HeaderDB) Close() {
	h.lock.Lock()
}

/*----- header serialization ------- */
/* byteLength   desc          at offset
   80	       header	           0
    4	       height             80
   32	       total work         84
*/
func serializeHeader(sh StoredHeader) ([]byte, error) {
	var buf bytes.Buffer
	err := sh.header.Serialize(&buf)
	if err != nil {
		return nil, err
	}
	err = binary.Write(&buf, binary.BigEndian, sh.height)
	if err != nil {
		return nil, err
	}
	biBytes := sh.totalWork.Bytes()
	pad := make([]byte, 32-len(biBytes))
	serializedBI := append(pad, biBytes...)
	buf.Write(serializedBI)
	return buf.Bytes(), nil
}

func deserializeHeader(b []byte) (sh StoredHeader, err error) {
	r := bytes.NewReader(b)
	hdr := new(wire.BlockHeader)
	err = hdr.Deserialize(r)
	if err != nil {
		return sh, err
	}
	var height uint32
	err = binary.Read(r, binary.BigEndian, &height)
	if err != nil {
		return sh, err
	}
	biBytes := make([]byte, 32)
	_, err = r.Read(biBytes)
	if err != nil {
		return sh, err
	}
	bi := new(big.Int)
	bi.SetBytes(biBytes)
	sh = StoredHeader{
		header:    *hdr,
		height:    height,
		totalWork: bi,
	}
	return sh, nil
}
