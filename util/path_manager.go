package util

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mitchellh/go-homedir"
)

type openbazaarSchemaManager struct {
	os              string
	rootPath        string
	testModeEnabled bool
}

// SchemaContext are the parameters which the SchemaManager derive its source of
// truth. When their zero values are provided, a reasonable default will be
// assumed during runtime.
type SchemaContext struct {
	RootPath        string
	TestModeEnabled bool
	OS              string
}

// DefaultPathTransform accepts a string path representing the location where
// application data can be stored and returns a string representing the location
// where OpenBazaar prefers to store its schema on the filesystem relative to that
// path. If the path cannot be transformed, an error will be returned
func OpenbazaarPathTransform(basePath string, testModeEnabled bool) (path string, err error) {
	path, err = homedir.Expand(filepath.Join(basePath, directoryName(testModeEnabled)))
	if err == nil {
		path = filepath.Clean(path)
	}
	return
}

// GenerateTempPath returns a string path representing the location where
// it is okay to store temporary data. No structure or created or deleted as
// part of this operation.
func GenerateTempPath() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return filepath.Join(os.TempDir(), fmt.Sprintf("ob_tempdir_%d", r.Intn(999)))
}

// NewSchemaManager returns a service that handles the root storage directory
// required during runtime. This service also ensures no errors can be produced
// at runtime after initial creation. An error may be produced if the SchemaManager
// is unable to verify the availability of the root storage directory.
func NewSchemaManager() (*openbazaarSchemaManager, error) {
	transformedPath, err := OpenbazaarPathTransform(defaultRootPath(), false)
	if err != nil {
		return nil, err
	}
	return NewCustomSchemaManager(SchemaContext{
		RootPath:        transformedPath,
		TestModeEnabled: false,
		OS:              runtime.GOOS,
	})
}

// NewCustomSchemaManger allows a custom SchemaContext to be provided to change
func NewCustomSchemaManager(ctx SchemaContext) (*openbazaarSchemaManager, error) {
	if len(ctx.RootPath) == 0 {
		path, err := OpenbazaarPathTransform(defaultRootPath(), ctx.TestModeEnabled)
		if err != nil {
			return nil, err
		}
		ctx.RootPath = path
	}
	if len(ctx.OS) == 0 {
		ctx.OS = runtime.GOOS
	}

	return &openbazaarSchemaManager{
		rootPath:        ctx.RootPath,
		testModeEnabled: ctx.TestModeEnabled,
		os:              ctx.OS,
	}, nil
}

func defaultRootPath() (path string) {
	if runtime.GOOS == "darwin" {
		return "~/Library/Application Support"
	}
	return "~"
}

func directoryName(isTestnet bool) (directoryName string) {
	if runtime.GOOS == "linux" {
		directoryName = ".openbazaar2.0"
	} else {
		directoryName = "OpenBazaar2.0"
	}

	if isTestnet {
		directoryName += "-testnet"
	}
	return
}

// RootPath returns the expected location of the root storage directory
func (m *openbazaarSchemaManager) RootPath() string { return m.rootPath }

// DatastorePath returns the expected location of the datastore file
func (m *openbazaarSchemaManager) DatastorePath() string {
	if m.testModeEnabled {
		return m.RootPathJoin("datastore", "testnet.db")
	}
	return m.RootPathJoin("datastore", "mainnet.db")
}

// RootPathJoin is a helper function which joins the pathArgs to the service's
// rootPath and returns the result
func (m *openbazaarSchemaManager) RootPathJoin(pathArgs ...string) string {
	allPathArgs := append([]string{m.rootPath}, pathArgs...)
	return filepath.Join(allPathArgs...)
}

// MustVerifySchemaVersion will ensure that the schema is currently
// the same as the expectedVersion otherwise returning an error. If the
// schema is exactly the same, nil will be returned.
func (m *openbazaarSchemaManager) MustVerifySchemaVersion(expectedVersion string) error {
	schemaVersion, err := ioutil.ReadFile(m.RootPathJoin("repover"))
	if err != nil {
		return fmt.Errorf("Accessing schema version: %s", err.Error())
	}
	if string(schemaVersion) != expectedVersion {
		return fmt.Errorf("Schema does not match expected version %s", expectedVersion)
	}
	return nil
}

func (m *openbazaarSchemaManager) BuildSchemaDirectories() error {
	if err := os.MkdirAll(m.RootPathJoin("datastore"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "listings"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "ratings"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "images"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "images", "tiny"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "images", "small"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "images", "medium"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "images", "large"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "images", "original"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "feed"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "posts"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "channel"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("root", "files"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("outbox"), os.ModePerm); err != nil {
		return err
	}
	if err := os.MkdirAll(m.RootPathJoin("logs"), os.ModePerm); err != nil {
		return err
	}
	return nil
}

func (m *openbazaarSchemaManager) DestroySchemaDirectories() {
	os.RemoveAll(m.rootPath)
}
