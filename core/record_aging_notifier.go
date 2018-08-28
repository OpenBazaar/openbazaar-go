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
	// PerformTask dependencies
	datastore repo.Datastore
	broadcast chan repo.Notifier

	// Worker-handling dependencies
	intervalDelay time.Duration
	logger        *logging.Logger
	watchdogTimer *time.Ticker
	stopWorker    chan bool
}

type notifierResult struct {
	notificationsMade int
	recordsUpdated    int
	subject           string
}

type notifierSummary map[string]*notifierResult

func (result notifierSummary) Add(operand *notifierResult) {
	if result == nil {
		result = make(map[string]*notifierResult, 0)
	}
	result[operand.subject] = operand
}

func (result notifierSummary) String() string {
	var summaries = make([]string, 0)
	for subject, result := range result {
		summaries = append(summaries, fmt.Sprintf("%s: %d/%d", subject, result.notificationsMade, result.recordsUpdated))
	}
	return strings.Join(summaries, ", ")
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
	var summary = notifierSummary{}

	if result, err := notifier.generateSellerDisputeNotifications(); err != nil {
		notifier.logger.Errorf("generateSellerDisputeNotifications failed: %s", err)
	} else {
		summary.Add(result)
	}
	if result, err := notifier.generateBuyerDisputeTimeoutNotifications(); err != nil {
		notifier.logger.Errorf("generateBuyerDisputeTimeoutNotifications failed: %s", err)
	} else {
		summary.Add(result)
	}
	if result, err := notifier.generateBuyerDisputeExpiryNotifications(); err != nil {
		notifier.logger.Errorf("generateBuyerDisputeExpiryNotifications failed: %s", err)
	} else {
		summary.Add(result)
	}
	if result, err := notifier.generateModeratorDisputeExpiryNotifications(); err != nil {
		notifier.logger.Errorf("generateModeratorDisputeExpiryNotifications failed: %s", err)
	} else {
		summary.Add(result)
	}
	notifier.logger.Debugf("notifications created/records updated: %s", summary.String())
}

func (notifier *recordAgingNotifier) generateSellerDisputeNotifications() (*notifierResult, error) {
	sales, err := notifier.datastore.Sales().GetSalesForDisputeTimeoutNotification()
	if err != nil {
		return nil, err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)
		updatedSales       = make([]*repo.SaleRecord, 0)
	)

	for _, s := range sales {
		var (
			timeSinceCreation = executedAt.Sub(s.Timestamp)
			updated           = false
		)
		if s.LastDisputeTimeoutNotifiedAt.Before(s.Timestamp.Add(repo.VendorDisputeTimeout_lastInterval)) && timeSinceCreation > repo.VendorDisputeTimeout_lastInterval {
			notificationsToAdd = append(notificationsToAdd, s.BuildVendorDisputeTimeoutLastNotification(executedAt))
			updated = true
		}
		if updated {
			s.LastDisputeTimeoutNotifiedAt = executedAt
			updatedSales = append(updatedSales, s)
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("commiting vendor dispute notifications: %s", err.Error())
	}
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	if err = notifier.datastore.Sales().UpdateSalesLastDisputeTimeoutNotifiedAt(updatedSales); err != nil {
		return nil, fmt.Errorf("update sales disputeTimeoutNotifiedAt: %s", err.Error())
	}
	return &notifierResult{len(notificationsToAdd), len(updatedSales), "sales"}, nil
}

func (notifier *recordAgingNotifier) generateBuyerDisputeTimeoutNotifications() (*notifierResult, error) {
	purchases, err := notifier.datastore.Purchases().GetPurchasesForDisputeTimeoutNotification()
	if err != nil {
		return nil, err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)
		updatedPurchases   = make([]*repo.PurchaseRecord, 0)
	)

	for _, p := range purchases {
		var (
			timeSinceCreation = executedAt.Sub(p.Timestamp)
			updated           = false
		)
		if p.LastDisputeTimeoutNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeTimeout_firstInterval)) && timeSinceCreation > repo.BuyerDisputeTimeout_firstInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeTimeoutFirstNotification(executedAt))
			updated = true
		}
		if p.LastDisputeTimeoutNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeTimeout_secondInterval)) && timeSinceCreation > repo.BuyerDisputeTimeout_secondInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeTimeoutSecondNotification(executedAt))
			updated = true
		}
		if p.LastDisputeTimeoutNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeTimeout_thirdInterval)) && timeSinceCreation > repo.BuyerDisputeTimeout_thirdInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeTimeoutThirdNotification(executedAt))
			updated = true
		}
		if p.LastDisputeTimeoutNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeTimeout_lastInterval)) && timeSinceCreation > repo.BuyerDisputeTimeout_lastInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeTimeoutLastNotification(executedAt))
			updated = true
		}
		if updated {
			p.LastDisputeTimeoutNotifiedAt = executedAt
			updatedPurchases = append(updatedPurchases, p)
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("commiting purchase dispute notifications: %s", err.Error())
	}
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	if err = notifier.datastore.Purchases().UpdatePurchasesLastDisputeTimeoutNotifiedAt(updatedPurchases); err != nil {
		return nil, fmt.Errorf("updating lastDisputeTimeoutNotifiedAt on purchases: %s", err.Error())
	}
	return &notifierResult{len(notificationsToAdd), len(updatedPurchases), "purchaseTimeout"}, nil
}

