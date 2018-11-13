package decision

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	message "gx/ipfs/QmNkxFCmPtr2RQxjZNRCNryLud4L9wMEiBJsLgF14MqTHj/go-bitswap/message"

	blocks "gx/ipfs/QmRcHuYzAyswytBuMF78rj3LTChYszomRFXNg4685ZN1WM/go-block-format"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	testutil "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dssync "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/sync"
	blockstore "gx/ipfs/QmcDDgAXDbpDUpadCJKLr49KYR4HuL7T8Z1dZTHt6ixsoR/go-ipfs-blockstore"
)

type peerAndEngine struct {
	Peer   peer.ID
	Engine *Engine
}

func newEngine(ctx context.Context, idStr string) peerAndEngine {
	return peerAndEngine{
		Peer: peer.ID(idStr),
		//Strategy: New(true),
		Engine: NewEngine(ctx,
			blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))),
	}
}

func TestConsistentAccounting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sender := newEngine(ctx, "Ernie")
	receiver := newEngine(ctx, "Bert")

	// Send messages from Ernie to Bert
	for i := 0; i < 1000; i++ {

		m := message.New(false)
		content := []string{"this", "is", "message", "i"}
		m.AddBlock(blocks.NewBlock([]byte(strings.Join(content, " "))))

		sender.Engine.MessageSent(receiver.Peer, m)
		receiver.Engine.MessageReceived(sender.Peer, m)
	}

	// Ensure sender records the change
	if sender.Engine.numBytesSentTo(receiver.Peer) == 0 {
		t.Fatal("Sent bytes were not recorded")
	}

	// Ensure sender and receiver have the same values
	if sender.Engine.numBytesSentTo(receiver.Peer) != receiver.Engine.numBytesReceivedFrom(sender.Peer) {
		t.Fatal("Inconsistent book-keeping. Strategies don't agree")
	}

	// Ensure sender didn't record receving anything. And that the receiver
	// didn't record sending anything
	if receiver.Engine.numBytesSentTo(sender.Peer) != 0 || sender.Engine.numBytesReceivedFrom(receiver.Peer) != 0 {
		t.Fatal("Bert didn't send bytes to Ernie")
	}
}

func TestPeerIsAddedToPeersWhenMessageReceivedOrSent(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sanfrancisco := newEngine(ctx, "sf")
	seattle := newEngine(ctx, "sea")

	m := message.New(true)

	sanfrancisco.Engine.MessageSent(seattle.Peer, m)
	seattle.Engine.MessageReceived(sanfrancisco.Peer, m)

	if seattle.Peer == sanfrancisco.Peer {
		t.Fatal("Sanity Check: Peers have same Key!")
	}

	if !peerIsPartner(seattle.Peer, sanfrancisco.Engine) {
		t.Fatal("Peer wasn't added as a Partner")
	}

	if !peerIsPartner(sanfrancisco.Peer, seattle.Engine) {
		t.Fatal("Peer wasn't added as a Partner")
	}

	seattle.Engine.PeerDisconnected(sanfrancisco.Peer)
	if peerIsPartner(sanfrancisco.Peer, seattle.Engine) {
		t.Fatal("expected peer to be removed")
	}
}

func peerIsPartner(p peer.ID, e *Engine) bool {
	for _, partner := range e.Peers() {
		if partner == p {
			return true
		}
	}
	return false
}

func TestOutboxClosedWhenEngineClosed(t *testing.T) {
	t.SkipNow() // TODO implement *Engine.Close
	e := NewEngine(context.Background(), blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore())))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for nextEnvelope := range e.Outbox() {
			<-nextEnvelope
		}
		wg.Done()
	}()
	// e.Close()
	wg.Wait()
	if _, ok := <-e.Outbox(); ok {
		t.Fatal("channel should be closed")
	}
}

