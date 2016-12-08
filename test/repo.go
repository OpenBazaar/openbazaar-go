package test

import (
	"os"
	"path"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
)

// Repository represents a test (temporary/volitile) repository
type Repository struct {
	Path     string
	Password string
	DB       *db.SQLiteDatastore
}

// NewRepository creates and initializes a new temporary repository for tests
func NewRepository() (*Repository, error) {
	// Create repo object
	r := &Repository{
		Path:     GetRepoPath(),
		Password: GetPassword(),
	}

	// Create database
	var err error
	r.DB, err = db.Create(r.Path, r.Password, true)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// ConfigFile returns the path to the test configuration file
func (r *Repository) ConfigFile() string {
	return path.Join(r.Path, "config")
}

// RemoveSettings purges settings from the database
func (r *Repository) RemoveSettings() error {
	return r.DB.Settings().Delete()
}

// RemoveRepo removes the test repository
func (r *Repository) RemoveRepo() error {
	return deleteDirectory(r.Path)
}

// RemoveRoot removes the profile json from the repository
func (r *Repository) RemoveRoot() error {
	return deleteDirectory(path.Join(r.Path, "root"))
}

// Reset sets the repo state back to a blank slate but retains keys
// Initialize the IPFS repo if it does not already exist
func (r *Repository) Reset() error {
	// Clear old root
	err := r.RemoveRoot()
	if err != nil {
		return err
	}

	// Rebuild any neccessary structure
	err = repo.DoInit(r.Path, 4096, true, r.Password, r.Password, r.DB.Config().Init)
	if err != nil && err != repo.ErrRepoExists {
		return err
	}

	// Remove any settings
	err = r.RemoveSettings()
	if err != nil {
		return err
	}

	// Remove any inventory
	inventory, err := r.DB.Inventory().GetAll()
	if err != nil {
		return err
	}
	for slug := range inventory {
		err := r.DB.Inventory().DeleteAll(slug)
		if err != nil {
			return err
		}
	}

	return nil
}

// MustReset sets the repo state back to a blank slate but retains keys
// It panics upon failure instead of allowing tests to continue
func (r *Repository) MustReset() {
	err := r.Reset()
	if err != nil {
		panic(err)
	}
}

func deleteDirectory(path string) error {
	err := os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
