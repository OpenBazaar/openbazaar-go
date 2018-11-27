package flatfs_test

import (
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	"gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/query"
	dstest "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/test"

	"gx/ipfs/QmbYQHTSz4Sg4Y2Vyd7hftGgM1yXwSUBKb47VpoPRuP5Nc/go-ds-flatfs"
)

func tempdir(t testing.TB) (path string, cleanup func()) {
	path, err := ioutil.TempDir("", "test-datastore-flatfs-")
	if err != nil {
		t.Fatalf("cannot create temp directory: %v", err)
	}

	cleanup = func() {
		if err := os.RemoveAll(path); err != nil {
			t.Errorf("tempdir cleanup failed: %v", err)
		}
	}
	return path, cleanup
}

func tryAllShardFuncs(t *testing.T, testFunc func(mkShardFunc, *testing.T)) {
	t.Run("prefix", func(t *testing.T) { testFunc(flatfs.Prefix, t) })
	t.Run("suffix", func(t *testing.T) { testFunc(flatfs.Suffix, t) })
	t.Run("next-to-last", func(t *testing.T) { testFunc(flatfs.NextToLast, t) })
}

type mkShardFunc func(int) *flatfs.ShardIdV1

func testPut(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	err = fs.Put(datastore.NewKey("quux"), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}
}

func TestPut(t *testing.T) { tryAllShardFuncs(t, testPut) }

