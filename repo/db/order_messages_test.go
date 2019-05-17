package db_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/golang/protobuf/ptypes/any"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
)

func buildNewOrderMessageStore() (repo.OrderMessageStore, func(), error) {
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
	return db.NewOrderMessageStore(database, new(sync.Mutex)), appSchema.DestroySchemaDirectories, nil
}

func TestOrderMessageDB_Put(t *testing.T) {
	var (
		messagesdb, teardown, err = buildNewOrderMessageStore()
		orderID                   = "orderID1"
		mType                     = pb.Message_ORDER
		payload                   = "sample message"
		peerID                    = "jack"
	)
	if err != nil {
		t.Fatal(err)
	}
	defer teardown()

	msg := pb.Message{
		MessageType: mType,
		Payload:     &any.Any{Value: []byte(payload)},
	}

	err = messagesdb.Put(orderID, mType, peerID, msg)
	if err != nil {
		t.Error(err)
	}

	retMsg, peer, err := messagesdb.GetByOrderIDType(orderID, mType)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(string(retMsg.Payload.Value), "  ", peer)

	if !(string(retMsg.Payload.Value) == payload) {
		t.Error("incorrect payload")
	}

	if !(peer == peerID) {
		t.Error("incorrect peerID")
	}
}
