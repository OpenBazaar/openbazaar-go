package api

import (
	"fmt"
	"net/smtp"
	"strings"

	"errors"
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
			// Fixme: right now this assumes that n is a notification but it should be agnostic
			// enough to let us send any data to the websocket. You can technically do that by
			// sending over a []byte as the serialize function ignores []bytes but it's kind of hacky.
			manager.sendNotification(n)
			sanitized, err := SanitizeJSON(notifications.Serialize(n))
			if err != nil {
				log.Error(err)
				continue
			}
			out <- sanitized
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
			log.Errorf("Notification failed: %s", err.Error())
		}
	}
}

// Create list of notifiers based on settings data (currently only SMTP is available)
// TODO: should be extended to include new notifiers in the list
func (m *notificationManager) getNotifiers() []notifier {
	settings, err := m.node.Datastore.Settings().Get()
	notifiers := []notifier{}
	if err != nil {
		return notifiers
	}

	// SMTP notifier
	conf := settings.SMTPSettings
	if conf != nil && conf.Notifications {
		notifiers = append(notifiers, &smtpNotifier{settings: conf})
	}
	return notifiers
}

// Notifier implementations
type smtpNotifier struct {
	settings *repo.SMTPSettings
}

func (notifier *smtpNotifier) notify(n interface{}) error {
	template := strings.Join([]string{
		"From: %s",
		"To: %s",
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"Subject: [OpenBazaar] %s\r\n",
		"%s\r\n",
	}, "\r\n")
	head, body := notifications.Describe(n)
	if head == "" || body == "" {
		return nil
	}
	conf := notifier.settings
	data := fmt.Sprintf(template, conf.SenderEmail, conf.RecipientEmail, head, body)
	return sendEmail(notifier.settings, []byte(data))
}

// Send email using PLAIN authentication to the server
func sendEmail(conf *repo.SMTPSettings, body []byte) error {
	hostAndPort := strings.Split(conf.ServerAddress, ":")
	host := conf.ServerAddress
	if len(hostAndPort) == 2 {
		host = hostAndPort[0]
	}
	auth := smtp.PlainAuth("", conf.Username, conf.Password, host)
	recipients := []string{conf.RecipientEmail}
	return smtp.SendMail(conf.ServerAddress, auth, conf.SenderEmail, recipients, body)
}

func validateSMTPSettings(s repo.SettingsData) error {
	if s.SMTPSettings != nil && s.SMTPSettings.Notifications &&
		(s.SMTPSettings.Password == "" || s.SMTPSettings.Username == "" || s.SMTPSettings.RecipientEmail == "" || s.SMTPSettings.SenderEmail == "" || s.SMTPSettings.ServerAddress == "") {
		return errors.New("SMTP fields must be set if notifications are turned on")
	}
	return nil
}
