package api

import (
	"errors"
	"fmt"
	"net/smtp"
	"strings"

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

func manageNotifications(node *core.OpenBazaarNode, out chan []byte) chan repo.Notifier {
	manager := &notificationManager{node: node}
	nodeBroadcast := make(chan repo.Notifier)
	go func() {
		for {
			n := <-nodeBroadcast
			// Fixme: right now this assumes that n is a notification but it should be agnostic
			// enough to let us send any data to the websocket. You can technically do that by
			// sending over a []byte as the serialize function ignores []bytes but it's kind of hacky.
			manager.sendNotification(n)
			data, err := n.WebsocketData()
			if err != nil {
				log.Error("marshal notification:", err)
				continue
			}
			sanitized, err := SanitizeJSON(data)
			if err != nil {
				log.Error("sanitize notification:", err)
				continue
			}
			out <- sanitized
		}
	}()
	return nodeBroadcast
}

type notifier interface {
	notify(n repo.Notifier) error
}

// Send notification via all supported notifier mechanisms
func (m *notificationManager) sendNotification(n repo.Notifier) {
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
	var notifiers []notifier
	if err != nil {
		return notifiers
	}

	// SMTP notifier
	conf := settings.SMTPSettings

	profile, err := m.node.GetProfile()
	if err != nil {
		return nil
	}

	if conf != nil && conf.Notifications {
		conf.OpenBazaarName = profile.Name
		notifiers = append(notifiers, &smtpNotifier{settings: conf})
	}
	return notifiers
}

// Notifier implementations
type smtpNotifier struct {
	settings *repo.SMTPSettings
}

func (notifier *smtpNotifier) notify(n repo.Notifier) error {
	template := strings.Join([]string{
		"From: %s",
		"To: %s",
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"Subject: [OpenBazaar - %s] %s\r\n",
		"%s\r\n",
	}, "\r\n")
	head, body, ok := n.GetSMTPTitleAndBody()
	if !ok {
		return nil
	}
	conf := notifier.settings
	data := fmt.Sprintf(template, conf.SenderEmail, conf.RecipientEmail, conf.OpenBazaarName, head, body)
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
