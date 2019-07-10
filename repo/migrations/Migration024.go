package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	// MigrationCreateMessages_AM06MessagesCreateSQL the messages create sql
	MigrationCreateMessages_AM06MessagesCreateSQL = "create table messages (messageID text primary key not null, orderID text, message_type integer, message blob, peerID text, url text, acknowledged bool, tries integer, created_at integer, updated_at integer);"
	// MigrationCreateMessages_AM06CreateIndexMessagesSQL_MessageID the messages index on messageID sql
	MigrationCreateMessages_AM06CreateIndexMessagesSQL_MessageID = "create index index_messages_messageID on messages (messageID);"
	// MigrationCreateMessages_AM06CreateIndexMessagesSQL_OrderID_MType the messages composite index on orderID and messageType create sql
	MigrationCreateMessages_AM06CreateIndexMessagesSQL_OrderID_MType = "create index index_messages_orderIDmType on messages (orderID, message_type);"
	// MigrationCreateMessages_AM06CreateIndexMessagesSQL_PeerID_MType the messages composite index on peerID and messageType create sql
	MigrationCreateMessages_AM06CreateIndexMessagesSQL_PeerID_MType = "create index index_messages_peerIDmType on messages (peerID, message_type);"
	// MigrationCreateMessages_AM06MessagesDeleteSQL the messages delete sql
	MigrationCreateMessages_AM06MessagesDeleteSQL = "drop table if exists messages;"
	// MigrationCreateMessages_AM06CreateIndexMessagesSQL_MessageID delete the messages index on messageID sql
	MigrationCreateMessages_AM06DeleteIndexMessagesSQL_MessageID = "drop index if exists index_messages_messageID;"
	// MigrationCreateMessages_AM06CreateIndexMessagesSQL_OrderID_MType delete the messages composite index on orderID and messageType create sql
	MigrationCreateMessages_AM06DeleteIndexMessagesSQL_OrderID_MType = "drop index if exists index_messages_orderIDmType;"
	// MigrationCreateMessages_AM06CreateIndexMessagesSQL_PeerID_MType delete the messages composite index on peerID and messageType create sql
	MigrationCreateMessages_AM06DeleteIndexMessagesSQL_PeerID_MType = "drop index if exists index_messages_peerIDmType;"
	// MigrationCreateMessages_AM06UpVer set the repo Up version
	MigrationCreateMessages_AM06UpVer = "25"
	// MigrationCreateMessages_AM06DownVer set the repo Down version
	MigrationCreateMessages_AM06DownVer = "24"
)

// MigrationCreateMessages_AM06  local migration struct
type MigrationCreateMessages_AM06 struct{}

// Migration024  migration struct
type Migration024 struct {
	MigrationCreateMessages_AM06
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
	if _, err = tx.Exec(MigrationCreateMessages_AM06MessagesCreateSQL); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessages_AM06CreateIndexMessagesSQL_MessageID); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessages_AM06CreateIndexMessagesSQL_OrderID_MType); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessages_AM06CreateIndexMessagesSQL_PeerID_MType); err != nil {
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
	if _, err = tx.Exec(MigrationCreateMessages_AM06DeleteIndexMessagesSQL_MessageID); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessages_AM06DeleteIndexMessagesSQL_OrderID_MType); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessages_AM06DeleteIndexMessagesSQL_PeerID_MType); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessages_AM06MessagesDeleteSQL); err != nil {
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

// Up the migration Up code
func (MigrationCreateMessages_AM06) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	return createMessages(repoPath, databasePassword, MigrationCreateMessages_AM06UpVer, testnetEnabled)
}

// Down the migration Down code
func (MigrationCreateMessages_AM06) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	return deleteMessages(repoPath, databasePassword, MigrationCreateMessages_AM06DownVer, testnetEnabled)
}
