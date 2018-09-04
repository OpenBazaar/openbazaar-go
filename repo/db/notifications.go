package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type NotficationsDB struct {
	modelStore
}

func NewNotificationStore(db *sql.DB, lock *sync.Mutex) repo.NotificationStore {
	return &NotficationsDB{modelStore{db, lock}}
}

func (n *NotficationsDB) PutRecord(record *repo.Notification) error {
	ser, err := json.Marshal(record)
	if err != nil {
		return err
	}

	var read int
	if record.IsRead {
		read = 1
	}

	n.lock.Lock()
	defer n.lock.Unlock()
	_, err = n.ExecuteQuery("insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)", record.GetID(), string(ser), strings.ToLower(record.GetTypeString()), record.GetUnixCreatedAt(), read)
	if err != nil {
		return err
	}
	return nil
}

func (n *NotficationsDB) GetAll(offsetId string, limit int, typeFilter []string) ([]*repo.Notification, int, error) {
	var ret []*repo.Notification

	var stm string
	var cstm string
	var filter string

	typeFilterClause := ""
	var types []string
	if len(typeFilter) > 0 {
		typeFilterClauseParts := make([]string, 0, len(typeFilter))

		for i := 0; i < len(typeFilter); i++ {
			types = append(types, strings.ToLower(typeFilter[i]))
			typeFilterClauseParts = append(typeFilterClauseParts, "?")
		}

		typeFilterClause = "type in (" + strings.Join(typeFilterClauseParts, ",") + ")"
	}

	// Prepare statements
	var args []interface{}
	if offsetId != "" {
		args = append(args, offsetId)
		if len(types) > 0 {
			filter = " and " + typeFilterClause
			for _, a := range types {
				args = append(args, a)
			}
		}
		stm = "select serializedNotification, timestamp, read from notifications where rowid<(select rowid from notifications where notifID=?)" + filter + " order by rowid desc limit " + strconv.Itoa(limit) + ";"
		cstm = "select Count(*) from notifications where timestamp<(select timestamp from notifications where notifID=?)" + filter + " order by rowid desc;"
	} else {
		if len(types) > 0 {
			filter = " where " + typeFilterClause
			for _, a := range types {
				args = append(args, a)
			}
		}
		stm = "select serializedNotification, timestamp, read from notifications" + filter + " order by rowid desc limit " + strconv.Itoa(limit) + ";"
		cstm = "select Count(*) from notifications" + filter + " order by rowid desc;"
	}

	// Gather records
	n.lock.Lock()
	defer n.lock.Unlock()

	rows, err := n.db.Query(stm, args...)
	if err != nil {
		return ret, 0, err
	}
	for rows.Next() {
		var (
			data         []byte
			readInt      int
			timestampInt int
		)
		if err := rows.Scan(&data, &timestampInt, &readInt); err != nil {
			log.Errorf("notifications: GetAll: scanning: %s\n", err.Error())
			continue
		}
		var notification = &repo.Notification{}
		err := json.Unmarshal(data, notification)
		if err != nil {
			log.Errorf("notifications: GetAll: unmarshalling: %s\n", err.Error())
			continue
		}

		// TODO: These should get removed when (*Notification).MarshalJSON begins to include
		// these values. Overriding them here allows for the marshalled representation of
		// the ID field to become out of sync with the DB version of ID, which is overridden
		// here. (Making Notification.NotifierData.GetID() != Notification.GetID())
		var read bool
		if readInt == 1 {
			read = true
		}
		notification.IsRead = read
		notification.CreatedAt = time.Unix(int64(timestampInt), 0).UTC()
		// END

		ret = append(ret, notification)
	}
	row := n.db.QueryRow(cstm, args...)
	var count int
	err = row.Scan(&count)
	if err != nil {
		return ret, 0, err
	}
	return ret, count, nil
}

func (n *NotficationsDB) MarkAsRead(notifID string) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	tx, err := n.db.Begin()
	if err != nil {
		return err
	}
	stmt, _ := tx.Prepare("update notifications set read=1 where notifID=?")

	defer stmt.Close()
	_, err = stmt.Exec(notifID)
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func (n *NotficationsDB) MarkAllAsRead() error {
	n.lock.Lock()
	defer n.lock.Unlock()
	_, err := n.ExecuteQuery("update notifications set read=1")
	return err
}

func (n *NotficationsDB) Delete(notifID string) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	_, err := n.ExecuteQuery("delete from notifications where notifID=?", notifID)
	if err != nil {
		return fmt.Errorf("notifications: delete: %s", err.Error())
	}
	return nil
}

func (n *NotficationsDB) GetUnreadCount() (int, error) {
	stm := "select Count(*) from notifications where read=0;"
	row := n.db.QueryRow(stm)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
