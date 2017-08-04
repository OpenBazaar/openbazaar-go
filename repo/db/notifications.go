package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	notif "github.com/OpenBazaar/openbazaar-go/api/notifications"
	"strconv"
	"strings"
	"sync"
	"time"
)

type NotficationsDB struct {
	db   *sql.DB
	lock sync.RWMutex
}

func (n *NotficationsDB) Put(notifID string, notification notif.Data, notifType string, timestamp time.Time) error {
	ser, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	n.lock.Lock()
	defer n.lock.Unlock()
	_, err = n.db.Exec("insert into notifications(serializedNotification, type, timestamp, read) values(?,?,?,?)", string(ser), strings.ToLower(notifType), int(timestamp.Unix()), 0)
	if err != nil {
		return err
	}
	return nil
}

func (n *NotficationsDB) GetAll(offsetId string, limit int, typeFilter []string) ([]notif.Notification, int, error) {
	var ret []notif.Notification

	n.lock.RLock()
	defer n.lock.RUnlock()

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

	var args []interface{}
	if offsetId != "" {
		args = append(args, offsetId)
		if len(types) > 0 {
			filter = " and " + typeFilterClause
			for _, a := range types {
				args = append(args, a)
			}
		}
		stm = "select serializedNotification, timestamp, read from notifications where timestamp<(select timestamp from notifications where notifID=?)" + filter + " order by timestamp desc limit " + strconv.Itoa(limit) + ";"
		cstm = "select Count(*) from notifications where timestamp<(select timestamp from notifications where notifID=?)" + filter + " order by timestamp desc;"
	} else {
		if len(types) > 0 {
			filter = " where " + typeFilterClause
			for _, a := range types {
				args = append(args, a)
			}
		}
		stm = "select serializedNotification, timestamp, read from notifications" + filter + " order by timestamp desc limit " + strconv.Itoa(limit) + ";"
		cstm = "select Count(*) from notifications" + filter + " order by timestamp desc;"
	}
	rows, err := n.db.Query(stm, args...)
	if err != nil {
		return ret, 0, err
	}
	for rows.Next() {
		var data []byte
		var timestampInt int
		var readInt int
		if err := rows.Scan(&data, &timestampInt, &readInt); err != nil {
			fmt.Println(err)
			continue
		}
		var read bool
		if readInt == 1 {
			read = true
		}
		timestamp := time.Unix(int64(timestampInt), 0)
		var ni interface{}
		err := json.Unmarshal(data, &ni)
		if err != nil {
			fmt.Println(err)
			continue
		}
		n := notif.Notification{
			Data:      ni,
			Timestamp: timestamp,
			Read:      read,
		}
		ret = append(ret, n)
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
	_, err := n.db.Exec("update notifications set read=1")
	return err
}

func (n *NotficationsDB) Delete(notifID string) error {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.db.Exec("delete from notifications where notifID=?", notifID)
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
