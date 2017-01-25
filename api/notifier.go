package api

import (
	"net/smtp"

	"github.com/OpenBazaar/openbazaar-go/api/notifications"
	"github.com/OpenBazaar/openbazaar-go/core"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

// Notification manager intercepts data form 'inChan' which is embedded
// in different parts of the system and retransmits to the 'outChan',
// which is listened by websocket API, while adding specific handling for
// each received object.
type notificationManager struct {
	node *core.OpenBazaarNode
}

func manageNotifications(node *core.OpenBazaarNode, out chan []byte) chan interface{} {
	manager := &notificationManager{node: node}
	nodeBroadcast := make(chan interface{})
	go func() {
		for {
			n := <-nodeBroadcast
			manager.sendNotification(n)
			out <- notifications.Serialize(n)
		}
	}()
	return nodeBroadcast
}

type notifier interface {
	notify(n interface{}) error
}

// Send notification via all supported notifier mechanisms
func (m *notificationManager) sendNotification(n interface{}) {
	for _, notifier := range m.getNotifiers() {
		if err := notifier.notify(n); err != nil {
			log.Errorf("Notification failed")
		}
	}
}

// Create list of notifiers based on settings data (currently only SMTP is available)
// TODO: should be extended to include new notifiers in the list
func (m *notificationManager) getNotifiers() []notifier {
	settings, err := m.node.Datastore.Settings().Get()
	notifiers := make([]notifier, 0)
	if err != nil {
		return notifiers
	}

	// SMTP notifier
	conf := settings.SMTPSettings
	if conf.Notifications {
		notifiers = append(notifiers, &smtpNotifier{settings: conf})
	}
	return notifiers
}

// Notifier implementations
type smtpNotifier struct {
	settings *repo.SMTPSettings
}

func (notifier *smtpNotifier) notify(n interface{}) error {
	head, body := notifications.Describe(n)
	subject := "[OpenBazaar] " + head
	return sendEmail(notifier.settings, subject, []byte(body))
}

// Send email using PLAIN authentication to the server
func sendEmail(conf *repo.SMTPSettings, subject string, body []byte) error {
	auth := smtp.PlainAuth("", conf.SenderEmail, conf.Password, conf.ServerAddress)
	return smtp.SendMail(
		conf.ServerAddress+":587",
		auth,
		conf.SenderEmail,
		[]string{conf.RecipientEmail},
		body,
	)
}
