package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

const (
	AM06_messagesCreateSQL = "create table order_messages (messageID text primary key not null, orderID text, message_type integer, message blob, peerID text, url text, acknowledged bool, tries integer, created_at integer, updated_at integer);"
	AM06_upVer             = "25"
	AM06_downVer           = "24"
)

type AM06 struct{}

type Migration024 struct {
	AM06
}

func createOrderMessages(repoPath, databasePassword, rVer string, testnetEnabled bool) error {
	fmt.Println("in create order messages")
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
	if _, err = tx.Exec(AM06_messagesCreateSQL); err != nil {
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

func (AM06) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	fmt.Println("in am06 up")
	return createOrderMessages(repoPath, databasePassword, AM06_upVer, testnetEnabled)
}

func (AM06) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	return createOrderMessages(repoPath, databasePassword, AM06_downVer, testnetEnabled)
}
