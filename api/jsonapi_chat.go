package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/golang/protobuf/ptypes"
)

func (i *jsonAPIHandler) POSTChat(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var chat repo.ChatMessage
	err := decoder.Decode(&chat)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(chat.Subject) > 500 {
		ErrorResponse(w, http.StatusBadRequest, "Subject line is too long")
		return
	}
	if len(chat.Message) > 20000 {
		ErrorResponse(w, http.StatusBadRequest, "Message is too long")
		return
	}

	t := time.Now()
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var flag pb.Chat_Flag
	if chat.Message == "" {
		flag = pb.Chat_TYPING
	} else {
		flag = pb.Chat_MESSAGE
	}
	h := sha256.Sum256([]byte(chat.Message + chat.Subject + ptypes.TimestampString(ts)))
	encoded, err := mh.Encode(h[:], mh.SHA2_256)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	msgID, err := mh.Cast(encoded)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	chatPb := &pb.Chat{
		MessageId: msgID.B58String(),
		Subject:   chat.Subject,
		Message:   chat.Message,
		Timestamp: ts,
		Flag:      flag,
	}
	err = i.node.SendChat(chat.PeerId, chatPb)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Put to database
	if chatPb.Flag == pb.Chat_MESSAGE {
		err = i.node.Datastore.Chat().Put(msgID.B58String(), chat.PeerId, chat.Subject, chat.Message, t, false, true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	SanitizedResponse(w, fmt.Sprintf(`{"messageId": "%s"}`, msgID.B58String()))
}

func (i *jsonAPIHandler) POSTGroupChat(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var chat repo.GroupChatMessage
	err := decoder.Decode(&chat)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(chat.Subject) > 500 {
		ErrorResponse(w, http.StatusBadRequest, "Subject line is too long")
		return
	}
	if len(chat.Subject) <= 0 {
		ErrorResponse(w, http.StatusBadRequest, "Group chats must include a unique subject to be used as the group chat ID")
		return
	}
	if len(chat.Message) > 20000 {
		ErrorResponse(w, http.StatusBadRequest, "Message is too long")
		return
	}

	t := time.Now()
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	var flag pb.Chat_Flag
	if chat.Message == "" {
		flag = pb.Chat_TYPING
	} else {
		flag = pb.Chat_MESSAGE
	}
	h := sha256.Sum256([]byte(chat.Message + chat.Subject + ptypes.TimestampString(ts)))
	encoded, err := mh.Encode(h[:], mh.SHA2_256)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	msgID, err := mh.Cast(encoded)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	chatPb := &pb.Chat{
		MessageId: msgID.B58String(),
		Subject:   chat.Subject,
		Message:   chat.Message,
		Timestamp: ts,
		Flag:      flag,
	}
	for _, pid := range chat.PeerIds {
		err = i.node.SendChat(pid, chatPb)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	// Put to database
	if chatPb.Flag == pb.Chat_MESSAGE {
		err = i.node.Datastore.Chat().Put(msgID.B58String(), "", chat.Subject, chat.Message, t, false, true)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	SanitizedResponse(w, fmt.Sprintf(`{"messageId": "%s"}`, msgID.B58String()))
}

func (i *jsonAPIHandler) GETChatMessages(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	if strings.ToLower(peerID) == "chatmessages" {
		peerID = ""
	}
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
	messages := i.node.Datastore.Chat().GetMessages(peerID, r.URL.Query().Get("subject"), offsetID, l)

	ret, err := json.MarshalIndent(messages, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) GETChatConversations(w http.ResponseWriter, r *http.Request) {
	conversations := i.node.Datastore.Chat().GetConversations()
	ret, err := json.MarshalIndent(conversations, "", "    ")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if isNullJSON(ret) {
		ret = []byte("[]")
	}
	SanitizedResponse(w, string(ret))
}

func (i *jsonAPIHandler) POSTMarkChatAsRead(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	if strings.ToLower(peerID) == "markchatasread" {
		peerID = ""
	}
	subject := r.URL.Query().Get("subject")
	lastID, updated, err := i.node.Datastore.Chat().MarkAsRead(peerID, subject, false, "")
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if updated && peerID != "" {
		chatPb := &pb.Chat{
			MessageId: lastID,
			Subject:   subject,
			Flag:      pb.Chat_READ,
		}
		err = i.node.SendChat(peerID, chatPb)
		if err != nil {
			ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if subject != "" {
		go func() {
			i.node.Datastore.Purchases().MarkAsRead(subject)
			i.node.Datastore.Sales().MarkAsRead(subject)
			i.node.Datastore.Cases().MarkAsRead(subject)
		}()
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETEChatMessage(w http.ResponseWriter, r *http.Request) {
	_, messageID := path.Split(r.URL.Path)
	err := i.node.Datastore.Chat().DeleteMessage(messageID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}

func (i *jsonAPIHandler) DELETEChatConversation(w http.ResponseWriter, r *http.Request) {
	_, peerID := path.Split(r.URL.Path)
	err := i.node.Datastore.Chat().DeleteConversation(peerID)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	SanitizedResponse(w, `{}`)
}
