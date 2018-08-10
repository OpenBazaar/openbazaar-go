package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/op/go-logging"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

const (
	notifierTestingInterval = time.Duration(1) * time.Minute
	notifierRegularInterval = time.Duration(10) * time.Minute
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

// StartRecordAgingNotifier - start the notifier
func (n *OpenBazaarNode) StartRecordAgingNotifier() {
	n.RecordAgingNotifier = &recordAgingNotifier{
		datastore:     n.Datastore,
		broadcast:     n.Broadcast,
		intervalDelay: n.intervalDelay(),
		logger:        logging.MustGetLogger("recordAgingNotifier"),
	}
	go n.RecordAgingNotifier.Run()
}

func (n *OpenBazaarNode) intervalDelay() time.Duration {
	if n.TestnetEnable {
		return notifierTestingInterval
	}
	return notifierRegularInterval
}

func (notifier *recordAgingNotifier) RunCount() int { return notifier.runCount }

func (notifier *recordAgingNotifier) Run() {
	notifier.watchdogTimer = time.NewTicker(notifier.intervalDelay)
	notifier.stopWorker = make(chan bool)

	// Run once on start, then wait for watchdog
	notifier.PerformTask()
	for {
		select {
		case <-notifier.watchdogTimer.C:
			notifier.PerformTask()
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

func (notifier *recordAgingNotifier) PerformTask() {
	notifier.runCount++ // += 1
	notifier.logger.Debugf("performTask started (count %d)", notifier.runCount)

	if err := notifier.generateSellerDisputeNotifications(); err != nil {
		notifier.logger.Errorf("generateSellerDisputeNotifications failed: %s", err)
	}
	if err := notifier.generateBuyerDisputeTimeoutNotifications(); err != nil {
		notifier.logger.Errorf("generateBuyerDisputeTimeoutNotifications failed: %s", err)
	}
	if err := notifier.generateBuyerDisputeExpiryNotifications(); err != nil {
		notifier.logger.Errorf("generateBuyerDisputeExpiryNotifications failed: %s", err)
	}
	if err := notifier.generateModeratorDisputeExpiryNotifications(); err != nil {
		notifier.logger.Errorf("generateModeratorDisputeExpiryNotifications failed: %s", err)
	}
}

func (notifier *recordAgingNotifier) generateSellerDisputeNotifications() error {
	sales, err := notifier.datastore.Sales().GetSalesForDisputeTimeoutNotification()
	if err != nil {
		return err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)
	)

	for _, s := range sales {
		var timeSinceCreation = executedAt.Sub(s.Timestamp)
		if s.LastDisputeTimeoutNotifiedAt.Before(s.Timestamp.Add(repo.VendorDisputeTimeout_lastInterval)) && timeSinceCreation > repo.VendorDisputeTimeout_lastInterval {
			notificationsToAdd = append(notificationsToAdd, s.BuildVendorDisputeTimeoutLastNotification(executedAt))
		}
		if len(notificationsToAdd) > 0 {
			s.LastDisputeTimeoutNotifiedAt = executedAt
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return err
	}

	for _, n := range notificationsToAdd {
		var ser, err = n.MarshalJSON()
		if err != nil {
			notifier.logger.Warning("marshaling vendor dispute notification:", err.Error())
			notifier.logger.Debugf("failed marshal: %+v", n)
			continue
		}
		var template = "insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)"
		_, err = notificationTx.Exec(template, n.GetID(), string(ser), strings.ToLower(n.GetTypeString()), n.GetUnixCreatedAt(), 0)
		if err != nil {
			notifier.logger.Warning("inserting vendor dispute notification:", err.Error())
			notifier.logger.Debugf("failed insert: %+v", n)
			continue
		}
	}

	if err = notificationTx.Commit(); err != nil {
		if rollbackErr := notificationTx.Rollback(); rollbackErr != nil {
			err = fmt.Errorf("%s %s %s", err.Error(), "\nand also failed during rollback:", rollbackErr.Error())
		}
		return fmt.Errorf("commiting vendor dispute notifications: %s", err.Error())
	}
	notifier.logger.Debugf("created %d vendor dispute notifications", len(notificationsToAdd))
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	if err = notifier.datastore.Sales().UpdateSalesLastDisputeTimeoutNotifiedAt(sales); err != nil {
		return fmt.Errorf("update sales disputeTimeoutNotifiedAt: %s", err.Error())
	}
	notifier.logger.Debugf("updated lastDisputeTimeoutNotifiedAt on %d sales", len(sales))
	return nil
}

func (notifier *recordAgingNotifier) generateBuyerDisputeTimeoutNotifications() error {
	purchases, err := notifier.datastore.Purchases().GetPurchasesForDisputeTimeoutNotification()
	if err != nil {
		return err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)
	)

	for _, p := range purchases {
		var timeSinceCreation = executedAt.Sub(p.Timestamp)
		// Extra seconds added to creation time is a hack to order SQL results
		if p.LastDisputeTimeoutNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeTimeout_firstInterval)) && timeSinceCreation > repo.BuyerDisputeTimeout_firstInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeTimeoutFirstNotification(executedAt))
		}
		if p.LastDisputeTimeoutNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeTimeout_secondInterval)) && timeSinceCreation > repo.BuyerDisputeTimeout_secondInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeTimeoutSecondNotification(executedAt))
		}
		if p.LastDisputeTimeoutNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeTimeout_thirdInterval)) && timeSinceCreation > repo.BuyerDisputeTimeout_thirdInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeTimeoutThirdNotification(executedAt))
		}
		if p.LastDisputeTimeoutNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeTimeout_lastInterval)) && timeSinceCreation > repo.BuyerDisputeTimeout_lastInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeTimeoutLastNotification(executedAt))
		}
		if len(notificationsToAdd) > 0 {
			p.LastDisputeTimeoutNotifiedAt = executedAt
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return err
	}

	for _, n := range notificationsToAdd {
		var ser, err = n.MarshalJSON()
		if err != nil {
			notifier.logger.Warning("marshaling purchase dispute notification:", err.Error())
			notifier.logger.Debugf("failed marshal: %+v", n)
			continue
		}
		var template = "insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)"
		_, err = notificationTx.Exec(template, n.GetID(), string(ser), strings.ToLower(n.GetTypeString()), n.GetUnixCreatedAt(), 0)
		if err != nil {
			notifier.logger.Warning("inserting purchase dispute notification:", err.Error())
			notifier.logger.Debugf("failed insert: %+v", n)
			continue
		}
	}

	if err = notificationTx.Commit(); err != nil {
		if rollbackErr := notificationTx.Rollback(); rollbackErr != nil {
			err = fmt.Errorf(err.Error(), "\nand also failed during rollback:", rollbackErr.Error())
		}
		return fmt.Errorf("commiting purchase dispute notifications: %s", err.Error())
	}
	notifier.logger.Debugf("created %d purchase dispute notifications", len(notificationsToAdd))
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	if err = notifier.datastore.Purchases().UpdatePurchasesLastDisputeTimeoutNotifiedAt(purchases); err != nil {
		return fmt.Errorf("updating lastDisputeTimeoutNotifiedAt on purchases: %s", err.Error())
	}
	notifier.logger.Debugf("updated lastDisputeTimeoutNotifiedAt on %d purchases", len(purchases))
	return nil
}

