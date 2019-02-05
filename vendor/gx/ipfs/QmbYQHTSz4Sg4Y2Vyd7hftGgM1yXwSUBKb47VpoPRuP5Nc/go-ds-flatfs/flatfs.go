// Package flatfs is a Datastore implementation that stores all
// objects in a two-level directory structure in the local file
// system, regardless of the hierarchy of the keys.
package flatfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	"gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/query"

	logging "gx/ipfs/QmZChCsSt8DctjceaL56Eibc29CVQq4dGKRXC5JRZ6Ppae/go-log"
)

var log = logging.Logger("flatfs")

const (
	extension                  = ".data"
	diskUsageMessageTimeout    = 5 * time.Second
	diskUsageCheckpointPercent = 1.0
	diskUsageCheckpointTimeout = 2 * time.Second
)

var (
	// DiskUsageFile is the name of the file to cache the size of the
	// datastore in disk
	DiskUsageFile = "diskUsage.cache"
	// DiskUsageFilesAverage is the maximum number of files per folder
	// to stat in order to calculate the size of the datastore.
	// The size of the rest of the files in a folder will be assumed
	// to be the average of the values obtained. This includes
	// regular files and directories.
	DiskUsageFilesAverage = 2000
	// DiskUsageCalcTimeout is the maximum time to spend
	// calculating the DiskUsage upon a start when no
	// DiskUsageFile is present.
	// If this period did not suffice to read the size of the datastore,
	// the remaining sizes will be stimated.
	DiskUsageCalcTimeout = 5 * time.Minute
)

const (
	opPut = iota
	opDelete
	opRename
)

type initAccuracy string

const (
	unknownA  initAccuracy = "unknown"
	exactA    initAccuracy = "initial-exact"
	approxA   initAccuracy = "initial-approximate"
	timedoutA initAccuracy = "initial-timed-out"
)

func combineAccuracy(a, b initAccuracy) initAccuracy {
	if a == unknownA || b == unknownA {
		return unknownA
	}
	if a == timedoutA || b == timedoutA {
		return timedoutA
	}
	if a == approxA || b == approxA {
		return approxA
	}
	if a == exactA && b == exactA {
		return exactA
	}
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return unknownA
}

var _ datastore.Datastore = (*Datastore)(nil)

