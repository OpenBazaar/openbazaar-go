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
	lock sync.RWMutex
}

func (c *ChatDB) Put(messageId string, peerId string, subject string, message string, timestamp time.Time, read bool, outgoing bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into chat(messageID, peerID, subject, message, read, timestamp, outgoing) values(?,?,?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}
	readInt := 0
	if read {
		readInt = 1
	}

	outgoingInt := 0
	if outgoing {
		outgoingInt = 1
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		messageId,
		peerId,
		subject,
		message,
		readInt,
		int(timestamp.Unix()),
		outgoingInt,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (c *ChatDB) GetConversations() []repo.ChatConversation {
	c.lock.RLock()
	defer c.lock.RUnlock()
	var ret []repo.ChatConversation

	stm := "select distinct peerID from chat order by timestamp desc;"
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
	defer rows.Close()
	for _, peerId := range ids {
		stm := "select Count(*) from chat where peerID='" + peerId + "' and read=0 and subject='' and outgoing=0;"
		row := c.db.QueryRow(stm)
		var count int
		row.Scan(&count)
		stm = "select max(timestamp), message, outgoing from chat where peerID='" + peerId + "' and subject=''"
		row = c.db.QueryRow(stm)
		var m string
		var ts int
		var outInt int
		row.Scan(&ts, &m, &outInt)
		outgoing := false
		if outInt > 0 {
			outgoing = true
		}
		convo := repo.ChatConversation{
			PeerId:   peerId,
			Unread:   count,
			Last:     m,
			Outgoing: outgoing,
		}
		ret = append(ret, convo)
	}
	return ret
}

func (c *ChatDB) GetMessages(peerID string, subject string, offsetId string, limit int) []repo.ChatMessage {
	c.lock.RLock()
	defer c.lock.RUnlock()
	var ret []repo.ChatMessage

	var stm string
	if offsetId != "" {
		stm = "select messageID, message, read, timestamp, outgoing from chat where subject='" + subject + "' and peerID='" + peerID + "' and timestamp<(select timestamp from chat where messageID=" + offsetId + ") order by timestamp desc limit " + strconv.Itoa(limit) + " ;"
	} else {
		stm = "select messageID, message, read, timestamp, outgoing from chat where subject='" + subject + "' and peerID='" + peerID + "' order by timestamp desc limit " + strconv.Itoa(limit) + ";"
	}
	rows, err := c.db.Query(stm)
	if err != nil {
		log.Error(err)
		return ret
	}
	for rows.Next() {
		var msgID string
		var message string
		var readInt int
		var timestampInt int
		var outgoingInt int
		if err := rows.Scan(&msgID, &message, &readInt, &timestampInt, &outgoingInt); err != nil {
			continue
		}
		var read bool
		if readInt == 1 {
			read = true
		}
		var outgoing bool
		if outgoingInt == 1 {
			outgoing = true
		}
		timestamp := time.Unix(int64(timestampInt), 0)
		chatMessage := repo.ChatMessage{
			PeerId:    peerID,
			MessageId: msgID,
			Subject:   subject,
			Message:   message,
			Read:      read,
			Timestamp: timestamp,
			Outgoing:  outgoing,
		}
		ret = append(ret, chatMessage)
	}
	return ret
}

func (c *ChatDB) MarkAsRead(peerID string, subject string, outgoing bool, messageId string) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	outgoingInt := 0
	if outgoing {
		outgoingInt = 1
	}
	tx, err := c.db.Begin()
	if err != nil {
		return "", err
	}
	var stmt *sql.Stmt
	if messageId != "" {
		stmt, _ = tx.Prepare("update chat set read=1 where peerID=? and subject=? and outgoing=? and timestamp<=(select timestamp from chat where messageID=?)")
		_, err = stmt.Exec(peerID, subject, outgoingInt, messageId)
	} else {
		stmt, _ = tx.Prepare("update chat set read=1 where peerID=? and subject=? and outgoing=?")
		_, err = stmt.Exec(peerID, subject, outgoingInt)
	}
	defer stmt.Close()
	if err != nil {
		tx.Rollback()
		return "", err
	}
	tx.Commit()

	stmt2, err := c.db.Prepare("select max(timestamp), messageID from chat where peerID=? and subject=? and outgoing=?")
	if err != nil {
		return "", err
	}
	defer stmt2.Close()
	var ts int
	var msgId string
	err = stmt2.QueryRow(peerID, subject, outgoingInt).Scan(&ts, &msgId)
	if err != nil {
		return "", err
	}
	return msgId, nil
}

func (c *ChatDB) DeleteMessage(msgID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.db.Exec("delete from chat where messageID=?", msgID)
	return nil
}

func (c *ChatDB) DeleteConversation(peerId string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.db.Exec("delete from chat where peerId=? and subject=''", peerId)
	return nil
}