func (notifier *recordAgingNotifier) generateBuyerDisputeExpiryNotifications() error {
	purchases, err := notifier.datastore.Purchases().GetPurchasesForDisputeExpiryNotification()
	if err != nil {
		return err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)
	)

	for _, p := range purchases {
		var timeSinceDisputedAt = executedAt.Sub(p.DisputedAt)
		// Extra seconds added to creation time is a hack to order SQL results
		if p.LastDisputeExpiryNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeExpiry_firstInterval)) && timeSinceDisputedAt > repo.BuyerDisputeExpiry_firstInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeExpiryFirstNotification(executedAt))
		}
		if p.LastDisputeExpiryNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeExpiry_secondInterval)) && timeSinceDisputedAt > repo.BuyerDisputeExpiry_secondInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeExpirySecondNotification(executedAt))
		}
		if p.LastDisputeExpiryNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeExpiry_lastInterval)) && timeSinceDisputedAt > repo.BuyerDisputeExpiry_lastInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeExpiryLastNotification(executedAt))
		}
		// TODO: Check if THIS purchase made a notification before bumping timestamp
		if len(notificationsToAdd) > 0 {
			p.LastDisputeExpiryNotifiedAt = executedAt
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return err
	}

	for _, n := range notificationsToAdd {
		var ser, err = n.MarshalJSON()
		if err != nil {
			notifier.logger.Warning("marshaling buyer expiration notification:", err.Error())
			notifier.logger.Debugf("failed marshal: %+v", n)
			continue
		}
		var template = "insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)"
		_, err = notificationTx.Exec(template, n.GetID(), string(ser), strings.ToLower(n.GetTypeString()), n.GetUnixCreatedAt(), 0)
		if err != nil {
			notifier.logger.Warning("inserting buyer expiration notification:", err.Error())
			notifier.logger.Debugf("failed insert: %+v", n)
			continue
		}
	}

	if err = notificationTx.Commit(); err != nil {
		if rollbackErr := notificationTx.Rollback(); rollbackErr != nil {
			err = fmt.Errorf(err.Error(), "\nand also failed during rollback:", rollbackErr.Error())
		}
		return fmt.Errorf("commiting buyer expiration notifications: %s", err.Error())
	}
	notifier.logger.Debugf("created %d buyer expiration notifications", len(notificationsToAdd))
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	if err = notifier.datastore.Purchases().UpdatePurchasesLastDisputeExpiryNotifiedAt(purchases); err != nil {
		return fmt.Errorf("updating lastDisputeExpiryNotifiedAt on purchases: %s", err.Error())
	}
	notifier.logger.Debugf("updated lastDisputeExpiryNotifiedAt on %d purchases", len(purchases))
	return nil
}

