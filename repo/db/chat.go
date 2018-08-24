package db

import (
	"database/sql"
	"strconv"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type ChatDB struct {
	modelStore
}

func NewChatStore(db *sql.DB, lock *sync.Mutex) repo.ChatStore {
	return &ChatDB{modelStore{db, lock}}
}

func (c *ChatDB) Put(messageId string, peerId string, subject string, message string, timestamp time.Time, read bool, outgoing bool) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert into chat(messageID, peerID, subject, message, read, timestamp, outgoing) values(?,?,?,?,?,?,?)`
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
	c.lock.Lock()
	defer c.lock.Unlock()
	var ret []repo.ChatConversation

	stm := "select distinct peerID from chat where subject='' order by timestamp desc;"
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
		timestamp := time.Unix(int64(ts), 0)
		convo := repo.ChatConversation{
			PeerId:    peerId,
			Unread:    count,
			Last:      m,
			Timestamp: timestamp,
			Outgoing:  outgoing,
		}
		ret = append(ret, convo)
	}
	return ret
}

func (c *ChatDB) GetMessages(peerID string, subject string, offsetId string, limit int) []repo.ChatMessage {
	c.lock.Lock()
	defer c.lock.Unlock()
	var ret []repo.ChatMessage

	var peerStm string
	if peerID != "" {
		peerStm = " and peerID='" + peerID + "'"
	}

	var stm string
	if offsetId != "" {
		stm = "select messageID, peerID, message, read, timestamp, outgoing from chat where subject='" + subject + "'" + peerStm + " and timestamp<(select timestamp from chat where messageID='" + offsetId + "') order by timestamp desc limit " + strconv.Itoa(limit) + " ;"
	} else {
		stm = "select messageID, peerID, message, read, timestamp, outgoing from chat where subject='" + subject + "'" + peerStm + " order by timestamp desc limit " + strconv.Itoa(limit) + ";"
	}
	rows, err := c.db.Query(stm)
	if err != nil {
		log.Error(err)
		return ret
	}
	for rows.Next() {
		var msgID string
		var pid string
		var message string
		var readInt int
		var timestampInt int
		var outgoingInt int
		if err := rows.Scan(&msgID, &pid, &message, &readInt, &timestampInt, &outgoingInt); err != nil {
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
			PeerId:    pid,
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

func (c *ChatDB) MarkAsRead(peerID string, subject string, outgoing bool, messageId string) (string, bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	updated := false
	outgoingInt := 0
	if outgoing {
		outgoingInt = 1
	}
	var stmt *sql.Stmt
	var tx *sql.Tx
	var err error
	if messageId != "" {
		stm := "select messageID from chat where peerID=? and subject=? and outgoing=? and read=0 and timestamp<=(select timestamp from chat where messageID=?) limit 1"
		rows, err := c.db.Query(stm, peerID, subject, outgoingInt, messageId)
		if err != nil {
			return "", updated, err
		}
		if rows.Next() {
			updated = true
		}
		rows.Close()
		tx, err = c.db.Begin()
		if err != nil {
			return "", updated, err
		}
		stmt, err = tx.Prepare("update chat set read=1 where peerID=? and subject=? and outgoing=? and timestamp<=(select timestamp from chat where messageID=?)")
		if err != nil {
			return "", updated, err
		}
		_, err = stmt.Exec(peerID, subject, outgoingInt, messageId)
		if err != nil {
			return "", updated, err
		}
	} else {
		var peerStm string
		if peerID != "" {
			peerStm = " and peerID=?"
		}

		stm := "select messageID from chat where subject=?" + peerStm + " and outgoing=? and read=0 limit 1"
		var rows *sql.Rows
		var err error
		if peerID != "" {
			rows, err = c.db.Query(stm, subject, peerID, outgoingInt)
		} else {
			rows, err = c.db.Query(stm, subject, outgoingInt)
		}
		if err != nil {
			return "", updated, err
		}
		if rows.Next() {
			updated = true
		}
		rows.Close()
		tx, err = c.db.Begin()
		if err != nil {
			return "", updated, err
		}
		stmt, err = tx.Prepare("update chat set read=1 where subject=?" + peerStm + " and outgoing=?")
		if err != nil {
			return "", updated, err
		}
		if peerID != "" {
			_, err = stmt.Exec(subject, peerID, outgoingInt)
		} else {
			_, err = stmt.Exec(subject, outgoingInt)
		}
		if err != nil {
			return "", updated, err
		}
	}
	defer stmt.Close()
	if err != nil {
		tx.Rollback()
		return "", updated, err
	}
	tx.Commit()

	var peerStm string

	if peerID != "" {
		peerStm = " and peerID=?"
	}
	stmt2, err := c.db.Prepare("select max(timestamp), messageID from chat where subject=?" + peerStm + " and outgoing=?")
	if err != nil {
		return "", updated, err
	}
	defer stmt2.Close()
	var (
		timestamp sql.NullInt64
		msgId     sql.NullString
	)
	if peerID != "" {
		err = stmt2.QueryRow(subject, peerID, outgoingInt).Scan(&timestamp, &msgId)
	} else {
		err = stmt2.QueryRow(subject, outgoingInt).Scan(&timestamp, &msgId)
	}
	if err != nil {
		return "", updated, err
	}
	return msgId.String, updated, nil
}

func (c *ChatDB) GetUnreadCount(subject string) (int, error) {
	stm := "select Count(*) from chat where read=0 and subject=? and outgoing=0;"
	row := c.db.QueryRow(stm, subject)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
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
