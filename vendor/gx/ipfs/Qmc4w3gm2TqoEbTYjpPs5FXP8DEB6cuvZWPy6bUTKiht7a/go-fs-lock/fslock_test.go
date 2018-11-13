package fslock_test

import (
	"os"
	"path"
	"testing"

	lock "gx/ipfs/Qmc4w3gm2TqoEbTYjpPs5FXP8DEB6cuvZWPy6bUTKiht7a/go-fs-lock"
)

func assertLock(t *testing.T, confdir, lockFile string, expected bool) {
	t.Helper()

	isLocked, err := lock.Locked(confdir, lockFile)
	if err != nil {
		t.Fatal(err)
	}

	if isLocked != expected {
		t.Fatalf("expected %t to be %t", isLocked, expected)
	}
}

func TestLockSimple(t *testing.T) {
	lockFile := "my-test.lock"
	confdir := os.TempDir()

	// make sure we start clean
	_ = os.Remove(path.Join(confdir, lockFile))

	assertLock(t, confdir, lockFile, false)

	lockfile, err := lock.Lock(confdir, lockFile)
	if err != nil {
		t.Fatal(err)
	}

	assertLock(t, confdir, lockFile, true)

	if err := lockfile.Close(); err != nil {
		t.Fatal(err)
	}

	assertLock(t, confdir, lockFile, false)

	// second round of locking

	lockfile, err = lock.Lock(confdir, lockFile)
	if err != nil {
		t.Fatal(err)
	}

	assertLock(t, confdir, lockFile, true)

	if err := lockfile.Close(); err != nil {
		t.Fatal(err)
	}

	assertLock(t, confdir, lockFile, false)
}

func TestLockMultiple(t *testing.T) {
	lockFile1 := "test-1.lock"
	lockFile2 := "test-2.lock"
	confdir := os.TempDir()

	// make sure we start clean
	_ = os.Remove(path.Join(confdir, lockFile1))
	_ = os.Remove(path.Join(confdir, lockFile2))

	lock1, err := lock.Lock(confdir, lockFile1)
	if err != nil {
		t.Fatal(err)
	}
	lock2, err := lock.Lock(confdir, lockFile2)
	if err != nil {
		t.Fatal(err)
	}

	assertLock(t, confdir, lockFile1, true)
	assertLock(t, confdir, lockFile2, true)

	if err := lock1.Close(); err != nil {
		t.Fatal(err)
	}

	assertLock(t, confdir, lockFile1, false)
	assertLock(t, confdir, lockFile2, true)

	if err := lock2.Close(); err != nil {
		t.Fatal(err)
	}

	assertLock(t, confdir, lockFile1, false)
	assertLock(t, confdir, lockFile2, false)
}
