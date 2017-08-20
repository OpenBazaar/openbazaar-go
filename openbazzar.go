package openbazaar

import (
	"log"
)

// OpenBazaar allows creation an initialization of an openbazaar server
type OpenBazaar struct{}

// HelloWorld tests that the framework can be called from an iOS project
func (o *OpenBazaar) HelloWorld() {
	log.Println("Hello World")
}

/* COMMENTED OUT AS WILL NOT COMPILE DUE TO ERROR
* $make ios_framework
* gomobile bind -target=ios github.com/OpenBazaar/openbazaar-go
* gomobile: go install -pkgdir=/Users/nicj/Developer/go/pkg/gomobile/pkg_darwin_arm -tags ios github.com/OpenBazaar/openbazaar-go failed: exit status 2
* # github.com/OpenBazaar/openbazaar-go/vendor/gx/ipfs/QmeJcz1smiskcJbPRTWpzqnLpD2vYqkNiGHVrckaaWHCLv/go-reuseport/singlepoll
* vendor/gx/ipfs/QmeJcz1smiskcJbPRTWpzqnLpD2vYqkNiGHVrckaaWHCLv/go-reuseport/singlepoll/default.go:21: undefined: poll.New
*
* make: *** [ios_framework] Error 1
*

// Initialize initializes a new openbazaar repository
func (o *OpenBazaar) Initialize(
	password,
	dataDir,
	mnemonic string,
	testNetwork bool,
	forceCreate bool,
	walletCreationDate string) error {

	creationDate := time.Now()
	if walletCreationDate != "" {
		var err error
		creationDate, err = time.Parse(time.RFC3339, walletCreationDate)
		if err != nil {
			return errors.New("Wallet creation date timestamp must be in RFC3339 format")
		}
	}

	_, err := initializeRepo(dataDir, password, mnemonic, testNetwork, creationDate)

	return err
}

// duplicated from cli/openbazaard.go
// the cli should probably call into this API
func initializeRepo(
	dataDir,
	password,
	mnemonic string,
	testnet bool,
	creationDate time.Time) (*db.SQLiteDatastore, error) {

	// Database
	sqliteDB, err := db.Create(dataDir, password, testnet)
	if err != nil {
		return sqliteDB, err
	}

	// Initialize the IPFS repo if it does not already exist
	err = repo.DoInit(dataDir, 4096, testnet, password, mnemonic, creationDate, sqliteDB.Config().Init)
	if err != nil {
		return sqliteDB, err
	}
	return sqliteDB, nil
}
*/
