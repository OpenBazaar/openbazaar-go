package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

type OrderMessagesDB struct {
	modelStore
}

func NewOrderMessageStore(db *sql.DB, lock *sync.Mutex) repo.OrderMessageStore {
	return &OrderMessagesDB{modelStore{db, lock}}
}

func (o *OrderMessagesDB) Put(orderID string, mType pb.Message_MessageType, peerID string, msg pb.Message) error {
	o.lock.Lock()
	defer o.lock.Unlock()

	tx, err := o.db.Begin()
	if err != nil {
		return err
	}
	stm := `insert or replace into order_messages(messageID, orderID, message_type, message, peerID, created_at) values(?,?,?,?,?,?)`
	stmt, err := tx.Prepare(stm)
	if err != nil {
		return err
	}

	msg0, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("err marshaling : ", err)
	}

	defer stmt.Close()
	_, err = stmt.Exec(
		fmt.Sprintf("%s-%d", orderID, int(mType)),
		orderID,
		int(mType),
		msg0,
		peerID,
		int(time.Now().Unix()),
	)
	if err != nil {
		rErr := tx.Rollback()
		if rErr != nil {
			return fmt.Errorf("order_message put fail: %s\nand rollback failed: %s\n", err.Error(), rErr.Error())
		}
		return err
	}

	return tx.Commit()
}

// GetByOrderIDType returns the dispute payout data for a case
func (o *OrderMessagesDB) GetByOrderIDType(orderID string, mType pb.Message_MessageType) (*pb.Message, string, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	var (
		msg0   []byte
		peerID string
	)

	stmt, err := o.db.Prepare("select message, peerID from order_messages where messageID=?")
	if err != nil {
		return nil, "", err
	}
	err = stmt.QueryRow(fmt.Sprintf("%s-%d", orderID, mType)).Scan(&msg0, &peerID)
	if err != nil {
		return nil, "", err
	}

	msg := new(pb.Message)

	if len(msg0) > 0 {
		err = json.Unmarshal(msg0, msg)
		if err != nil {
			return nil, "", err
		}
	}

	return msg, peerID, nil
}
