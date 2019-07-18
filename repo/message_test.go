package repo_test

import (
	"bytes"
	"testing"

	"github.com/golang/protobuf/ptypes/any"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

func TestMessage(t *testing.T) {
	var (
		mType   = pb.Message_ORDER
		payload = "sample message"
	)

	msg := pb.Message{
		MessageType: mType,
		Payload:     &any.Any{Value: []byte(payload)},
	}

	repoMsg := repo.Message{Msg: msg}

	repoMsgBytes, err := repoMsg.MarshalJSON()
	if err != nil {
		t.Error(err)
	}

	var retRepoMsg repo.Message

	err = retRepoMsg.UnmarshalJSON(repoMsgBytes)
	if err != nil {
		t.Error(err)
	}

	if retRepoMsg.GetMessageType() != pb.Message_ORDER {
		t.Error("wrong msg type")
	}

	if !bytes.Equal(retRepoMsg.GetPayload().GetValue(), []byte(payload)) {
		t.Error("wrong msg type")
	}

}
