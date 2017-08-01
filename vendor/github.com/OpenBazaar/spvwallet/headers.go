package spvwallet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"path"
	"sort"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/cevaris/ordered_map"
	"strings"
)

const (
	MAX_HEADERS = 2000
	CACHE_SIZE  = 100
)

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

	// Grab a header given hash
	GetHeader(hash chainhash.Hash) (StoredHeader, error)

	// Retrieve the best header from the database
	GetBestHeader() (StoredHeader, error)

	// Get the height of chain
	Height() (uint32, error)

	// Cleanly close the db
	Close()

	// Print all headers
	Print(io.Writer)
}

type StoredHeader struct {
	header    wire.BlockHeader
	height    uint32
	totalWork *big.Int
}

// HeaderDB implements Headers using bolt DB
type HeaderDB struct {
	lock      *sync.Mutex
	db        *bolt.DB
	filePath  string
	bestCache *StoredHeader
	cache     *HeaderCache
}

var (
	BKTHeaders  = []byte("Headers")
	BKTChainTip = []byte("ChainTip")
	KEYChainTip = []byte("ChainTip")
)

func NewHeaderDB(filePath string) *HeaderDB {
	if !strings.Contains(filePath, ".bin") {
		filePath = path.Join(filePath, "headers.bin")
	}
	h := new(HeaderDB)
	db, _ := bolt.Open(filePath, 0644, &bolt.Options{InitialMmapSize: 5000000})
	h.db = db
	h.lock = new(sync.Mutex)
	h.filePath = filePath
	h.cache = &HeaderCache{ordered_map.NewOrderedMap(), sync.Mutex{}, CACHE_SIZE}

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

	h.initializeCache()
	return h
}

func (h *HeaderDB) Put(sh StoredHeader, newBestHeader bool) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.cache.Set(sh)
	if newBestHeader {
		h.bestCache = &sh
	}
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
	hash := header.PrevBlock
	return h.GetHeader(hash)
}

func (h *HeaderDB) GetHeader(hash chainhash.Hash) (sh StoredHeader, err error) {
	h.lock.Lock()
	defer h.lock.Unlock()
	cachedHeader, cerr := h.cache.Get(hash)
	if cerr == nil {
		return cachedHeader, nil
	}
	err = h.db.View(func(btx *bolt.Tx) error {
		hdrs := btx.Bucket(BKTHeaders)
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
	if h.bestCache != nil {
		best := h.bestCache
		return *best, nil
	}
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
	if h.bestCache != nil {
		return h.bestCache.height, nil
	}
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

func (h *HeaderDB) Print(w io.Writer) {
	h.lock.Lock()
	defer h.lock.Unlock()
	m := make(map[float64][]string)
	h.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		bkt := tx.Bucket(BKTHeaders)
		bkt.ForEach(func(k, v []byte) error {
			sh, _ := deserializeHeader(v)
			h := float64(sh.height)
			_, ok := m[h]
			if ok {
				for {
					h += .1
					_, ok := m[h]
					if !ok {
						break
					}
				}
			}
			m[h] = []string{sh.header.BlockHash().String(), sh.header.PrevBlock.String()}
			return nil
		})

		return nil
	})
	var keys []float64
	for k := range m {
		keys = append(keys, float64(k))
	}
	sort.Float64s(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "Height: %.1f, Hash: %s, Parent: %s\n", k, m[k][0], m[k][1])
	}
}

func (h *HeaderDB) initializeCache() {
	best, err := h.GetBestHeader()
	if err != nil {
		return
	}
	h.bestCache = &best
	headers := []StoredHeader{best}
	for i := 0; i < 99; i++ {
		sh, err := h.GetPreviousHeader(best.header)
		if err != nil {
			break
		}
		headers = append(headers, sh)
	}
	for i := len(headers) - 1; i >= 0; i-- {
		h.cache.Set(headers[i])
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

type HeaderCache struct {
	headers *ordered_map.OrderedMap
	sync.Mutex
	cacheSize int
}

func (h *HeaderCache) pop() {
	iter := h.headers.IterFunc()
	k, ok := iter()
	if ok {
		h.headers.Delete(k.Key)
	}
}

func (h *HeaderCache) Set(sh StoredHeader) {
	h.Lock()
	defer h.Unlock()
	if h.headers.Len() > h.cacheSize {
		h.pop()
	}
	hash := sh.header.BlockHash()
	h.headers.Set(hash.String(), sh)
}

func (h *HeaderCache) Get(hash chainhash.Hash) (StoredHeader, error) {
	sh, ok := h.headers.Get(hash.String())
	if !ok {
		return StoredHeader{}, errors.New("Not found")
	}
	return sh.(StoredHeader), nil
}
