package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	// MigrationCreateMessagesAM06MessagesCreateSQL the messages create sql
	MigrationCreateMessagesAM06MessagesCreateSQL = "create table messages (messageID text primary key not null, orderID text, message_type integer, message blob, peerID text, url text, acknowledged bool, tries integer, created_at integer, updated_at integer);"
	// MigrationCreateMessagesAM06CreateIndexMessagesSQLMessageID the messages index on messageID sql
	MigrationCreateMessagesAM06CreateIndexMessagesSQLMessageID = "create index index_messages_messageID on messages (messageID);"
	// MigrationCreateMessagesAM06CreateIndexMessagesSQLOrderIDMType the messages composite index on orderID and messageType create sql
	MigrationCreateMessagesAM06CreateIndexMessagesSQLOrderIDMType = "create index index_messages_orderIDmType on messages (orderID, message_type);"
	// MigrationCreateMessagesAM06CreateIndexMessagesSQLPeerIDMType the messages composite index on peerID and messageType create sql
	MigrationCreateMessagesAM06CreateIndexMessagesSQLPeerIDMType = "create index index_messages_peerIDmType on messages (peerID, message_type);"
	// MigrationCreateMessagesAM06MessagesDeleteSQL the messages delete sql
	MigrationCreateMessagesAM06MessagesDeleteSQL = "drop table if exists messages;"
	// MigrationCreateMessagesAM06DeleteIndexMessagesSQLMessageID delete the messages index on messageID sql
	MigrationCreateMessagesAM06DeleteIndexMessagesSQLMessageID = "drop index if exists index_messages_messageID;"
	// MigrationCreateMessagesAM06DeleteIndexMessagesSQLOrderIDMType delete the messages composite index on orderID and messageType create sql
	MigrationCreateMessagesAM06DeleteIndexMessagesSQLOrderIDMType = "drop index if exists index_messages_orderIDmType;"
	// MigrationCreateMessagesAM06DeleteIndexMessagesSQLPeerIDMType delete the messages composite index on peerID and messageType create sql
	MigrationCreateMessagesAM06DeleteIndexMessagesSQLPeerIDMType = "drop index if exists index_messages_peerIDmType;"
	// MigrationCreateMessagesAM06UpVer set the repo Up version
	MigrationCreateMessagesAM06UpVer = "25"
	// MigrationCreateMessagesAM06DownVer set the repo Down version
	MigrationCreateMessagesAM06DownVer = "24"
)

// MigrationCreateMessagesAM06  local migration struct
type MigrationCreateMessagesAM06 struct{}

// Migration024  migration struct
type Migration024 struct {
	MigrationCreateMessagesAM06
}

func createMessages(repoPath, databasePassword, rVer string, testnetEnabled bool) error {
	var (
		databaseFilePath    string
		repoVersionFilePath = path.Join(repoPath, "repover")
	)
	if testnetEnabled {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}

	db, err := sql.Open("sqlite3", databaseFilePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if databasePassword != "" {
		p := fmt.Sprintf("pragma key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM06MessagesCreateSQL); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Error(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM06CreateIndexMessagesSQLMessageID); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Error(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM06CreateIndexMessagesSQLOrderIDMType); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Error(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM06CreateIndexMessagesSQLPeerIDMType); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Error(err0)
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	// Bump schema version
	err = ioutil.WriteFile(repoVersionFilePath, []byte(rVer), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func deleteMessages(repoPath, databasePassword, rVer string, testnetEnabled bool) error {
	var (
		databaseFilePath    string
		repoVersionFilePath = path.Join(repoPath, "repover")
	)
	if testnetEnabled {
		databaseFilePath = path.Join(repoPath, "datastore", "testnet.db")
	} else {
		databaseFilePath = path.Join(repoPath, "datastore", "mainnet.db")
	}

	db, err := sql.Open("sqlite3", databaseFilePath)
	if err != nil {
		return err
	}
	defer db.Close()
	if databasePassword != "" {
		p := fmt.Sprintf("pragma key = '%s';", databasePassword)
		_, err := db.Exec(p)
		if err != nil {
			return err
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM06DeleteIndexMessagesSQLMessageID); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Error(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM06DeleteIndexMessagesSQLOrderIDMType); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Error(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM06DeleteIndexMessagesSQLPeerIDMType); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Error(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM06MessagesDeleteSQL); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Error(err0)
		}
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	// Bump schema version
	err = ioutil.WriteFile(repoVersionFilePath, []byte(rVer), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

// Up the migration Up code
func (MigrationCreateMessagesAM06) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	return createMessages(repoPath, databasePassword, MigrationCreateMessagesAM06UpVer, testnetEnabled)
}

// Down the migration Down code
func (MigrationCreateMessagesAM06) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	return deleteMessages(repoPath, databasePassword, MigrationCreateMessagesAM06DownVer, testnetEnabled)
}
