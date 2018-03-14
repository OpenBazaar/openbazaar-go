package core

import (
	"fmt"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/op/go-logging"
)

type recordAgingNotifier struct {
	// PerformTask dependancies
	disputeCasesDB  repo.CaseStore
	notificationsDB repo.NotificationStore
	broadcast       chan interface{}

	// Worker-handling dependancies
	intervalDelay time.Duration
	logger        *logging.Logger
	runCount      int
	watchdogTimer *time.Ticker
	stopWorker    chan bool
}

func (n *OpenBazaarNode) StartRecordAgingNotifier() {
	n.RecordAgingNotifier = &recordAgingNotifier{
		disputeCasesDB:  n.Datastore.Cases(),
		notificationsDB: n.Datastore.Notifications(),
		broadcast:       n.Broadcast,
		intervalDelay:   time.Duration(10) * time.Minute,
		logger:          logging.MustGetLogger("recordAgingNotifier"),
	}
	go n.RecordAgingNotifier.Run()
}

func (d *recordAgingNotifier) RunCount() int { return d.runCount }

func (d *recordAgingNotifier) Run() {
	d.watchdogTimer = time.NewTicker(d.intervalDelay)
	d.stopWorker = make(chan bool)

	// Run once on start, then wait for watchdog
	if err := d.PerformTask(); err != nil {
		d.logger.Error("performTask failure:", err.Error())
	}
	for {
		select {
		case <-d.watchdogTimer.C:
			if err := d.PerformTask(); err != nil {
				d.logger.Error("performTask failure:", err.Error())
			}
		case <-d.stopWorker:
			d.watchdogTimer.Stop()
			return
		}
	}
}

func (d *recordAgingNotifier) Stop() {
	d.stopWorker <- true
	close(d.stopWorker)
}

func (d *recordAgingNotifier) PerformTask() (err error) {
	d.runCount += 1
	d.logger.Infof("performTask started (count %d)", d.runCount)

	err = d.GenerateModeratorNotifications()
	return
}

func (d *recordAgingNotifier) GenerateModeratorNotifications() error {
	disputes, err := d.disputeCasesDB.GetDisputesForNotification()
	if err != nil {
		return err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.NotificationRecord, 0)

		fifteenDays    = time.Duration(15*24) * time.Hour
		thirtyDays     = time.Duration(30*24) * time.Hour
		fourtyFourDays = time.Duration(44*24) * time.Hour
		fourtyFiveDays = time.Duration(45*24) * time.Hour
	)
	for _, d := range disputes {
		var timeSinceCreation = executedAt.Sub(d.Timestamp)
		if d.LastNotifiedAt.Before(d.Timestamp) || d.LastNotifiedAt.Equal(d.Timestamp) {
			notificationsToAdd = append(notificationsToAdd, d.BuildZeroDayNotification(executedAt))
		}
		if d.LastNotifiedAt.Before(d.Timestamp.Add(fifteenDays)) && timeSinceCreation > fifteenDays {
			notificationsToAdd = append(notificationsToAdd, d.BuildFifteenDayNotification(executedAt))
		}
		if d.LastNotifiedAt.Before(d.Timestamp.Add(thirtyDays)) && timeSinceCreation > thirtyDays {
			notificationsToAdd = append(notificationsToAdd, d.BuildThirtyDayNotification(executedAt))
		}
		if d.LastNotifiedAt.Before(d.Timestamp.Add(fourtyFourDays)) && timeSinceCreation > fourtyFourDays {
			notificationsToAdd = append(notificationsToAdd, d.BuildFourtyFourDayNotification(executedAt))
		}
		if d.LastNotifiedAt.Before(d.Timestamp.Add(fourtyFiveDays)) && timeSinceCreation > fourtyFiveDays {
			notificationsToAdd = append(notificationsToAdd, d.BuildFourtyFiveDayNotification(executedAt))
		}
		if len(notificationsToAdd) > 0 {
			d.LastNotifiedAt = executedAt
		}
	}

	notificationTx, err := d.notificationsDB.BeginTransaction()
	if err != nil {
		return err
	}

	for _, n := range notificationsToAdd {
		var serializedNotification, err = n.MarshalNotificationToJSON()
		if err != nil {
			d.logger.Warning("marshaling dispute expiration notification:", err.Error())
			d.logger.Infof("failed marshal: %+v", n)
			continue
		}
		var template = "insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)"
		_, err = notificationTx.Exec(template, n.GetID(), serializedNotification, n.GetDowncaseType(), n.GetSQLTimestamp(), 0)
		if err != nil {
			d.logger.Warning("inserting dispute expiration notification:", err.Error())
			d.logger.Infof("failed insert: %+v", n)
			continue
		}
	}

	if err = notificationTx.Commit(); err != nil {
		if rollbackErr := notificationTx.Rollback(); rollbackErr != nil {
			err = fmt.Errorf(err.Error(), "\nand also failed during rollback:", rollbackErr.Error())
		}
		return fmt.Errorf("commiting dispute expiration notifications:", err.Error())
	}
	d.logger.Infof("created %d dispute expiration notifications", len(notificationsToAdd))

	for _, n := range notificationsToAdd {
		d.broadcast <- n.Notification
	}

	err = d.disputeCasesDB.UpdateDisputesLastNotifiedAt(disputes)
	d.logger.Infof("updated lastNotifiedAt on %d disputes", len(disputes))
	return nil
}