func TestPartnerWantsThenCancels(t *testing.T) {
	numRounds := 10
	if testing.Short() {
		numRounds = 1
	}
	alphabet := strings.Split("abcdefghijklmnopqrstuvwxyz", "")
	vowels := strings.Split("aeiou", "")

	type testCase [][]string
	testcases := []testCase{
		{
			alphabet, vowels,
		},
		{
			alphabet, stringsComplement(alphabet, vowels),
			alphabet[1:25], stringsComplement(alphabet[1:25], vowels), alphabet[2:25], stringsComplement(alphabet[2:25], vowels),
			alphabet[3:25], stringsComplement(alphabet[3:25], vowels), alphabet[4:25], stringsComplement(alphabet[4:25], vowels),
			alphabet[5:25], stringsComplement(alphabet[5:25], vowels), alphabet[6:25], stringsComplement(alphabet[6:25], vowels),
			alphabet[7:25], stringsComplement(alphabet[7:25], vowels), alphabet[8:25], stringsComplement(alphabet[8:25], vowels),
			alphabet[9:25], stringsComplement(alphabet[9:25], vowels), alphabet[10:25], stringsComplement(alphabet[10:25], vowels),
			alphabet[11:25], stringsComplement(alphabet[11:25], vowels), alphabet[12:25], stringsComplement(alphabet[12:25], vowels),
			alphabet[13:25], stringsComplement(alphabet[13:25], vowels), alphabet[14:25], stringsComplement(alphabet[14:25], vowels),
			alphabet[15:25], stringsComplement(alphabet[15:25], vowels), alphabet[16:25], stringsComplement(alphabet[16:25], vowels),
			alphabet[17:25], stringsComplement(alphabet[17:25], vowels), alphabet[18:25], stringsComplement(alphabet[18:25], vowels),
			alphabet[19:25], stringsComplement(alphabet[19:25], vowels), alphabet[20:25], stringsComplement(alphabet[20:25], vowels),
			alphabet[21:25], stringsComplement(alphabet[21:25], vowels), alphabet[22:25], stringsComplement(alphabet[22:25], vowels),
			alphabet[23:25], stringsComplement(alphabet[23:25], vowels), alphabet[24:25], stringsComplement(alphabet[24:25], vowels),
			alphabet[25:25], stringsComplement(alphabet[25:25], vowels),
		},
	}

	bs := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	for _, letter := range alphabet {
		block := blocks.NewBlock([]byte(letter))
		if err := bs.Put(block); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < numRounds; i++ {
		expected := make([][]string, 0, len(testcases))
		e := NewEngine(context.Background(), bs)
		for _, testcase := range testcases {
			set := testcase[0]
			cancels := testcase[1]
			keeps := stringsComplement(set, cancels)
			expected = append(expected, keeps)

			partner := testutil.RandPeerIDFatal(t)

			partnerWants(e, set, partner)
			partnerCancels(e, cancels, partner)
		}
		if err := checkHandledInOrder(t, e, expected); err != nil {
			t.Logf("run #%d of %d", i, numRounds)
			t.Fatal(err)
		}
	}
}

func partnerWants(e *Engine, keys []string, partner peer.ID) {
	add := message.New(false)
	for i, letter := range keys {
		block := blocks.NewBlock([]byte(letter))
		add.AddEntry(block.Cid(), len(keys)-i)
	}
	e.MessageReceived(partner, add)
}

func partnerCancels(e *Engine, keys []string, partner peer.ID) {
	cancels := message.New(false)
	for _, k := range keys {
		block := blocks.NewBlock([]byte(k))
		cancels.Cancel(block.Cid())
	}
	e.MessageReceived(partner, cancels)
}

func checkHandledInOrder(t *testing.T, e *Engine, expected [][]string) error {
	for _, keys := range expected {
		next := <-e.Outbox()
		envelope := <-next
		received := envelope.Message.Blocks()
		// Verify payload message length
		if len(received) != len(keys) {
			return errors.New(fmt.Sprintln("# blocks received", len(received), "# blocks expected", len(keys)))
		}
		// Verify payload message contents
		for _, k := range keys {
			found := false
			expected := blocks.NewBlock([]byte(k))
			for _, block := range received {
				if block.Cid().Equals(expected.Cid()) {
					found = true
					break
				}
			}
			if !found {
				return errors.New(fmt.Sprintln("received", received, "expected", string(expected.RawData())))
			}
		}
	}
	return nil
}

func stringsComplement(set, subset []string) []string {
	m := make(map[string]struct{})
	for _, letter := range subset {
		m[letter] = struct{}{}
	}
	var complement []string
	for _, letter := range set {
		if _, exists := m[letter]; !exists {
			complement = append(complement, letter)
		}
	}
	return complement
}
