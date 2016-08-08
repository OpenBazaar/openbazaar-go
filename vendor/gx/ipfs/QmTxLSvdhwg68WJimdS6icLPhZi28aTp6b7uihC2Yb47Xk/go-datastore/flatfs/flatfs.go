// Package flatfs is a Datastore implementation that stores all
// objects in a two-level directory structure in the local file
// system, regardless of the hierarchy of the keys.
package flatfs

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
	"gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore/query"
	"gx/ipfs/QmaeRR9SpXumU5tYLRkq6x6pfMe8qKzxn4ujBpsTJ2zQG7/go-os-rename"

	logging "gx/ipfs/QmNQynaz7qfriSUJkiEZUrm2Wen1u3Kj9goZzWtrPyu7XR/go-log"
)

var log = logging.Logger("flatfs")

const (
	extension    = ".data"
	maxPrefixLen = 16
)

var (
	ErrBadPrefixLen = errors.New("bad prefix length")
)

type Datastore struct {
	path string
	// length of the dir splay prefix
	prefixLen int

	// sychronize all writes and directory changes for added safety
	sync bool
}

var _ datastore.Datastore = (*Datastore)(nil)

func New(path string, prefixLen int, sync bool) (*Datastore, error) {
	if prefixLen <= 0 || prefixLen > maxPrefixLen {
		return nil, ErrBadPrefixLen
	}
	fs := &Datastore{
		path:      path,
		prefixLen: prefixLen,
		sync:      sync,
	}
	return fs, nil
}

var padding = strings.Repeat("_", maxPrefixLen)

func (fs *Datastore) encode(key datastore.Key) (dir, file string) {
	noslash := key.String()[1:]
	prefix := (noslash + padding)[:fs.prefixLen]
	dir = path.Join(fs.path, prefix)
	file = path.Join(dir, noslash+extension)
	return dir, file
}

func (fs *Datastore) decode(file string) (key datastore.Key, ok bool) {
	if path.Ext(file) != extension {
		return datastore.Key{}, false
	}
	name := file[:len(file)-len(extension)]
	return datastore.NewKey(name), true
}

func (fs *Datastore) makePrefixDir(dir string) error {
	if err := fs.makePrefixDirNoSync(dir); err != nil {
		return err
	}

	// In theory, if we create a new prefix dir and add a file to
	// it, the creation of the prefix dir itself might not be
	// durable yet. Sync the root dir after a successful mkdir of
	// a prefix dir, just to be paranoid.
	if fs.sync {
		if err := syncDir(fs.path); err != nil {
			return err
		}
	}
	return nil
}

func (fs *Datastore) makePrefixDirNoSync(dir string) error {
	if err := os.Mkdir(dir, 0777); err != nil {
		// EEXIST is safe to ignore here, that just means the prefix
		// directory already existed.
		if !os.IsExist(err) {
			return err
		}
	}
	return nil
}

var putMaxRetries = 6

func (fs *Datastore) Put(key datastore.Key, value interface{}) error {
	val, ok := value.([]byte)
	if !ok {
		return datastore.ErrInvalidType
	}

	var err error
	for i := 1; i <= putMaxRetries; i++ {
		err = fs.doPut(key, val)
		if err == nil {
			break
		}

		if !strings.Contains(err.Error(), "too many open files") {
			break
		}

		log.Errorf("too many open files, retrying in %dms", 100*i)
		time.Sleep(time.Millisecond * 100 * time.Duration(i))
	}
	return err
}