func testGet(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	const input = "foobar"
	err = fs.Put(datastore.NewKey("quux"), []byte(input))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	buf, err := fs.Get(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if g, e := string(buf), input; g != e {
		t.Fatalf("Get gave wrong content: %q != %q", g, e)
	}
}

func TestGet(t *testing.T) { tryAllShardFuncs(t, testGet) }

func testPutOverwrite(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	const (
		loser  = "foobar"
		winner = "xyzzy"
	)
	err = fs.Put(datastore.NewKey("quux"), []byte(loser))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	err = fs.Put(datastore.NewKey("quux"), []byte(winner))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	data, err := fs.Get(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if g, e := string(data), winner; g != e {
		t.Fatalf("Get gave wrong content: %q != %q", g, e)
	}
}

func TestPutOverwrite(t *testing.T) { tryAllShardFuncs(t, testPutOverwrite) }

func testGetNotFoundError(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	_, err = fs.Get(datastore.NewKey("quux"))
	if g, e := err, datastore.ErrNotFound; g != e {
		t.Fatalf("expected ErrNotFound, got: %v\n", g)
	}
}

func TestGetNotFoundError(t *testing.T) { tryAllShardFuncs(t, testGetNotFoundError) }

type params struct {
	shard *flatfs.ShardIdV1
	dir   string
	key   string
}

func testStorage(p *params, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	target := p.dir + string(os.PathSeparator) + p.key + ".data"
	fs, err := flatfs.CreateOrOpen(temp, p.shard, false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	err = fs.Put(datastore.NewKey(p.key), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	fs.Close()
	seen := false
	haveREADME := false
	walk := func(absPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		path, err := filepath.Rel(temp, absPath)
		if err != nil {
			return err
		}
		switch path {
		case ".", "..", "SHARDING", flatfs.DiskUsageFile:
			// ignore
		case "_README":
			_, err := ioutil.ReadFile(absPath)
			if err != nil {
				t.Error("could not read _README file")
			}
			haveREADME = true
		case p.dir:
			if !fi.IsDir() {
				t.Errorf("directory is not a file? %v", fi.Mode())
			}
			// we know it's there if we see the file, nothing more to
			// do here
		case target:
			seen = true
			if !fi.Mode().IsRegular() {
				t.Errorf("expected a regular file, mode: %04o", fi.Mode())
			}
			if runtime.GOOS != "windows" {
				if g, e := fi.Mode()&os.ModePerm&0007, os.FileMode(0000); g != e {
					t.Errorf("file should not be world accessible: %04o", fi.Mode())
				}
			}
		default:
			t.Errorf("saw unexpected directory entry: %q %v", path, fi.Mode())
		}
		return nil
	}
	if err := filepath.Walk(temp, walk); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if !seen {
		t.Error("did not see the data file")
	}
	if fs.ShardStr() == flatfs.IPFS_DEF_SHARD_STR && !haveREADME {
		t.Error("expected _README file")
	} else if fs.ShardStr() != flatfs.IPFS_DEF_SHARD_STR && haveREADME {
		t.Error("did not expect _README file")
	}
}

func TestStorage(t *testing.T) {
	t.Run("prefix", func(t *testing.T) {
		testStorage(&params{
			shard: flatfs.Prefix(2),
			dir:   "qu",
			key:   "quux",
		}, t)
	})
	t.Run("suffix", func(t *testing.T) {
		testStorage(&params{
			shard: flatfs.Suffix(2),
			dir:   "ux",
			key:   "quux",
		}, t)
	})
	t.Run("next-to-last", func(t *testing.T) {
		testStorage(&params{
			shard: flatfs.NextToLast(2),
			dir:   "uu",
			key:   "quux",
		}, t)
	})
}

func testHasNotFound(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	found, err := fs.Has(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Has fail: %v\n", err)
	}
	if found {
		t.Fatal("Has should have returned false")
	}
}

func TestHasNotFound(t *testing.T) { tryAllShardFuncs(t, testHasNotFound) }

func testHasFound(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	err = fs.Put(datastore.NewKey("quux"), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	found, err := fs.Has(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Has fail: %v\n", err)
	}
	if !found {
		t.Fatal("Has should have returned true")
	}
}

func TestHasFound(t *testing.T) { tryAllShardFuncs(t, testHasFound) }

func testGetSizeFound(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	_, err = fs.GetSize(datastore.NewKey("quux"))
	if err != datastore.ErrNotFound {
		t.Fatalf("GetSize should have returned ErrNotFound, got: %v\n", err)
	}
}

func TestGetSizeFound(t *testing.T) { tryAllShardFuncs(t, testGetSizeFound) }

func testGetSizeNotFound(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	err = fs.Put(datastore.NewKey("quux"), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	size, err := fs.GetSize(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("GetSize failed with: %v\n", err)
	}
	if size != len("foobar") {
		t.Fatalf("GetSize returned wrong size: got %d, expected %d", size, len("foobar"))
	}
}

func TestGetSizeNotFound(t *testing.T) { tryAllShardFuncs(t, testGetSizeNotFound) }

func testDeleteNotFound(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	err = fs.Delete(datastore.NewKey("quux"))
	if g, e := err, datastore.ErrNotFound; g != e {
		t.Fatalf("expected ErrNotFound, got: %v\n", g)
	}
}

func TestDeleteNotFound(t *testing.T) { tryAllShardFuncs(t, testDeleteNotFound) }

func testDeleteFound(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	err = fs.Put(datastore.NewKey("quux"), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	err = fs.Delete(datastore.NewKey("quux"))
	if err != nil {
		t.Fatalf("Delete fail: %v\n", err)
	}

	// check that it's gone
	_, err = fs.Get(datastore.NewKey("quux"))
	if g, e := err, datastore.ErrNotFound; g != e {
		t.Fatalf("expected Get after Delete to give ErrNotFound, got: %v\n", g)
	}
}

func TestDeleteFound(t *testing.T) { tryAllShardFuncs(t, testDeleteFound) }

func testQuerySimple(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	const myKey = "quux"
	err = fs.Put(datastore.NewKey(myKey), []byte("foobar"))
	if err != nil {
		t.Fatalf("Put fail: %v\n", err)
	}

	res, err := fs.Query(query.Query{KeysOnly: true})
	if err != nil {
		t.Fatalf("Query fail: %v\n", err)
	}
	entries, err := res.Rest()
	if err != nil {
		t.Fatalf("Query Results.Rest fail: %v\n", err)
	}
	seen := false
	for _, e := range entries {
		switch e.Key {
		case datastore.NewKey(myKey).String():
			seen = true
		default:
			t.Errorf("saw unexpected key: %q", e.Key)
		}
	}
	if !seen {
		t.Errorf("did not see wanted key %q in %+v", myKey, entries)
	}
}

func TestQuerySimple(t *testing.T) { tryAllShardFuncs(t, testQuerySimple) }

func testDiskUsage(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	time.Sleep(100 * time.Millisecond)
	duNew, err := fs.DiskUsage()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("duNew:", duNew)

	count := 200
	for i := 0; i < count; i++ {
		k := datastore.NewKey(fmt.Sprintf("test-%d", i))
		v := []byte("10bytes---")
		err = fs.Put(k, v)
		if err != nil {
			t.Fatalf("Put fail: %v\n", err)
		}
	}

	time.Sleep(100 * time.Millisecond)
	duElems, err := fs.DiskUsage()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("duPostPut:", duElems)

	for i := 0; i < count; i++ {
		k := datastore.NewKey(fmt.Sprintf("test-%d", i))
		err = fs.Delete(k)
		if err != nil {
			t.Fatalf("Delete fail: %v\n", err)
		}
	}

	time.Sleep(100 * time.Millisecond)
	duDelete, err := fs.DiskUsage()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("duPostDelete:", duDelete)

	du, err := fs.DiskUsage()
	t.Log("duFinal:", du)
	if err != nil {
		t.Fatal(err)
	}
	fs.Close()

	// Check that disk usage file is correct
	duB, err := ioutil.ReadFile(filepath.Join(temp, flatfs.DiskUsageFile))
	if err != nil {
		t.Fatal(err)
	}
	contents := make(map[string]interface{})
	err = json.Unmarshal(duB, &contents)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure diskUsage value is correct
	if val, ok := contents["diskUsage"].(float64); !ok || uint64(val) != du {
		t.Fatalf("Unexpected value for diskUsage in %s: %v (expected %d)",
			flatfs.DiskUsageFile, contents["diskUsage"], du)
	}

	// Make sure the accuracy value is correct
	if val, ok := contents["accuracy"].(string); !ok || val != "initial-exact" {
		t.Fatalf("Unexpected value for accuracyin %s: %v",
			flatfs.DiskUsageFile, contents["accuracy"])
	}

	// Make sure size is correctly calculated on re-open
	os.Remove(filepath.Join(temp, flatfs.DiskUsageFile))
	fs, err = flatfs.Open(temp, false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}

	duReopen, err := fs.DiskUsage()
	if err != nil {
		t.Fatal(err)
	}
	t.Log("duReopen:", duReopen)

	// Checks
	if duNew == 0 {
		t.Error("new datastores should have some size")
	}

	if duElems <= duNew {
		t.Error("size should grow as new elements are added")
	}

	if duElems-duDelete != uint64(count*10) {
		t.Error("size should be reduced exactly as size of objects deleted")
	}

	if duReopen < duNew {
		t.Error("Reopened datastore should not be smaller")
	}
}

func TestDiskUsage(t *testing.T) {
	tryAllShardFuncs(t, testDiskUsage)
}

func TestDiskUsageDoubleCount(t *testing.T) {
	tryAllShardFuncs(t, testDiskUsageDoubleCount)
}

// test that concurrently writing and deleting the same key/value
// does not throw any errors and disk usage does not do
// any double-counting.
func testDiskUsageDoubleCount(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	var count int
	var wg sync.WaitGroup
	testKey := datastore.NewKey("test")

	put := func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			v := []byte("10bytes---")
			err := fs.Put(testKey, v)
			if err != nil {
				t.Fatalf("Put fail: %v\n", err)
			}
		}
	}

	del := func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			err := fs.Delete(testKey)
			if err != nil && !strings.Contains(err.Error(), "key not found") {
				t.Fatalf("Delete fail: %v\n", err)
			}
		}
	}

	// Add one element and then remove it and check disk usage
	// makes sense
	count = 1
	wg.Add(2)
	put()
	du, _ := fs.DiskUsage()
	del()
	du2, _ := fs.DiskUsage()
	if du-10 != du2 {
		t.Error("should have deleted exactly 10 bytes:", du, du2)
	}

	// Add and remove many times at the same time
	count = 200
	wg.Add(4)
	go put()
	go del()
	go put()
	go del()
	wg.Wait()

	du3, _ := fs.DiskUsage()
	has, err := fs.Has(testKey)
	if err != nil {
		t.Fatal(err)
	}

	if has { // put came last
		if du3 != du {
			t.Error("du should be the same as after first put:", du, du3)
		}
	} else { //delete came last
		if du3 != du2 {
			t.Error("du should be the same as after first delete:", du2, du3)
		}
	}
}

func testDiskUsageBatch(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	fsBatch, _ := fs.Batch()

	count := 200
	var wg sync.WaitGroup
	testKeys := []datastore.Key{}
	for i := 0; i < count; i++ {
		k := datastore.NewKey(fmt.Sprintf("test%d", i))
		testKeys = append(testKeys, k)
	}

	put := func() {
		for i := 0; i < count; i++ {
			fsBatch.Put(testKeys[i], []byte("10bytes---"))
		}
	}
	commit := func() {
		defer wg.Done()
		err := fsBatch.Commit()
		if err != nil {
			t.Fatalf("Batch Put fail: %v\n", err)
		}
	}

	del := func() {
		defer wg.Done()
		for _, k := range testKeys {
			err := fs.Delete(k)
			if err != nil && !strings.Contains(err.Error(), "key not found") {
				t.Fatalf("Delete fail: %v\n", err)
			}
		}
	}

	// Put many elements and then delete them and check disk usage
	// makes sense
	wg.Add(2)
	put()
	commit()
	du, _ := fs.DiskUsage()
	del()
	du2, _ := fs.DiskUsage()
	if du-uint64(10*count) != du2 {
		t.Errorf("should have deleted exactly %d bytes: %d %d", 10*count, du, du2)
	}

	// Do deletes while doing putManys concurrently
	wg.Add(2)
	put()
	go commit()
	go del()
	wg.Wait()

	du3, _ := fs.DiskUsage()
	// Now query how many keys we have
	results, err := fs.Query(query.Query{
		KeysOnly: true,
	})
	rest, err := results.Rest()
	if err != nil {
		t.Fatal(err)
	}

	expectedSize := uint64(len(rest) * 10)

	if exp := du2 + expectedSize; exp != du3 {
		t.Error("diskUsage has skewed off from real size:",
			exp, du3)
	}
}

func TestDiskUsageBatch(t *testing.T) { tryAllShardFuncs(t, testDiskUsageBatch) }

func testDiskUsageEstimation(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	count := 50000
	for i := 0; i < count; i++ {
		k := datastore.NewKey(fmt.Sprintf("%d-test-%d", i, i))
		v := make([]byte, 1000)
		err = fs.Put(k, v)
		if err != nil {
			t.Fatalf("Put fail: %v\n", err)
		}
	}

	// Delete checkpoint
	fs.Close()
	os.Remove(filepath.Join(temp, flatfs.DiskUsageFile))

	// This will do a full du
	flatfs.DiskUsageFilesAverage = -1
	fs, err = flatfs.Open(temp, false)
	if err != nil {
		t.Fatalf("Open fail: %v\n", err)
	}

	duReopen, err := fs.DiskUsage()
	if err != nil {
		t.Fatal(err)
	}

	fs.Close()
	os.Remove(filepath.Join(temp, flatfs.DiskUsageFile))

	// This will estimate the size. Since all files are the same
	// length we can use a low file average number.
	flatfs.DiskUsageFilesAverage = 100
	// Make sure size is correctly calculated on re-open
	fs, err = flatfs.Open(temp, false)
	if err != nil {
		t.Fatalf("Open fail: %v\n", err)
	}

	duEst, err := fs.DiskUsage()
	if err != nil {
		t.Fatal(err)
	}

	t.Log("RealDu:", duReopen)
	t.Log("Est:", duEst)

	diff := int(math.Abs(float64(int(duReopen) - int(duEst))))
	maxDiff := int(0.05 * float64(duReopen)) // %5 of actual

	if diff > maxDiff {
		t.Fatalf("expected a better estimation within 5%%")
	}

	// Make sure the accuracy value is correct
	if fs.Accuracy() != "initial-approximate" {
		t.Errorf("Unexpected value for fs.Accuracy(): %s", fs.Accuracy())
	}

	fs.Close()

	// Reopen into a new variable
	fs2, err := flatfs.Open(temp, false)
	if err != nil {
		t.Fatalf("Open fail: %v\n", err)
	}

	// Make sure the accuracy value is preserved
	if fs2.Accuracy() != "initial-approximate" {
		t.Errorf("Unexpected value for fs.Accuracy(): %s", fs2.Accuracy())
	}
}

func TestDiskUsageEstimation(t *testing.T) { tryAllShardFuncs(t, testDiskUsageEstimation) }

func testBatchPut(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	dstest.RunBatchTest(t, fs)
}

func TestBatchPut(t *testing.T) { tryAllShardFuncs(t, testBatchPut) }

func testBatchDelete(dirFunc mkShardFunc, t *testing.T) {
	temp, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, dirFunc(2), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	dstest.RunBatchDeleteTest(t, fs)
}

func TestBatchDelete(t *testing.T) { tryAllShardFuncs(t, testBatchDelete) }

func TestSHARDINGFile(t *testing.T) {
	tempdir, cleanup := tempdir(t)
	defer cleanup()

	fun := flatfs.IPFS_DEF_SHARD

	err := flatfs.Create(tempdir, fun)
	if err != nil {
		t.Fatalf("Create: %v\n", err)
	}

	fs, err := flatfs.Open(tempdir, false)
	if err != nil {
		t.Fatalf("Open fail: %v\n", err)
	}
	if fs.ShardStr() != flatfs.IPFS_DEF_SHARD_STR {
		t.Fatalf("Expected '%s' for shard function got '%s'", flatfs.IPFS_DEF_SHARD_STR, fs.ShardStr())
	}
	fs.Close()

	fs, err = flatfs.CreateOrOpen(tempdir, fun, false)
	if err != nil {
		t.Fatalf("Could not reopen repo: %v\n", err)
	}
	fs.Close()

	fs, err = flatfs.CreateOrOpen(tempdir, flatfs.Prefix(5), false)
	if err == nil {
		t.Fatalf("Was able to open repo with incompatible sharding function")
	}
}

func TestInvalidPrefix(t *testing.T) {
	_, err := flatfs.ParseShardFunc("/bad/prefix/v1/next-to-last/2")
	if err == nil {
		t.Fatalf("Expected an error while parsing a shard identifier with a bad prefix")
	}
}

func TestNonDatastoreDir(t *testing.T) {
	tempdir, cleanup := tempdir(t)
	defer cleanup()

	ioutil.WriteFile(filepath.Join(tempdir, "afile"), []byte("Some Content"), 0644)

	err := flatfs.Create(tempdir, flatfs.NextToLast(2))
	if err == nil {
		t.Fatalf("Expected an error when creating a datastore in a non-empty directory")
	}
}

func TestNoCluster(t *testing.T) {
	tempdir, cleanup := tempdir(t)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(tempdir, flatfs.NextToLast(1), false)
	if err != nil {
		t.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	r := rand.New(rand.NewSource(0))
	N := 3200 // should be divisible by 32 so the math works out
	for i := 0; i < N; i++ {
		blk := make([]byte, 1000)
		r.Read(blk)

		key := "CIQ" + base32.StdEncoding.EncodeToString(blk[:10])
		err := fs.Put(datastore.NewKey(key), blk)
		if err != nil {
			t.Fatalf("Put fail: %v\n", err)
		}
	}

	fs.Close()
	dirs, err := ioutil.ReadDir(tempdir)
	if err != nil {
		t.Fatalf("ReadDir fail: %v\n", err)
	}
	idealFilesPerDir := float64(N) / 32.0
	tolerance := math.Floor(idealFilesPerDir * 0.25)
	count := 0
	for _, dir := range dirs {
		if dir.Name() == flatfs.SHARDING_FN ||
			dir.Name() == flatfs.README_FN ||
			dir.Name() == flatfs.DiskUsageFile {
			continue
		}
		count += 1
		files, err := ioutil.ReadDir(filepath.Join(tempdir, dir.Name()))
		if err != nil {
			t.Fatalf("ReadDir fail: %v\n", err)
		}
		num := float64(len(files))
		if math.Abs(num-idealFilesPerDir) > tolerance {
			t.Fatalf("Dir %s has %.0f files, expected between %.f and %.f files",
				filepath.Join(tempdir, dir.Name()), num, idealFilesPerDir-tolerance, idealFilesPerDir+tolerance)
		}
	}
	if count != 32 {
		t.Fatalf("Expected 32 directories and one file in %s", tempdir)
	}
}

func BenchmarkConsecutivePut(b *testing.B) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var blocks [][]byte
	var keys []datastore.Key
	for i := 0; i < b.N; i++ {
		blk := make([]byte, 256*1024)
		r.Read(blk)
		blocks = append(blocks, blk)

		key := base32.StdEncoding.EncodeToString(blk[:8])
		keys = append(keys, datastore.NewKey(key))
	}
	temp, cleanup := tempdir(b)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, flatfs.Prefix(2), false)
	if err != nil {
		b.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := fs.Put(keys[i], blocks[i])
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer() // avoid counting cleanup
}

func BenchmarkBatchedPut(b *testing.B) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var blocks [][]byte
	var keys []datastore.Key
	for i := 0; i < b.N; i++ {
		blk := make([]byte, 256*1024)
		r.Read(blk)
		blocks = append(blocks, blk)

		key := base32.StdEncoding.EncodeToString(blk[:8])
		keys = append(keys, datastore.NewKey(key))
	}
	temp, cleanup := tempdir(b)
	defer cleanup()

	fs, err := flatfs.CreateOrOpen(temp, flatfs.Prefix(2), false)
	if err != nil {
		b.Fatalf("New fail: %v\n", err)
	}
	defer fs.Close()

	b.ResetTimer()

	for i := 0; i < b.N; {
		batch, err := fs.Batch()
		if err != nil {
			b.Fatal(err)
		}

		for n := i; i-n < 512 && i < b.N; i++ {
			err := batch.Put(keys[i], blocks[i])
			if err != nil {
				b.Fatal(err)
			}
		}
		err = batch.Commit()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer() // avoid counting cleanup
}
