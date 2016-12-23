package db

import (
	"database/sql"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"strconv"
	"sync"
	"time"
)

type ChatDB struct {
	db   *sql.DB
	lock *sync.Mutex
}

func (c *ChatDB) Put(peerId string, subject string, message string, read bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into chat(peerID, subject, message, read, timestamp) values(?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}
	readInt := 0
	if read {
		readInt = 1
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		peerId,
		subject,
		message,
		readInt,
		time.Now().Second(),
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *ChatDB) GetConversations() []repo.ChatConversation {
	c.lock.Lock()
	defer c.lock.Unlock()
	var ret []repo.ChatConversation

	stm := "select distinct peerID from chat;"
	rows, err := c.db.Query(stm)
	if err != nil {
		return ret
	}
	var ids []string
	for rows.Next() {
		var peerId string
		if err := rows.Scan(&peerId); err != nil {
			continue
		}
		ids = append(ids, peerId)

	}
	rows.Close()
	defer rows.Close()
	for _, peerId := range ids {
		stm := "select Count(*) from chat where peerID='" + peerId + "' and read=0 and subject='';"
		row := c.db.QueryRow(stm)
		var count int
		row.Scan(&count)
		convo := repo.ChatConversation{
			PeerId: peerId,
			Unread: count,
		}
		ret = append(ret, convo)
	}
	return ret
}

func (c *ChatDB) GetMessages(peerID string, subject string, offsetId int, limit int) []repo.ChatMessage {
	c.lock.Lock()
	defer c.lock.Unlock()
	var ret []repo.ChatMessage

	var stm string
	if offsetId > 0 {
		stm = "select rowid, message, read, timestamp from chat where timestamp<(select timestamp from chat where rowid=" + strconv.Itoa(offsetId) + ") order by timestamp desc limit " + strconv.Itoa(limit) + " ;"
	} else {
		stm = "select rowid, message, read, timestamp from chat order by timestamp desc limit " + strconv.Itoa(limit) + ";"
	}

	rows, err := c.db.Query(stm)
	if err != nil {
		return ret
	}
	for rows.Next() {
		var msgID int
		var message string
		var readInt int
		var timestampInt int
		if err := rows.Scan(&msgID, &message, &readInt, &timestampInt); err != nil {
			continue
		}
		var read bool
		if readInt == 1 {
			read = true
		}
		timestamp := time.Unix(int64(timestampInt), 0)
		chatMessage := repo.ChatMessage{
			PeerId:    peerID,
			MessageId: msgID,
			Subject:   subject,
			Message:   message,
			Read:      read,
			Timestamp: timestamp,
		}
		ret = append(ret, chatMessage)
	}
	return ret
}

func (c *ChatDB) MarkAsRead(msgId int) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stmt, _ := tx.Prepare("update chat set read=1 where rowid=?")

	defer stmt.Close()
	_, err = stmt.Exec(msgId)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *ChatDB) DeleteMessage(msgID int) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.db.Exec("delete from chat where rowid=?", msgID)
	return nil
}

func (c *ChatDB) DeleteConversation(peerId string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.db.Exec("delete from chat where peerId=? and subject=''", peerId)
	return nil
}
