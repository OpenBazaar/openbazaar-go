package api

import (
	"encoding/json"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/OpenBazaar/openbazaar-go/repo"
)

func (i *jsonAPIHandler) POSTTestEmailNotifications(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var settings repo.SMTPSettings
	err := decoder.Decode(&settings)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	notifier := smtpNotifier{&settings}
	err = notifier.notify(repo.TestNotification{})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	SanitizedResponse(w, "{}")
}

func (i *jsonAPIHandler) GETNotifications(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "-1"
	}
	l, err := strconv.Atoi(limit)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	offsetID := r.URL.Query().Get("offsetId")
	filter := r.URL.Query().Get("filter")

	types := strings.Split(filter, ",")
	var filters []string
	for _, t := range types {
		if t != "" {
			filters = append(filters, t)
		}
	}

	type notifData struct {
		Unread        int               `json:"unread"`
		Total         int               `json:"total"`
		Notifications []json.RawMessage `json:"notifications"`
	}
	notifs, total, err := i.node.Datastore.Notifications().GetAll(offsetID, l, filters)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	unread, err := i.node.Datastore.Notifications().GetUnreadCount()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	payload := notifData{unread, total, []json.RawMessage{}}
	for _, n := range notifs {
		data, err := n.Data()
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
		payload.Notifications = append(payload.Notifications, data)
	}
	ret, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	retString := string(ret)
	if strings.Contains(retString, "null") {
		retString = strings.Replace(retString, "null", "[]", -1)
	}
	SanitizedResponse(w, retString)
}

func (i *jsonAPIHandler) POSTMarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
	_, notifID := path.Split(r.URL.Path)
	err := i.node.Datastore.Notifications().MarkAsRead(notifID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) POSTMarkNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
	err := i.node.Datastore.Notifications().MarkAllAsRead()
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETENotification(w http.ResponseWriter, r *http.Request) {
	_, notifID := path.Split(r.URL.Path)
	err := i.node.Datastore.Notifications().Delete(notifID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}
