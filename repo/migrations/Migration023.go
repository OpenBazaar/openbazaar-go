package migrations

import (
	"database/sql"
	"fmt"
	"path"
	"time"
)

type Migration023_ChatMessage struct {
	MessageId string
	PeerId    string
	Subject   string
	Message   string
	Read      bool
	Outgoing  bool
	Timestamp time.Time
}

type Migration023 struct{}

func (Migration023) Up(repoPath, databasePassword string, testnetEnabled bool) error {
	var databaseFilePath string
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

	if err := migrateTimestamp(db, func(in int64) int64 {
		// convert from seconds to nanoseconds
		return time.Unix(in, 0).UnixNano()
	}); err != nil {
		return err
	}

	if err := writeRepoVer(repoPath, 24); err != nil {
		return fmt.Errorf("bumping repover to 24: %s", err.Error())
	}
	return nil
}

func (Migration023) Down(repoPath, databasePassword string, testnetEnabled bool) error {
	var databaseFilePath string
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

	if err := migrateTimestamp(db, func(in int64) int64 {
		// convert from nanoseconds to seconds
		return time.Unix(0, in).Unix()
	}); err != nil {
		return err
	}

	if err := writeRepoVer(repoPath, 23); err != nil {
		return fmt.Errorf("dropping repover to 23: %s", err.Error())
	}
	return nil
}

func migrateTimestamp(db *sql.DB, migrate func(int64) int64) error {
	const (
		selectChatSQL = "select messageID, timestamp from chat;"
		updateChatSQL = "update chat set timestamp=? where messageID=?;"
	)

	chatRows, err := db.Query(selectChatSQL)
	if err != nil {
		return fmt.Errorf("query chat: %s", err.Error())
	}

	var updates = make(map[string]int64)
	for chatRows.Next() {
		var (
			messageID string
			timestamp int64
		)
		if err := chatRows.Scan(&messageID, &timestamp); err != nil {
			return fmt.Errorf("unexpected error scanning message (%s): %s", messageID, err.Error())
		}
		updates[messageID] = migrate(timestamp)
	}
	chatRows.Close()
	for id, newTime := range updates {
		if _, err := db.Exec(updateChatSQL, newTime, id); err != nil {
			return fmt.Errorf("updating record (%s): %s", id, err.Error())
		}
	}
	return nil
}