func (fs *Datastore) doPut(key datastore.Key, val []byte) error {
	dir, path := fs.encode(key)
	if err := fs.makePrefixDir(dir); err != nil {
		return err
	}

	tmp, err := ioutil.TempFile(dir, "put-")
	if err != nil {
		return err
	}
	closed := false
	removed := false
	defer func() {
		if !closed {
			// silence errcheck
			_ = tmp.Close()
		}
		if !removed {
			// silence errcheck
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err := tmp.Write(val); err != nil {
		return err
	}
	if fs.sync {
		if err := tmp.Sync(); err != nil {
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	closed = true

	err = osrename.Rename(tmp.Name(), path)
	if err != nil {
		return err
	}
	removed = true

	if fs.sync {
		if err := syncDir(dir); err != nil {
			return err
		}
	}
	return nil
}

func (fs *Datastore) putMany(data map[datastore.Key]interface{}) error {
	var dirsToSync []string
	files := make(map[*os.File]string)

	for key, value := range data {
		val, ok := value.([]byte)
		if !ok {
			return datastore.ErrInvalidType
		}
		dir, path := fs.encode(key)
		if err := fs.makePrefixDirNoSync(dir); err != nil {
			return err
		}
		dirsToSync = append(dirsToSync, dir)

		tmp, err := ioutil.TempFile(dir, "put-")
		if err != nil {
			return err
		}

		if _, err := tmp.Write(val); err != nil {
			return err
		}

		files[tmp] = path
	}

	ops := make(map[*os.File]int)

	defer func() {
		for fi, _ := range files {
			val, _ := ops[fi]
			switch val {
			case 0:
				_ = fi.Close()
				fallthrough
			case 1:
				_ = os.Remove(fi.Name())
			}
		}
	}()

	// Now we sync everything
	// sync and close files
	for fi, _ := range files {
		if fs.sync {
			if err := fi.Sync(); err != nil {
				return err
			}
		}

		if err := fi.Close(); err != nil {
			return err
		}

		// signify closed
		ops[fi] = 1
	}

	// move files to their proper places
	for fi, path := range files {
		if err := osrename.Rename(fi.Name(), path); err != nil {
			return err
		}

		// signify removed
		ops[fi] = 2
	}

	// now sync the dirs for those files
	if fs.sync {
		for _, dir := range dirsToSync {
			if err := syncDir(dir); err != nil {
				return err
			}
		}

		// sync top flatfs dir
		if err := syncDir(fs.path); err != nil {
			return err
		}
	}

	return nil
}

func (fs *Datastore) Get(key datastore.Key) (value interface{}, err error) {
	_, path := fs.encode(key)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, datastore.ErrNotFound
		}
		// no specific error to return, so just pass it through
		return nil, err
	}
	return data, nil
}

func (fs *Datastore) Has(key datastore.Key) (exists bool, err error) {
	_, path := fs.encode(key)
	switch _, err := os.Stat(path); {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, err
	}
}

func (fs *Datastore) Delete(key datastore.Key) error {
	_, path := fs.encode(key)
	switch err := os.Remove(path); {
	case err == nil:
		return nil
	case os.IsNotExist(err):
		return datastore.ErrNotFound
	default:
		return err
	}
}

func (fs *Datastore) Query(q query.Query) (query.Results, error) {
	if (q.Prefix != "" && q.Prefix != "/") ||
		len(q.Filters) > 0 ||
		len(q.Orders) > 0 ||
		q.Limit > 0 ||
		q.Offset > 0 ||
		!q.KeysOnly {
		// TODO this is overly simplistic, but the only caller is
		// `ipfs refs local` for now, and this gets us moving.
		return nil, errors.New("flatfs only supports listing all keys in random order")
	}

	reschan := make(chan query.Result, query.KeysOnlyBufSize)
	go func() {
		defer close(reschan)
		err := filepath.Walk(fs.path, func(path string, info os.FileInfo, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				log.Errorf("Walk func in Query got error: %v", err)
				return err
			}

			if !info.Mode().IsRegular() || strings.HasPrefix(info.Name(), ".") {
				return nil
			}

			key, ok := fs.decode(info.Name())
			if !ok {
				log.Warning("failed to decode entry in flatfs")
				return nil
			}

			reschan <- query.Result{
				Entry: query.Entry{
					Key: key.String(),
				},
			}
			return nil
		})
		if err != nil {
			log.Warning("walk failed: ", err)
		}
	}()
	return query.ResultsWithChan(q, reschan), nil
}

func (fs *Datastore) Close() error {
	return nil
}

type flatfsBatch struct {
	puts    map[datastore.Key]interface{}
	deletes map[datastore.Key]struct{}

	ds *Datastore
}

func (fs *Datastore) Batch() (datastore.Batch, error) {
	return &flatfsBatch{
		puts:    make(map[datastore.Key]interface{}),
		deletes: make(map[datastore.Key]struct{}),
		ds:      fs,
	}, nil
}

func (bt *flatfsBatch) Put(key datastore.Key, val interface{}) error {
	bt.puts[key] = val
	return nil
}

func (bt *flatfsBatch) Delete(key datastore.Key) error {
	bt.deletes[key] = struct{}{}
	return nil
}

func (bt *flatfsBatch) Commit() error {
	if err := bt.ds.putMany(bt.puts); err != nil {
		return err
	}

	for k, _ := range bt.deletes {
		if err := bt.ds.Delete(k); err != nil {
			return err
		}
	}

	return nil
}

var _ datastore.ThreadSafeDatastore = (*Datastore)(nil)

func (*Datastore) IsThreadSafe() {}
