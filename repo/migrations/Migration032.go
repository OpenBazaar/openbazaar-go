package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	_ "github.com/mutecomm/go-sqlcipher"
)

const (
	// MigrationCreateMessagesAM09MessagesCreateSQL the messages create sql
	MigrationCreateMessagesAM09MessagesCreateSQL = "create table messages (messageID text primary key not null, orderID text, message_type integer, message blob, peerID text, url text, acknowledged bool, tries integer, created_at integer, updated_at integer, err string, received_at integer);"
	// MigrationCreateMessagesAM09MessagesCreateSQLDown the messages create sql
	MigrationCreateMessagesAM09MessagesCreateSQLDown = "create table messages (messageID text primary key not null, orderID text, message_type integer, message blob, peerID text, url text, acknowledged bool, tries integer, created_at integer, updated_at integer);"
	// MigrationRenameMessagesAM09MessagesCreateSQL the messages create sql
	MigrationRenameMessagesAM09MessagesCreateSQL = "ALTER TABLE messages RENAME TO temp_messages"
	// MigrationInsertMessagesAM09Messages the messages create sql
	MigrationInsertMessagesAM09Messages = "INSERT INTO messages SELECT messgeID, orderID, message_type, message, peerID, url, acknowledged, tries, created_at, updated_at FROM temp_messages;"
	// MigrationCreateMessagesAM09MessagesDeleteSQL the messages delete sql
	MigrationCreateMessagesAM09MessagesDeleteSQL = "drop table if exists temp_messages;"
	// MigrationCreateMessagesAM09UpVer set the repo Up version
	MigrationCreateMessagesAM09UpVer = "33"
	// MigrationCreateMessagesAM09DownVer set the repo Down version
	MigrationCreateMessagesAM09DownVer = "32"
)

// MigrationCreateMessagesAM09  local migration struct
type MigrationCreateMessagesAM09 struct{}

// Migration032  migration struct
type Migration032 struct {
	MigrationCreateMessagesAM09
}

func createMessagesAM09(repoPath, databasePassword, rVer string, testnetEnabled bool) error {
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
	if _, err = tx.Exec(MigrationRenameMessagesAM09MessagesCreateSQL); err != nil {
		fmt.Println("11111111111111 : ", err)
		err0 := tx.Rollback()
		if err0 != nil {
			log.Println(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM09MessagesCreateSQL); err != nil {
		fmt.Println("222222222 : ", err)
		err0 := tx.Rollback()
		if err0 != nil {
			log.Println(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationInsertMessagesAM09Messages); err != nil {
		fmt.Println("33333333333 : ", err)
		err0 := tx.Rollback()
		if err0 != nil {
			log.Println(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM09MessagesDeleteSQL); err != nil {
		fmt.Println("444444444444 : ", err)
		err0 := tx.Rollback()
		if err0 != nil {
			log.Println(err0)
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

func deleteMessagesAM09(repoPath, databasePassword, rVer string, testnetEnabled bool) error {
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
	if _, err = tx.Exec(MigrationRenameMessagesAM09MessagesCreateSQL); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Println(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM09MessagesCreateSQLDown); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Println(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationInsertMessagesAM09Messages); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Println(err0)
		}
		return err
	}
	if _, err = tx.Exec(MigrationCreateMessagesAM09MessagesDeleteSQL); err != nil {
		err0 := tx.Rollback()
		if err0 != nil {
			log.Println(err0)
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
func (MigrationCreateMessagesAM09) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	return createMessagesAM09(repoPath, databasePassword, MigrationCreateMessagesAM09UpVer, testnetEnabled)
}

// Down the migration Down code
func (MigrationCreateMessagesAM09) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	return deleteMessagesAM09(repoPath, databasePassword, MigrationCreateMessagesAM09DownVer, testnetEnabled)
}
