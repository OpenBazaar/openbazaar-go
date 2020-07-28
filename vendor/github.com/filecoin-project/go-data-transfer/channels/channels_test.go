package channels_test

import (
	"testing"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-data-transfer/channels"
	"github.com/filecoin-project/go-data-transfer/testutil"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/stretchr/testify/require"
)

func TestChannels(t *testing.T) {
	channelList := channels.New()

	tid1 := datatransfer.TransferID(0)
	tid2 := datatransfer.TransferID(1)
	fv1 := &testutil.FakeDTType{}
	fv2 := &testutil.FakeDTType{}
	cids := testutil.GenerateCids(2)
	selector := builder.NewSelectorSpecBuilder(basicnode.Style.Any).Matcher().Node()
	peers := testutil.GeneratePeers(4)

	t.Run("adding channels", func(t *testing.T) {
		chid, err := channelList.CreateNew(tid1, cids[0], selector, fv1, peers[0], peers[0], peers[1])
		require.NoError(t, err)
		require.Equal(t, peers[0], chid.Initiator)
		require.Equal(t, tid1, chid.ID)

		// cannot add twice for same channel id
		_, err = channelList.CreateNew(tid1, cids[1], selector, fv2, peers[0], peers[1], peers[0])
		require.Error(t, err)

		// can add for different id
		chid, err = channelList.CreateNew(tid2, cids[1], selector, fv2, peers[3], peers[2], peers[3])
		require.NoError(t, err)
		require.Equal(t, peers[3], chid.Initiator)
		require.Equal(t, tid2, chid.ID)
	})

	t.Run("in progress channels", func(t *testing.T) {
		inProgress := channelList.InProgress()
		require.Len(t, inProgress, 2)
		require.Contains(t, inProgress, datatransfer.ChannelID{Initiator: peers[0], ID: tid1})
		require.Contains(t, inProgress, datatransfer.ChannelID{Initiator: peers[3], ID: tid2})
	})

	t.Run("get by id and sender", func(t *testing.T) {
		state, err := channelList.GetByID(datatransfer.ChannelID{Initiator: peers[0], ID: tid1})
		require.NoError(t, err)
		require.NotEqual(t, channels.EmptyChannelState, state)
		require.Equal(t, cids[0], state.BaseCID())
		require.Equal(t, selector, state.Selector())
		require.Equal(t, fv1, state.Voucher())
		require.Equal(t, peers[0], state.Sender())
		require.Equal(t, peers[1], state.Recipient())

		// empty if channel does not exist
		state, err = channelList.GetByID(datatransfer.ChannelID{Initiator: peers[1], ID: tid1})
		require.Equal(t, channels.EmptyChannelState, state)
		require.EqualError(t, err, channels.ErrNotFound.Error())

		// works for other channel as well
		state, err = channelList.GetByID(datatransfer.ChannelID{Initiator: peers[3], ID: tid2})
		require.NotEqual(t, channels.EmptyChannelState, state)
		require.NoError(t, err)
	})

	t.Run("updating send/receive values", func(t *testing.T) {
		state, err := channelList.GetByID(datatransfer.ChannelID{Initiator: peers[0], ID: tid1})
		require.NoError(t, err)
		require.NotEqual(t, channels.EmptyChannelState, state)
		require.Equal(t, uint64(0), state.Received())
		require.Equal(t, uint64(0), state.Sent())

		received, err := channelList.IncrementReceived(datatransfer.ChannelID{Initiator: peers[0], ID: tid1}, 50)
		require.Equal(t, uint64(50), received)
		require.NoError(t, err)
		sent, err := channelList.IncrementSent(datatransfer.ChannelID{Initiator: peers[0], ID: tid1}, 100)
		require.Equal(t, uint64(100), sent)
		require.NoError(t, err)

		state, err = channelList.GetByID(datatransfer.ChannelID{Initiator: peers[0], ID: tid1})
		require.NoError(t, err)
		require.Equal(t, uint64(50), state.Received())
		require.Equal(t, uint64(100), state.Sent())

		// errors if channel does not exist
		_, err = channelList.IncrementReceived(datatransfer.ChannelID{Initiator: peers[1], ID: tid1}, 200)
		require.EqualError(t, err, channels.ErrNotFound.Error())
		_, err = channelList.IncrementSent(datatransfer.ChannelID{Initiator: peers[1], ID: tid1}, 200)
		require.EqualError(t, err, channels.ErrNotFound.Error())

		received, err = channelList.IncrementReceived(datatransfer.ChannelID{Initiator: peers[0], ID: tid1}, 50)
		require.Equal(t, uint64(100), received)
		require.NoError(t, err)
		sent, err = channelList.IncrementSent(datatransfer.ChannelID{Initiator: peers[0], ID: tid1}, 25)
		require.Equal(t, uint64(125), sent)
		require.NoError(t, err)
	})
}
