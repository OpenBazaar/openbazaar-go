package pubsub

import (
	"encoding/binary"
	"fmt"
	"testing"

	pb "gx/ipfs/QmY4dowpPFCBsbaoaJc9mNWso64eDJsm32LJznwPNaAiJG/go-libp2p-pubsub/pb"
)

func TestMessageCache(t *testing.T) {
	mcache := NewMessageCache(3, 5)

	msgs := make([]*pb.Message, 60)
	for i := range msgs {
		msgs[i] = makeTestMessage(i)
	}

	for i := 0; i < 10; i++ {
		mcache.Put(msgs[i])
	}

	for i := 0; i < 10; i++ {
		mid := msgID(msgs[i])
		m, ok := mcache.Get(mid)
		if !ok {
			t.Fatalf("Message %d not in cache", i)
		}

		if m != msgs[i] {
			t.Fatalf("Message %d does not match cache", i)
		}
	}

	gids := mcache.GetGossipIDs("test")
	if len(gids) != 10 {
		t.Fatalf("Expected 10 gossip IDs; got %d", len(gids))
	}

	for i := 0; i < 10; i++ {
		mid := msgID(msgs[i])
		if mid != gids[i] {
			t.Fatalf("GossipID mismatch for message %d", i)
		}
	}

	mcache.Shift()
	for i := 10; i < 20; i++ {
		mcache.Put(msgs[i])
	}

	for i := 0; i < 20; i++ {
		mid := msgID(msgs[i])
		m, ok := mcache.Get(mid)
		if !ok {
			t.Fatalf("Message %d not in cache", i)
		}

		if m != msgs[i] {
			t.Fatalf("Message %d does not match cache", i)
		}
	}

	gids = mcache.GetGossipIDs("test")
	if len(gids) != 20 {
		t.Fatalf("Expected 20 gossip IDs; got %d", len(gids))
	}

	for i := 0; i < 10; i++ {
		mid := msgID(msgs[i])
		if mid != gids[10+i] {
			t.Fatalf("GossipID mismatch for message %d", i)
		}
	}

	for i := 10; i < 20; i++ {
		mid := msgID(msgs[i])
		if mid != gids[i-10] {
			t.Fatalf("GossipID mismatch for message %d", i)
		}
	}

	mcache.Shift()
	for i := 20; i < 30; i++ {
		mcache.Put(msgs[i])
	}

	mcache.Shift()
	for i := 30; i < 40; i++ {
		mcache.Put(msgs[i])
	}

	mcache.Shift()
	for i := 40; i < 50; i++ {
		mcache.Put(msgs[i])
	}

	mcache.Shift()
	for i := 50; i < 60; i++ {
		mcache.Put(msgs[i])
	}

	if len(mcache.msgs) != 50 {
		t.Fatalf("Expected 50 messages in the cache; got %d", len(mcache.msgs))
	}

	for i := 0; i < 10; i++ {
		mid := msgID(msgs[i])
		_, ok := mcache.Get(mid)
		if ok {
			t.Fatalf("Message %d still in cache", i)
		}
	}

	for i := 10; i < 60; i++ {
		mid := msgID(msgs[i])
		m, ok := mcache.Get(mid)
		if !ok {
			t.Fatalf("Message %d not in cache", i)
		}

		if m != msgs[i] {
			t.Fatalf("Message %d does not match cache", i)
		}
	}

	gids = mcache.GetGossipIDs("test")
	if len(gids) != 30 {
		t.Fatalf("Expected 30 gossip IDs; got %d", len(gids))
	}

	for i := 0; i < 10; i++ {
		mid := msgID(msgs[50+i])
		if mid != gids[i] {
			t.Fatalf("GossipID mismatch for message %d", i)
		}
	}

	for i := 10; i < 20; i++ {
		mid := msgID(msgs[30+i])
		if mid != gids[i] {
			t.Fatalf("GossipID mismatch for message %d", i)
		}
	}

	for i := 20; i < 30; i++ {
		mid := msgID(msgs[10+i])
		if mid != gids[i] {
			t.Fatalf("GossipID mismatch for message %d", i)
		}
	}

}

func makeTestMessage(n int) *pb.Message {
	seqno := make([]byte, 8)
	binary.BigEndian.PutUint64(seqno, uint64(n))
	data := []byte(fmt.Sprintf("%d", n))
	return &pb.Message{
		Data:     data,
		TopicIDs: []string{"test"},
		From:     []byte("test"),
		Seqno:    seqno,
	}
}
