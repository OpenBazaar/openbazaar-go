package migrations_test

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo/migrations"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func TestMigration023(t *testing.T) {
	var (
		basePath          = schema.GenerateTempPath()
		testRepoPath, err = schema.OpenbazaarPathTransform(basePath, true)
	)
	if err != nil {
		t.Fatal(err)
	}
	appSchema, err := schema.NewCustomSchemaManager(schema.SchemaContext{DataPath: testRepoPath, TestModeEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()

	var (
		databasePath = appSchema.DatabasePath()
		schemaPath   = appSchema.DataPathJoin("repover")

		schemaSQL          = "pragma key = 'foobarbaz';"
		CreateTableChatSQL = "create table chat (messageID text primary key not null, peerID text, subject text, message text, read integer, timestamp integer, outgoing integer);"
		insertChatSQL      = "insert into chat(messageID, peerID, subject, message, read, timestamp, outgoing) values(?,?,?,?,?,?,?);"
		selectChatSQL      = "select messageID, timestamp from chat;"
		setupSQL           = strings.Join([]string{
			schemaSQL,
			CreateTableChatSQL,
		}, " ")
	)
	fmt.Println("test path:", databasePath)
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(setupSQL); err != nil {
		t.Fatal(err)
	}

	// create chat records
	var examples = []migrations.Migration023_ChatMessage{
		{
			MessageId: "message1",
			PeerId:    "peerid",
			Timestamp: time.Date(2210, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			MessageId: "message2",
			PeerId:    "peerid",
			Timestamp: time.Date(2018, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			MessageId: "message3",
			PeerId:    "peerid",
			Timestamp: time.Date(2010, 1, 2, 3, 4, 5, 0, time.UTC),
		},
	}
	for _, e := range examples {
		_, err = db.Exec(insertChatSQL,
			e.MessageId,
			e.PeerId,
			e.Subject,
			e.Message,
			false,
			e.Timestamp.Unix(),
			false,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	// create schema version file
	if err = ioutil.WriteFile(schemaPath, []byte("22"), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// execute migration up
	m := migrations.Migration023{}
	if err := m.Up(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// assert repo version updated
	if err = appSchema.VerifySchemaVersion("24"); err != nil {
		t.Fatal(err)
	}

	// verify change was applied properly
	chatRows, err := db.Query(selectChatSQL)
	if err != nil {
		t.Fatal(err)
	}

	for chatRows.Next() {
		var (
			messageID    string
			timestampInt int64
		)
		if err := chatRows.Scan(&messageID, &timestampInt); err != nil {
			t.Fatal(err)
		}
		for _, e := range examples {
			if messageID == e.MessageId && timestampInt != e.Timestamp.UnixNano() {
				t.Errorf("expected message (%s) to have nanosecond-based timestamp, but was not", e.MessageId)
			}
		}
	}
	chatRows.Close()

	// execute migration down
	if err := m.Down(testRepoPath, "foobarbaz", true); err != nil {
		t.Fatal(err)
	}

	// assert repo version reverted
	if err = appSchema.VerifySchemaVersion("23"); err != nil {
		t.Fatal(err)
	}

	// verify change was reverted properly
	chatRows, err = db.Query(selectChatSQL)
	if err != nil {
		t.Fatal(err)
	}

	for chatRows.Next() {
		var (
			messageID    string
			timestampInt int64
		)
		if err := chatRows.Scan(&messageID, &timestampInt); err != nil {
			t.Fatal(err)
		}
		for _, e := range examples {
			if messageID == e.MessageId && timestampInt != e.Timestamp.Unix() {
				t.Errorf("expected message (%s) to have second-based timestamp, but did not", e.MessageId)
			}
		}
	}
	chatRows.Close()
}
