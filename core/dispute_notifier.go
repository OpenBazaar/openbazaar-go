package core

import (
	"fmt"
	"time"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

type disputeNotifier struct {
	disputeCasesDB  repo.CaseStore
	notificationsDB repo.NotificationStore
}

func (d *disputeNotifier) PerformTask() error {
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
			// TODO: Log error
			continue
		}
		var template = "insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)"
		_, err = notificationTx.Exec(template, n.GetID(), serializedNotification, n.GetDowncaseType(), n.GetSQLTimestamp(), 0)
		if err != nil {
			// TODO: Log error
			continue
		}
	}

	if err = notificationTx.Commit(); err != nil {
		// TODO: Log error
		if rollbackErr := notificationTx.Rollback(); rollbackErr != nil {
			// TODO: Log error
			err = fmt.Errorf(err.Error(), "\nand also failed during rollback:", rollbackErr.Error())
		}
		return fmt.Errorf("commiting notifications:", err.Error())
	}

	err = d.disputeCasesDB.UpdateDisputesLastNotifiedAt(disputes)
	return nil
}
