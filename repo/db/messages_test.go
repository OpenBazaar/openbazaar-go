package db_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/golang/protobuf/ptypes/any"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	"github.com/OpenBazaar/openbazaar-go/test/factory"
)

func buildNewMessageStore() (repo.MessageStore, func(), error) {
	appSchema := schema.MustNewCustomSchemaManager(schema.SchemaContext{
		DataPath:        schema.GenerateTempPath(),
		TestModeEnabled: true,
	})
	if err := appSchema.BuildSchemaDirectories(); err != nil {
		return nil, nil, err
	}
	if err := appSchema.InitializeDatabase(); err != nil {
		return nil, nil, err
	}
	database, err := appSchema.OpenDatabase()
	if err != nil {
		return nil, nil, err
	}
	return db.NewMessageStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestMessageDB_Put(t *testing.T) {
	SampleErr := errors.New("sample error")
	var (
		messagesdb, teardown, err = buildNewMessageStore()
		orderID                   = "orderID1"
		mType                     = pb.Message_ORDER
		payload                   = "sample message"
		peerID                    = "jack"
		recErr                    = SampleErr.Error()
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	msg := repo.Message{
		Msg: pb.Message{
			MessageType: mType,
			Payload:     &any.Any{Value: []byte(payload)},
		},
	}

	err = messagesdb.Put(fmt.Sprintf("%s-%d", orderID, mType), orderID, mType, peerID, msg, recErr, 0, nil)
	if err != nil {
		t.Error(err)
	}

	retMsg, peer, err := messagesdb.GetByOrderIDType(orderID, mType)
	if err != nil || retMsg == nil {
		t.Error(err)
	}

	if string(retMsg.GetPayload().Value) != payload {
		t.Error("incorrect payload")
	}

	if peer != peerID {
		t.Error("incorrect peerID")
	}
}

func TestMessageDB_MarkAsResolved(t *testing.T) {
	var (
		messagesdb, teardown, err = buildNewMessageStore()
		orderID                   = "orderID1"
		unexpectedOrderID         = "unexpectedOrderID2"
		msg                       = factory.NewMessageWithOrderPayload()
		peerID                    = "QmSomepeerid"
		recErr                    = "error message"
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	err = messagesdb.Put(fmt.Sprintf("%s-%d", orderID, msg.Msg.MessageType), orderID, msg.Msg.MessageType, peerID, msg, recErr, 0, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = messagesdb.Put(fmt.Sprintf("%s-%d", unexpectedOrderID, msg.Msg.MessageType), unexpectedOrderID, msg.Msg.MessageType, peerID, msg, "", 0, nil)
	if err != nil {
		t.Fatal(err)
	}

	erroredMsgs, err := messagesdb.GetAllErrored()
	if err != nil {
		t.Fatal(err)
	}

	if len(erroredMsgs) != 1 {
		t.Errorf("expected one error message, but found (%d)", len(erroredMsgs))
	}

	actual := erroredMsgs[0]
	if actual.PeerID != peerID {
		t.Errorf("expected peerID (%s), but found (%s)", peerID, actual.PeerID)
	}
	if actual.OrderID != orderID {
		t.Errorf("expected orderID (%s), but found (%s)", orderID, actual.OrderID)
	}

	if err := messagesdb.MarkAsResolved(actual); err != nil {
		t.Fatal(err)
	}

	erroredMsgs, err = messagesdb.GetAllErrored()
	if err != nil {
		t.Fatal(err)
	}

	if len(erroredMsgs) != 0 {
		t.Errorf("expected no error messages, but found (%d)", len(erroredMsgs))
	}
}