func (notifier *recordAgingNotifier) generateModeratorDisputeExpiryNotifications() error {
	disputes, err := notifier.datastore.Cases().GetDisputesForDisputeExpiryNotification()
	if err != nil {
		return err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)
	)

	for _, d := range disputes {
		var timeSinceCreation = executedAt.Sub(d.Timestamp)
		// Extra seconds added to creation time is a hack to order SQL results
		if d.LastDisputeExpiryNotifiedAt.Before(d.Timestamp.Add(repo.ModeratorDisputeExpiry_firstInterval)) && timeSinceCreation > repo.ModeratorDisputeExpiry_firstInterval {
			notificationsToAdd = append(notificationsToAdd, d.BuildModeratorDisputeExpiryFirstNotification(executedAt))
		}
		if d.LastDisputeExpiryNotifiedAt.Before(d.Timestamp.Add(repo.ModeratorDisputeExpiry_secondInterval)) && timeSinceCreation > repo.ModeratorDisputeExpiry_secondInterval {
			notificationsToAdd = append(notificationsToAdd, d.BuildModeratorDisputeExpirySecondNotification(executedAt))
		}
		if d.LastDisputeExpiryNotifiedAt.Before(d.Timestamp.Add(repo.ModeratorDisputeExpiry_thirdInterval)) && timeSinceCreation > repo.ModeratorDisputeExpiry_thirdInterval {
			notificationsToAdd = append(notificationsToAdd, d.BuildModeratorDisputeExpiryThirdNotification(executedAt))
		}
		if d.LastDisputeExpiryNotifiedAt.Before(d.Timestamp.Add(repo.ModeratorDisputeExpiry_lastInterval)) && timeSinceCreation > repo.ModeratorDisputeExpiry_lastInterval {
			notificationsToAdd = append(notificationsToAdd, d.BuildModeratorDisputeExpiryLastNotification(executedAt))
		}
		if len(notificationsToAdd) > 0 {
			d.LastDisputeExpiryNotifiedAt = executedAt
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return err
	}

	for _, n := range notificationsToAdd {
		var ser, err = n.MarshalJSON()
		if err != nil {
			notifier.logger.Warning("marshaling dispute expiration notification:", err.Error())
			notifier.logger.Debugf("failed marshal: %+v", n)
			continue
		}
		var template = "insert into notifications(notifID, serializedNotification, type, timestamp, read) values(?,?,?,?,?)"
		_, err = notificationTx.Exec(template, n.GetID(), string(ser), strings.ToLower(n.GetTypeString()), n.GetUnixCreatedAt(), 0)
		if err != nil {
			notifier.logger.Warning("inserting dispute expiration notification:", err.Error())
			notifier.logger.Debugf("failed insert: %+v", n)
			continue
		}
	}

	if err = notificationTx.Commit(); err != nil {
		if rollbackErr := notificationTx.Rollback(); rollbackErr != nil {
			err = fmt.Errorf(err.Error(), "\nand also failed during rollback:", rollbackErr.Error())
		}
		return fmt.Errorf("commiting dispute expiration notifications: %s", err.Error())
	}
	notifier.logger.Debugf("created %d dispute expiration notifications", len(notificationsToAdd))
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	if err = notifier.datastore.Cases().UpdateDisputesLastDisputeExpiryNotifiedAt(disputes); err != nil {
		return fmt.Errorf("updating lastDisputeExpiryNotifiedAt on disputes: %s", err.Error())
	}
	notifier.logger.Debugf("updated lastDisputeExpiryNotifiedAt on %d disputes", len(disputes))
	return nil
}