var (
	ErrDatastoreExists       = errors.New("datastore already exists")
	ErrDatastoreDoesNotExist = errors.New("datastore directory does not exist")
	ErrShardingFileMissing   = fmt.Errorf("%s file not found in datastore", SHARDING_FN)
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// Datastore implements the go-datastore Interface.
// Note this datastore cannot guarantee order of concurrent
// write operations to the same key. See the explanation in
// Put().
type Datastore struct {
	// atmoic operations should always be used with diskUsage.
	// Must be first in struct to ensure correct alignment
	// (see https://golang.org/pkg/sync/atomic/#pkg-note-BUG)
	diskUsage int64

	path string

	shardStr string
	getDir   ShardFunc

	// sychronize all writes and directory changes for added safety
	sync bool

	// these values should only be used during internalization or
	// inside the checkpoint loop
	dirty       bool
	storedValue diskUsageValue

	checkpointCh chan struct{}
	done         chan struct{}

	// opMap handles concurrent write operations (put/delete)
	// to the same key
	opMap *opMap
}

type diskUsageValue struct {
	DiskUsage int64        `json:"diskUsage"`
	Accuracy  initAccuracy `json:"accuracy"`
}

type ShardFunc func(string) string

type opT int

// op wraps useful arguments of write operations
type op struct {
	typ  opT           // operation type
	key  datastore.Key // datastore key. Mandatory.
	tmp  string        // temp file path
	path string        // file path
	v    []byte        // value
}

type opMap struct {
	ops sync.Map
}

type opResult struct {
	mu      sync.RWMutex
	success bool

	opMap *opMap
	name  string
}

// Returns nil if there's nothing to do.
func (m *opMap) Begin(name string) *opResult {
	for {
		myOp := &opResult{opMap: m, name: name}
		myOp.mu.Lock()
		opIface, loaded := m.ops.LoadOrStore(name, myOp)
		if !loaded { // no one else doing ops with this key
			return myOp
		}

		op := opIface.(*opResult)
		// someone else doing ops with this key, wait for
		// the result
		op.mu.RLock()
		if op.success {
			return nil
		}

		// if we are here, we will retry the operation
	}
}

func (o *opResult) Finish(ok bool) {
	o.success = ok
	o.opMap.ops.Delete(o.name)
	o.mu.Unlock()
}

func Create(path string, fun *ShardIdV1) error {

	err := os.Mkdir(path, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	dsFun, err := ReadShardFunc(path)
	switch err {
	case ErrShardingFileMissing:
		isEmpty, err := DirIsEmpty(path)
		if err != nil {
			return err
		}
		if !isEmpty {
			return fmt.Errorf("directory missing %s file: %s", SHARDING_FN, path)
		}

		err = WriteShardFunc(path, fun)
		if err != nil {
			return err
		}
		err = WriteReadme(path, fun)
		return err
	case nil:
		if fun.String() != dsFun.String() {
			return fmt.Errorf("specified shard func '%s' does not match repo shard func '%s'",
				fun.String(), dsFun.String())
		}
		return ErrDatastoreExists
	default:
		return err
	}
}

func Open(path string, syncFiles bool) (*Datastore, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, ErrDatastoreDoesNotExist
	} else if err != nil {
		return nil, err
	}

	shardId, err := ReadShardFunc(path)
	if err != nil {
		return nil, err
	}

	fs := &Datastore{
		path:      path,
		shardStr:  shardId.String(),
		getDir:    shardId.Func(),
		sync:      syncFiles,
		diskUsage: 0,
		opMap:     new(opMap),
	}

	// This sets diskUsage to the correct value
	// It might be slow, but allowing it to happen
	// while the datastore is usable might
	// cause diskUsage to not be accurate.
	err = fs.calculateDiskUsage()
	if err != nil {
		// Cannot stat() all
		// elements in the datastore.
		return nil, err
	}

	fs.checkpointCh = make(chan struct{}, 1)
	fs.done = make(chan struct{})
	go fs.checkpointLoop()
	return fs, nil
}

// convenience method
func CreateOrOpen(path string, fun *ShardIdV1, sync bool) (*Datastore, error) {
	err := Create(path, fun)
	if err != nil && err != ErrDatastoreExists {
		return nil, err
	}
	return Open(path, sync)
}

func (fs *Datastore) ShardStr() string {
	return fs.shardStr
}

func (fs *Datastore) encode(key datastore.Key) (dir, file string) {
	noslash := key.String()[1:]
	dir = filepath.Join(fs.path, fs.getDir(noslash))
	file = filepath.Join(dir, noslash+extension)
	return dir, file
}

func (fs *Datastore) decode(file string) (key datastore.Key, ok bool) {
	if filepath.Ext(file) != extension {
		return datastore.Key{}, false
	}
	name := file[:len(file)-len(extension)]
	return datastore.NewKey(name), true
}

func (fs *Datastore) makeDir(dir string) error {
	if err := fs.makeDirNoSync(dir); err != nil {
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

func (fs *Datastore) makeDirNoSync(dir string) error {
	if err := os.Mkdir(dir, 0755); err != nil {
		// EEXIST is safe to ignore here, that just means the prefix
		// directory already existed.
		if !os.IsExist(err) {
			return err
		}
		return nil
	}

	// Track DiskUsage of this NEW folder
	fs.updateDiskUsage(dir, true)
	return nil
}

// This function always runs under an opLock. Therefore, only one thread is
// touching the affected files.
func (fs *Datastore) renameAndUpdateDiskUsage(tmpPath, path string) error {
	fi, err := os.Stat(path)

	// Destination exists, we need to discount it from diskUsage
	if fs != nil && err == nil {
		atomic.AddInt64(&fs.diskUsage, -fi.Size())
	} else if !os.IsNotExist(err) {
		return err
	}

	// Rename and add new file's diskUsage. If the rename fails,
	// it will either a) Re-add the size of an existing file, which
	// was sustracted before b) Add 0 if there is no existing file.
	err = os.Rename(tmpPath, path)
	fs.updateDiskUsage(path, true)
	return err
}

var putMaxRetries = 6

// Put stores a key/value in the datastore.
//
// Note, that we do not guarantee order of write operations (Put or Delete)
// to the same key in this datastore.
//
// For example. i.e. in the case of two concurrent Put, we only guarantee
// that one of them will come through, but cannot assure which one even if
// one arrived slightly later than the other. In the case of a
// concurrent Put and a Delete operation, we cannot guarantee which one
// will win.
func (fs *Datastore) Put(key datastore.Key, value []byte) error {
	var err error
	for i := 1; i <= putMaxRetries; i++ {
		err = fs.doWriteOp(&op{
			typ: opPut,
			key: key,
			v:   value,
		})
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

func (fs *Datastore) doOp(oper *op) error {
	switch oper.typ {
	case opPut:
		return fs.doPut(oper.key, oper.v)
	case opDelete:
		return fs.doDelete(oper.key)
	case opRename:
		return fs.renameAndUpdateDiskUsage(oper.tmp, oper.path)
	default:
		panic("bad operation, this is a bug")
	}
}

// doWrite optmizes out write operations (put/delete) to the same
// key by queueing them and suceeding all queued
// operations if one of them does. In such case,
// we assume that the first suceeding operation
// on that key was the last one to happen after
// all successful others.
func (fs *Datastore) doWriteOp(oper *op) error {
	keyStr := oper.key.String()

	opRes := fs.opMap.Begin(keyStr)
	if opRes == nil { // nothing to do, a concurrent op succeeded
		return nil
	}

	// Do the operation
	err := fs.doOp(oper)

	// Finish it. If no error, it will signal other operations
	// waiting on this result to succeed. Otherwise, they will
	// retry.
	opRes.Finish(err == nil)
	return err
}

func (fs *Datastore) doPut(key datastore.Key, val []byte) error {

	dir, path := fs.encode(key)
	if err := fs.makeDir(dir); err != nil {
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
		if err := syncFile(tmp); err != nil {
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	closed = true

	err = fs.renameAndUpdateDiskUsage(tmp.Name(), path)
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
	files := make(map[*os.File]*op)

	for key, value := range data {
		val, ok := value.([]byte)
		if !ok {
			return datastore.ErrInvalidType
		}
		dir, path := fs.encode(key)
		if err := fs.makeDirNoSync(dir); err != nil {
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

		files[tmp] = &op{
			typ:  opRename,
			path: path,
			tmp:  tmp.Name(),
			key:  key,
		}
	}

	ops := make(map[*os.File]int)

	defer func() {
		for fi := range files {
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
	for fi := range files {
		if fs.sync {
			if err := syncFile(fi); err != nil {
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
	for fi, op := range files {
		fs.doWriteOp(op)
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

func (fs *Datastore) Get(key datastore.Key) (value []byte, err error) {
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

func (fs *Datastore) GetSize(key datastore.Key) (size int, err error) {
	_, path := fs.encode(key)
	switch s, err := os.Stat(path); {
	case err == nil:
		return int(s.Size()), nil
	case os.IsNotExist(err):
		return -1, datastore.ErrNotFound
	default:
		return -1, err
	}
}

// Delete removes a key/value from the Datastore. Please read
// the Put() explanation about the handling of concurrent write
// operations to the same key.
func (fs *Datastore) Delete(key datastore.Key) error {
	return fs.doWriteOp(&op{
		typ: opDelete,
		key: key,
		v:   nil,
	})
}

// This function always runs within an opLock for the given
// key, and not concurrently.
func (fs *Datastore) doDelete(key datastore.Key) error {
	_, path := fs.encode(key)

	fSize := fileSize(path)

	switch err := os.Remove(path); {
	case err == nil:
		atomic.AddInt64(&fs.diskUsage, -fSize)
		fs.checkpointDiskUsage()
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
		err := fs.walkTopLevel(fs.path, reschan)
		if err != nil {
			reschan <- query.Result{Error: errors.New("walk failed: " + err.Error())}
		}
	}()
	return query.ResultsWithChan(q, reschan), nil
}

func (fs *Datastore) walkTopLevel(path string, reschan chan query.Result) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, dir := range names {

		if len(dir) == 0 || dir[0] == '.' {
			continue
		}

		err = fs.walk(filepath.Join(path, dir), reschan)
		if err != nil {
			return err
		}

	}
	return nil
}

// folderSize estimates the diskUsage of a folder by reading
// up to DiskUsageFilesAverage entries in it and assumming any
// other files will have an avereage size.
func folderSize(path string, deadline time.Time) (int64, initAccuracy, error) {
	var du int64

	folder, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer folder.Close()

	stat, err := folder.Stat()
	if err != nil {
		return 0, "", err
	}

	files, err := folder.Readdirnames(-1)
	if err != nil {
		return 0, "", err
	}

	totalFiles := len(files)
	i := 0
	filesProcessed := 0
	maxFiles := DiskUsageFilesAverage
	if maxFiles <= 0 {
		maxFiles = totalFiles
	}

	// randomize file order
	// https://stackoverflow.com/a/42776696
	for i := len(files) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		files[i], files[j] = files[j], files[i]
	}

	accuracy := exactA
	for {
		// Do not process any files after deadline is over
		if time.Now().After(deadline) {
			accuracy = timedoutA
			break
		}

		if i >= totalFiles || filesProcessed >= maxFiles {
			if filesProcessed >= maxFiles {
				accuracy = approxA
			}
			break
		}

		// Stat the file
		fname := files[i]
		subpath := filepath.Join(path, fname)
		st, err := os.Stat(subpath)
		if err != nil {
			return 0, "", err
		}

		// Find folder size recursively
		if st.IsDir() {
			du2, acc, err := folderSize(filepath.Join(subpath), deadline)
			if err != nil {
				return 0, "", err
			}
			accuracy = combineAccuracy(acc, accuracy)
			du += du2
			filesProcessed++
		} else { // in any other case, add the file size
			du += st.Size()
			filesProcessed++
		}

		i++
	}

	nonProcessed := totalFiles - filesProcessed

	// Avg is total size in this folder up to now / total files processed
	// it includes folders ant not folders
	avg := 0.0
	if filesProcessed > 0 {
		avg = float64(du) / float64(filesProcessed)
	}
	duEstimation := int64(avg * float64(nonProcessed))
	du += duEstimation
	du += stat.Size()
	//fmt.Println(path, "total:", totalFiles, "totalStat:", i, "totalFile:", filesProcessed, "left:", nonProcessed, "avg:", int(avg), "est:", int(duEstimation), "du:", du)
	return du, accuracy, nil
}

// calculateDiskUsage tries to read the DiskUsageFile for a cached
// diskUsage value, otherwise walks the datastore files.
// it is only safe to call in Open()
func (fs *Datastore) calculateDiskUsage() error {
	// Try to obtain a previously stored value from disk
	if persDu := fs.readDiskUsageFile(); persDu > 0 {
		fs.diskUsage = persDu
		return nil
	}

	msgDone := make(chan struct{}, 1) // prevent race condition
	msgTimer := time.AfterFunc(diskUsageMessageTimeout, func() {
		fmt.Printf("Calculating datastore size. This might take %s at most and will happen only once\n",
			DiskUsageCalcTimeout.String())
		msgDone <- struct{}{}
	})
	defer msgTimer.Stop()
	deadline := time.Now().Add(DiskUsageCalcTimeout)
	du, accuracy, err := folderSize(fs.path, deadline)
	if err != nil {
		return err
	}
	if !msgTimer.Stop() {
		<-msgDone
	}
	if accuracy == timedoutA {
		fmt.Println("WARN: It took to long to calculate the datastore size")
		fmt.Printf("WARN: The total size (%d) is an estimation. You can fix errors by\n", du)
		fmt.Printf("WARN: replacing the %s file with the right disk usage in bytes and\n",
			filepath.Join(fs.path, DiskUsageFile))
		fmt.Println("WARN: re-opening the datastore")
	}

	fs.storedValue.Accuracy = accuracy
	fs.diskUsage = du
	fs.writeDiskUsageFile(du, true)

	return nil
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// updateDiskUsage reads the size of path and atomically
// increases or decreases the diskUsage variable.
// setting add to false will subtract from disk usage.
func (fs *Datastore) updateDiskUsage(path string, add bool) {
	fsize := fileSize(path)
	if !add {
		fsize = -fsize
	}

	if fsize != 0 {
		atomic.AddInt64(&fs.diskUsage, fsize)
		fs.checkpointDiskUsage()
	}
}

func (fs *Datastore) checkpointDiskUsage() {
	select {
	case fs.checkpointCh <- struct{}{}:
		// msg sent
	default:
		// checkpoint request already pending
	}
}

func (fs *Datastore) checkpointLoop() {
	timerActive := true
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case _, more := <-fs.checkpointCh:
			du := atomic.LoadInt64(&fs.diskUsage)
			fs.dirty = true
			if !more { // shutting down
				fs.writeDiskUsageFile(du, true)
				if fs.dirty {
					log.Errorf("could not store final value of disk usage to file, future estimates may be inaccurate")
				}
				fs.done <- struct{}{}
				return
			}
			// If the difference between the checkpointed disk usage and
			// current one is larger than than `diskUsageCheckpointPercent`
			// of the checkpointed: store it.
			newDu := float64(du)
			lastCheckpointDu := float64(fs.storedValue.DiskUsage)
			diff := math.Abs(newDu - lastCheckpointDu)
			if lastCheckpointDu*diskUsageCheckpointPercent < diff*100.0 {
				fs.writeDiskUsageFile(du, false)
			}
			// Otherwise insure the value will be written to disk after
			// `diskUsageCheckpointTimeout`
			if fs.dirty && !timerActive {
				timer.Reset(diskUsageCheckpointTimeout)
				timerActive = true
			}
		case <-timer.C:
			timerActive = false
			if fs.dirty {
				du := atomic.LoadInt64(&fs.diskUsage)
				fs.writeDiskUsageFile(du, false)
			}
		}
	}
}

func (fs *Datastore) writeDiskUsageFile(du int64, doSync bool) {
	tmp, err := ioutil.TempFile(fs.path, "du-")
	if err != nil {
		log.Warningf("cound not write disk usage: %v", err)
		return
	}

	removed := false
	defer func() {
		if !removed {
			// silence errcheck
			_ = os.Remove(tmp.Name())
		}
	}()

	toWrite := fs.storedValue
	toWrite.DiskUsage = du
	encoder := json.NewEncoder(tmp)
	if err := encoder.Encode(&toWrite); err != nil {
		log.Warningf("cound not write disk usage: %v", err)
		return
	}

	if doSync {
		if err := tmp.Sync(); err != nil {
			log.Warningf("cound not sync %s: %v", DiskUsageFile, err)
			return
		}
	}

	if err := tmp.Close(); err != nil {
		log.Warningf("cound not write disk usage: %v", err)
		return
	}

	if err := os.Rename(tmp.Name(), filepath.Join(fs.path, DiskUsageFile)); err != nil {
		log.Warningf("cound not write disk usage: %v", err)
		return
	}
	removed = true

	fs.storedValue = toWrite
	fs.dirty = false
}

// readDiskUsageFile is only safe to call in Open()
func (fs *Datastore) readDiskUsageFile() int64 {
	fpath := filepath.Join(fs.path, DiskUsageFile)
	duB, err := ioutil.ReadFile(fpath)
	if err != nil {
		return 0
	}
	err = json.Unmarshal(duB, &fs.storedValue)
	if err != nil {
		return 0
	}
	return fs.storedValue.DiskUsage
}

// DiskUsage implements the PersistentDatastore interface
// and returns the current disk usage in bytes used by
// this datastore.
//
// The size is approximative and may slightly differ from
// the real disk values.
func (fs *Datastore) DiskUsage() (uint64, error) {
	// it may differ from real disk values if
	// the filesystem has allocated for blocks
	// for a directory because it has many files in it
	// we don't account for "resized" directories.
	// In a large datastore, the differences should be
	// are negligible though.

	du := atomic.LoadInt64(&fs.diskUsage)
	return uint64(du), nil
}

// Accuracy returns a string representing the accuracy of the
// DiskUsage() result, the value returned is implementation defined
// and for informational purposes only
func (fs *Datastore) Accuracy() string {
	return string(fs.storedValue.Accuracy)
}

func (fs *Datastore) walk(path string, reschan chan query.Result) error {
	dir, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// not an error if the file disappeared
			return nil
		}
		return err
	}
	defer dir.Close()

	// ignore non-directories
	fileInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return nil
	}

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, fn := range names {

		if len(fn) == 0 || fn[0] == '.' {
			continue
		}

		key, ok := fs.decode(fn)
		if !ok {
			log.Warningf("failed to decode flatfs entry: %s", fn)
			continue
		}

		reschan <- query.Result{
			Entry: query.Entry{
				Key: key.String(),
			},
		}
	}
	return nil
}

// Deactivate closes background maintenance threads, most write
// operations will fail but readonly operations will continue to
// function
func (fs *Datastore) deactivate() error {
	if fs.checkpointCh != nil {
		close(fs.checkpointCh)
		<-fs.done
		fs.checkpointCh = nil
	}
	return nil
}

func (fs *Datastore) Close() error {
	return fs.deactivate()
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

func (bt *flatfsBatch) Put(key datastore.Key, val []byte) error {
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

	for k := range bt.deletes {
		if err := bt.ds.Delete(k); err != nil {
			return err
		}
	}

	return nil
}

var _ datastore.ThreadSafeDatastore = (*Datastore)(nil)

func (*Datastore) IsThreadSafe() {}