func (notifier *recordAgingNotifier) generateBuyerDisputeExpiryNotifications() (*notifierResult, error) {
	purchases, err := notifier.datastore.Purchases().GetPurchasesForDisputeExpiryNotification()
	if err != nil {
		return nil, err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)
		updatedPurchases   = make([]*repo.PurchaseRecord, 0)
	)

	for _, p := range purchases {
		var (
			timeSinceDisputedAt = executedAt.Sub(p.DisputedAt)
			updated             = false
		)
		if p.LastDisputeExpiryNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeExpiry_firstInterval)) && timeSinceDisputedAt > repo.BuyerDisputeExpiry_firstInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeExpiryFirstNotification(executedAt))
			updated = true
		}
		if p.LastDisputeExpiryNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeExpiry_secondInterval)) && timeSinceDisputedAt > repo.BuyerDisputeExpiry_secondInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeExpirySecondNotification(executedAt))
			updated = true
		}
		if p.LastDisputeExpiryNotifiedAt.Before(p.Timestamp.Add(repo.BuyerDisputeExpiry_lastInterval)) && timeSinceDisputedAt > repo.BuyerDisputeExpiry_lastInterval {
			notificationsToAdd = append(notificationsToAdd, p.BuildBuyerDisputeExpiryLastNotification(executedAt))
			updated = true
		}
		if updated {
			p.LastDisputeExpiryNotifiedAt = executedAt
			updatedPurchases = append(updatedPurchases, p)
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("commiting buyer expiration notifications: %s", err.Error())
	}
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	if err = notifier.datastore.Purchases().UpdatePurchasesLastDisputeExpiryNotifiedAt(updatedPurchases); err != nil {
		return nil, fmt.Errorf("updating lastDisputeExpiryNotifiedAt on purchases: %s", err.Error())
	}
	return &notifierResult{len(notificationsToAdd), len(updatedPurchases), "purchaseExpire"}, nil
}

func (notifier *recordAgingNotifier) generateModeratorDisputeExpiryNotifications() (*notifierResult, error) {
	disputes, err := notifier.datastore.Cases().GetDisputesForDisputeExpiryNotification()
	if err != nil {
		return nil, err
	}

	var (
		executedAt         = time.Now()
		notificationsToAdd = make([]*repo.Notification, 0)
		updatedDisputes    = make([]*repo.DisputeCaseRecord, 0)
	)

	for _, d := range disputes {
		var (
			timeSinceCreation = executedAt.Sub(d.Timestamp)
			updated           = false
		)
		if d.LastDisputeExpiryNotifiedAt.Before(d.Timestamp.Add(repo.ModeratorDisputeExpiry_firstInterval)) && timeSinceCreation > repo.ModeratorDisputeExpiry_firstInterval {
			notificationsToAdd = append(notificationsToAdd, d.BuildModeratorDisputeExpiryFirstNotification(executedAt))
			updated = true
		}
		if d.LastDisputeExpiryNotifiedAt.Before(d.Timestamp.Add(repo.ModeratorDisputeExpiry_secondInterval)) && timeSinceCreation > repo.ModeratorDisputeExpiry_secondInterval {
			notificationsToAdd = append(notificationsToAdd, d.BuildModeratorDisputeExpirySecondNotification(executedAt))
			updated = true
		}
		if d.LastDisputeExpiryNotifiedAt.Before(d.Timestamp.Add(repo.ModeratorDisputeExpiry_thirdInterval)) && timeSinceCreation > repo.ModeratorDisputeExpiry_thirdInterval {
			notificationsToAdd = append(notificationsToAdd, d.BuildModeratorDisputeExpiryThirdNotification(executedAt))
			updated = true
		}
		if d.LastDisputeExpiryNotifiedAt.Before(d.Timestamp.Add(repo.ModeratorDisputeExpiry_lastInterval)) && timeSinceCreation > repo.ModeratorDisputeExpiry_lastInterval {
			notificationsToAdd = append(notificationsToAdd, d.BuildModeratorDisputeExpiryLastNotification(executedAt))
			updated = true
		}
		if updated {
			d.LastDisputeExpiryNotifiedAt = executedAt
			updatedDisputes = append(updatedDisputes, d)
		}
	}

	notifier.datastore.Notifications().Lock()
	notificationTx, err := notifier.datastore.Notifications().BeginTransaction()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("commiting dispute expiration notifications: %s", err.Error())
	}
	notifier.datastore.Notifications().Unlock()

	for _, n := range notificationsToAdd {
		notifier.broadcast <- n.NotifierData
	}

	if err = notifier.datastore.Cases().UpdateDisputesLastDisputeExpiryNotifiedAt(updatedDisputes); err != nil {
		return nil, fmt.Errorf("updating lastDisputeExpiryNotifiedAt on disputes: %s", err.Error())
	}
	return &notifierResult{len(notificationsToAdd), len(updatedDisputes), "dispute"}, nil
}
