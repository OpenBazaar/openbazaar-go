package db

import (
	"database/sql"
	"fmt"
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

	// timestamp.UnixNano() is undefined when time has a zero value
	if timestamp.IsZero() {
		log.Warningf("putting chat message (%s): recieved zero timestamp, using current time", messageId)
		timestamp = time.Now()
	}

	stm := `insert into chat(messageID, peerID, subject, message, read, timestamp, outgoing) values(?,?,?,?,?,?,?)`
	stmt, err := c.PrepareQuery(stm)
	if err != nil {
		return fmt.Errorf("prepare chat sql: %s", err.Error())
	}
	defer stmt.Close()

	readInt := 0
	if read {
		readInt = 1
	}

	outgoingInt := 0
	if outgoing {
		outgoingInt = 1
	}

	_, err = stmt.Exec(
		messageId,
		peerId,
		subject,
		message,
		readInt,
		int(timestamp.UnixNano()),
		outgoingInt,
	)
	if err != nil {
		return fmt.Errorf("commit chat: %s", err.Error())
	}
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
		var (
			count  int
			m      string
			ts     int64
			outInt int
			stm    = "select Count(*) from chat where peerID='" + peerId + "' and read=0 and subject='' and outgoing=0;"
			row    = c.db.QueryRow(stm)
		)
		err = row.Scan(&count)
		if err != nil {
			log.Error(err)
		}
		stm = "select max(timestamp), message, outgoing from chat where peerID='" + peerId + "' and subject=''"
		row = c.db.QueryRow(stm)
		err = row.Scan(&ts, &m, &outInt)
		if err != nil {
			log.Error(err)
		}
		outgoing := false
		if outInt > 0 {
			outgoing = true
		}
		timestamp := repo.NewAPITime(time.Unix(0, ts))
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
		var (
			msgID        string
			pid          string
			message      string
			readInt      int
			timestampInt int64
			outgoingInt  int
		)
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
		timestamp := repo.NewAPITime(time.Unix(0, timestampInt))
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

	var (
		peerStm, messageStm string
		updateArgs          = []interface{}{subject, outgoingInt}
	)
	if peerID != "" {
		peerStm = " and peerID=?"
		updateArgs = append(updateArgs, peerID)
	}
	if messageId != "" {
		messageStm = " and timestamp<=(select timestamp from chat where messageID=?)"
		updateArgs = append(updateArgs, messageId)
	}

	result, err := c.db.Exec("update chat set read=1 where subject=? and outgoing=?"+peerStm+messageStm, updateArgs...)
	if err != nil {
		return "", false, fmt.Errorf("mark chat as read: %s", err)
	}
	if count, err := result.RowsAffected(); err != nil {
		log.Error("mark chat as read: unable to determine rows affected, assuming not updated")
	} else {
		if count > 0 {
			updated = true
		}
	}

	// get last message ID
	stmt2, err := c.db.Prepare("select max(timestamp), messageID from chat where subject=?" + peerStm + " and outgoing=?")
	if err != nil {
		return "", updated, fmt.Errorf("prepare get last message id sql: %s", err.Error())
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
		return "", updated, fmt.Errorf("query get last message id: %s", err.Error())
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
	_, err := c.db.Exec("delete from chat where messageID=?", msgID)
	if err != nil {
		log.Error(err)
	}
	return nil
}

func (c *ChatDB) DeleteConversation(peerId string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	_, err := c.db.Exec("delete from chat where peerId=? and subject=''", peerId)
	log.Error(err)
	return nil
}
