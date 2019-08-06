package db

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

// MessagesDB represents the messages table
type MessagesDB struct {
	modelStore
}

// NewMessageStore return new MessagesDB
func NewMessageStore(db *sql.DB, lock *sync.Mutex) repo.MessageStore {
	return &MessagesDB{modelStore{db, lock}}
}

// Put will insert a record into the messages
func (o *MessagesDB) Put(messageID, orderID string, mType pb.Message_MessageType, peerID string, msg repo.Message) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into messages(messageID, orderID, message_type, message, peerID, created_at) values(?,?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}

	msg0, err := msg.MarshalJSON()
	if err != nil {
		log.Errorf("err marshalling json: %v", err)
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		messageID,
		orderID,
		int(mType),
		msg0,
		peerID,
		int(time.Now().Unix()),
	)
	if err != nil {
		rErr := tx.Rollback()
		if rErr != nil {
			return fmt.Errorf("message put fail: %s and rollback failed: %s", err.Error(), rErr.Error())
		}
		return err
	}

	return tx.Commit()
}

// GetByOrderIDType returns the message for the specified order and message type
func (o *MessagesDB) GetByOrderIDType(orderID string, mType pb.Message_MessageType) (*repo.Message, string, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	var (
		msg0   []byte
		peerID string
	)

	stmt, err := o.db.Prepare("select message, peerID from messages where orderID=? and message_type=?")
	if err != nil {
		return nil, "", err
	}
	err = stmt.QueryRow(orderID, mType).Scan(&msg0, &peerID)
	if err != nil {
		return nil, "", err
	}

	msg := new(repo.Message)

	if len(msg0) > 0 {
		err = msg.UnmarshalJSON(msg0)
		if err != nil {
			return nil, "", err
		}
	}

	return msg, peerID, nil
}
