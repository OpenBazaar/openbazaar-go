package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	// AM06MessagesCreateSQL - the messages create sql
	AM06MessagesCreateSQL = "create table messages (messageID text primary key not null, orderID text, message_type integer, message blob, peerID text, url text, acknowledged bool, tries integer, created_at integer, updated_at integer);"
	// AM06CreateIndexMessagesSQL1 - the messages index on messageID sql
	AM06CreateIndexMessagesSQL1 = "create index index_messages1 on messages (messageID);"
	// AM06CreateIndexMessagesSQL2 - the messages composite index on orderID and messageType create sql
	AM06CreateIndexMessagesSQL2 = "create index index_messages2 on messages (orderID, message_type);"
	// AM06UpVer - set the repo Up version
	AM06UpVer = "25"
	// AM06DownVer - set the repo Down version
	AM06DownVer = "24"
)

// AM06 - local migration struct
type AM06 struct{}

// Migration024 - migration struct
type Migration024 struct {
	AM06
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
	if _, err = tx.Exec(AM06MessagesCreateSQL); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(AM06CreateIndexMessagesSQL1); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(AM06CreateIndexMessagesSQL2); err != nil {
		tx.Rollback()
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

// Up - the migration Up code
func (AM06) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	return createMessages(repoPath, databasePassword, AM06UpVer, testnetEnabled)
}

// Down - the migration Down code
func (AM06) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	return createMessages(repoPath, databasePassword, AM06DownVer, testnetEnabled)
}
