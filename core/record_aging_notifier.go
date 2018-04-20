package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/op/go-logging"
)

type recordAgingNotifier struct {
	// PerformTask dependancies
	datastore repo.Datastore
	broadcast chan repo.Notifier

	// Worker-handling dependancies
	intervalDelay time.Duration
	logger        *logging.Logger
	runCount      int
	watchdogTimer *time.Ticker
	stopWorker    chan bool
}

func (n *OpenBazaarNode) StartRecordAgingNotifier() {
	n.RecordAgingNotifier = &recordAgingNotifier{
		datastore:     n.Datastore,
		broadcast:     n.Broadcast,
		intervalDelay: time.Duration(10) * time.Minute,
		logger:        logging.MustGetLogger("recordAgingNotifier"),
	}
	go n.RecordAgingNotifier.Run()
}

func (notifier *recordAgingNotifier) RunCount() int { return notifier.runCount }

func (notifier *recordAgingNotifier) Run() {
	notifier.watchdogTimer = time.NewTicker(notifier.intervalDelay)
	notifier.stopWorker = make(chan bool)

	// Run once on start, then wait for watchdog
	if err := notifier.PerformTask(); err != nil {
		notifier.logger.Error("performTask failure:", err.Error())
	}
	for {
		select {
		case <-notifier.watchdogTimer.C:
			if err := notifier.PerformTask(); err != nil {
				notifier.logger.Error("performTask failure:", err.Error())
			}
		case <-notifier.stopWorker:
			notifier.watchdogTimer.Stop()
			return
		}
	}
}

func (notifier *recordAgingNotifier) Stop() {
	notifier.stopWorker <- true
	close(notifier.stopWorker)
}

func (notifier *recordAgingNotifier) PerformTask() (err error) {
	notifier.runCount += 1
	notifier.logger.Infof("performTask started (count %d)", notifier.runCount)

	if err = notifier.generateModeratorNotifications(); err != nil {
		return
	}
	err = notifier.generateBuyerNotifications()
	return
}

func (notifier *recordAgingNotifier) generateBuyerNotifications() error {
	purchases, err := notifier.datastore.Purchases().GetPurchasesForNotification()
	if err != nil {
		return err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)

		fifteenDays    = time.Duration(15*24) * time.Hour
		fourtyDays     = time.Duration(40*24) * time.Hour
		fourtyFourDays = time.Duration(44*24) * time.Hour
		fourtyFiveDays = time.Duration(45*24) * time.Hour
	)

	for _, p := range purchases {
		var timeSinceCreation = executedAt.Sub(p.Timestamp)
		if p.LastNotifiedAt.Before(p.Timestamp) || p.LastNotifiedAt.Equal(p.Timestamp) {
			notificationsToAdd = append(notificationsToAdd, p.BuildZeroDayNotification(executedAt))
		}
		if p.LastNotifiedAt.Before(p.Timestamp.Add(fifteenDays)) && timeSinceCreation > fifteenDays {
			notificationsToAdd = append(notificationsToAdd, p.BuildFifteenDayNotification(executedAt))
		}
		if p.LastNotifiedAt.Before(p.Timestamp.Add(fourtyDays)) && timeSinceCreation > fourtyDays {
			notificationsToAdd = append(notificationsToAdd, p.BuildFourtyDayNotification(executedAt))
		}
		if p.LastNotifiedAt.Before(p.Timestamp.Add(fourtyFourDays)) && timeSinceCreation > fourtyFourDays {
			notificationsToAdd = append(notificationsToAdd, p.BuildFourtyFourDayNotification(executedAt))
		}
		if p.LastNotifiedAt.Before(p.Timestamp.Add(fourtyFiveDays)) && timeSinceCreation > fourtyFiveDays {
			notificationsToAdd = append(notificationsToAdd, p.BuildFourtyFiveDayNotification(executedAt))
		}
		if len(notificationsToAdd) > 0 {
			p.LastNotifiedAt = executedAt
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return err
	}

	for _, n := range notificationsToAdd {
		var ser, err = json.Marshal(n.NotifierData)
		if err != nil {
			notifier.logger.Warning("marshaling purchase dispute notification:", err.Error())
			notifier.logger.Infof("failed marshal: %+v", n)
			continue
		}
		var template = "insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)"
		_, err = notificationTx.Exec(template, n.GetID(), string(ser), strings.ToLower(n.GetType()), n.GetUnixCreatedAt(), 0)
		if err != nil {
			notifier.logger.Warning("inserting purchase dispute notification:", err.Error())
			notifier.logger.Infof("failed insert: %+v", n)
			continue
		}
	}

	if err = notificationTx.Commit(); err != nil {
		if rollbackErr := notificationTx.Rollback(); rollbackErr != nil {
			err = fmt.Errorf(err.Error(), "\nand also failed during rollback:", rollbackErr.Error())
		}
		return fmt.Errorf("commiting purchase dispute notifications:", err.Error())
	}
	notifier.logger.Infof("created %d purchase dispute notifications", len(notificationsToAdd))
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	err = notifier.datastore.Purchases().UpdatePurchasesLastNotifiedAt(purchases)
	notifier.logger.Infof("updated lastNotifiedAt on %d purchases", len(purchases))
	return nil
}

func (notifier *recordAgingNotifier) generateModeratorNotifications() error {
	disputes, err := notifier.datastore.Cases().GetDisputesForNotification()
	if err != nil {
		return err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)

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

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return err
	}

	for _, n := range notificationsToAdd {
		var ser, err = json.Marshal(n.NotifierData)
		if err != nil {
			notifier.logger.Warning("marshaling dispute expiration notification:", err.Error())
			notifier.logger.Infof("failed marshal: %+v", n)
			continue
		}
		var template = "insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)"
		_, err = notificationTx.Exec(template, n.GetID(), string(ser), strings.ToLower(n.GetType()), n.GetUnixCreatedAt(), 0)
		if err != nil {
			notifier.logger.Warning("inserting dispute expiration notification:", err.Error())
			notifier.logger.Infof("failed insert: %+v", n)
			continue
		}
	}

	if err = notificationTx.Commit(); err != nil {
		if rollbackErr := notificationTx.Rollback(); rollbackErr != nil {
			err = fmt.Errorf(err.Error(), "\nand also failed during rollback:", rollbackErr.Error())
		}
		return fmt.Errorf("commiting dispute expiration notifications:", err.Error())
	}
	notifier.logger.Infof("created %d dispute expiration notifications", len(notificationsToAdd))
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	err = notifier.datastore.Cases().UpdateDisputesLastNotifiedAt(disputes)
	notifier.logger.Infof("updated lastNotifiedAt on %d disputes", len(disputes))
	return nil
}
