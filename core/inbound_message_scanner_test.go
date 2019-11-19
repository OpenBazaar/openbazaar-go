package core

import (
	"errors"
	"sync"
	"testing"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
	"github.com/OpenBazaar/openbazaar-go/repo/db"
	"github.com/OpenBazaar/openbazaar-go/schema"
	wi "github.com/OpenBazaar/wallet-interface"
	"github.com/op/go-logging"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
)

func TestPerformTaskInboundMessageScanner(t *testing.T) {
	var (
		orderMsgWithNoErr = repo.OrderMessage{
			MessageID:   "1",
			OrderID:     "1",
			MessageType: int32(pb.Message_ORDER),
			Message:     []byte("sample message"),
			MsgErr:      "",
			PeerID:      "sample",
			PeerPubkey:  []byte("sample"),
		}

		orderMsgWithErr = repo.OrderMessage{
			MessageID:   "2",
			OrderID:     "2",
			MessageType: int32(pb.Message_ORDER),
			Message:     []byte("sample message"),
			MsgErr:      ErrInsufficientFunds.Error(),
			PeerID:      "sample",
			PeerPubkey:  []byte("sample"),
		}

		orderMsgWithOtherErr = repo.OrderMessage{
			MessageID:   "3",
			OrderID:     "3",
			MessageType: int32(pb.Message_ORDER),
			Message:     []byte("sample message"),
			MsgErr:      "not a retryable err",
			PeerID:      "sample",
			PeerPubkey:  []byte("sample"),
		}

		existingRecords = []repo.OrderMessage{
			orderMsgWithNoErr,
			orderMsgWithErr,
			orderMsgWithOtherErr,
		}

		appSchema = schema.MustNewCustomSchemaManager(schema.SchemaContext{
			DataPath:        schema.GenerateTempPath(),
			TestModeEnabled: true,
		})
	)

	if err := appSchema.BuildSchemaDirectories(); err != nil {
		t.Fatal(err)
	}
	defer appSchema.DestroySchemaDirectories()
	if err := appSchema.InitializeDatabase(); err != nil {
		t.Fatal(err)
	}

	database, err := appSchema.OpenDatabase()
	if err != nil {
		t.Fatal(err)
	}
	s, err := database.Prepare("insert into messages (messageID, orderID, message_type, message, peerID, err, pubkey) values (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range existingRecords {
		_, err = s.Exec(r.MessageID, r.OrderID, r.MessageType, r.Message, r.PeerID, r.MsgErr, r.PeerPubkey)
		if err != nil {
			t.Fatal(err)
		}
	}

	handler := func(t pb.Message_MessageType) func(peer.ID, *pb.Message, interface{}) (*pb.Message, error) {
		return func(peer.ID, *pb.Message, interface{}) (*pb.Message, error) {
			if t == pb.Message_ORDER {
				return nil, nil
			}
			return nil, errors.New("unknown message type")
		}
	}

	extractor := func(data []byte) (*peer.ID, error) {
		return nil, nil
	}

	datastore := db.NewSQLiteDatastore(database, new(sync.Mutex), wi.Bitcoin)
	worker := &inboundMessageScanner{
		datastore:  datastore,
		logger:     logging.MustGetLogger("testInboundMsgScanner"),
		getHandler: handler,
		extractID:  extractor,
	}

	worker.PerformTask()
	msgs, err := worker.datastore.Messages().GetAllErrored()
	if err != nil {
		t.Errorf("err fetching msgs : %v", err)
	}

	if len(msgs) != 2 {
		t.Errorf("did not fetch the correct no of err records")
	}

	count := 0
	for _, msg := range msgs {
		if msg.MsgErr == ErrInsufficientFunds.Error() {
			count++
		}
	}

	if count != 1 {
		t.Errorf("did not pick the correct no of records to process")
	}

}
