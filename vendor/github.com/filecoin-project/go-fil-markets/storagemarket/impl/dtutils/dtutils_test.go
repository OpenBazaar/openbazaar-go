package dtutils_test

import (
	"testing"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-statemachine/fsm"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/dtutils"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/requestvalidation"
)

func TestProviderDataTransferSubscriber(t *testing.T) {
	expectedProposalCID := shared_testutil.GenerateCids(1)[0]
	tests := map[string]struct {
		code          datatransfer.EventCode
		called        bool
		voucher       datatransfer.Voucher
		expectedID    interface{}
		expectedEvent fsm.EventName
		expectedArgs  []interface{}
	}{
		"not a storage voucher": {
			called:  false,
			voucher: nil,
		},
		"open event": {
			code:   datatransfer.Open,
			called: true,
			voucher: &requestvalidation.StorageDataTransferVoucher{
				Proposal: expectedProposalCID,
			},
			expectedID:    expectedProposalCID,
			expectedEvent: storagemarket.ProviderEventDataTransferInitiated,
		},
		"completion event": {
			code:   datatransfer.Complete,
			called: true,
			voucher: &requestvalidation.StorageDataTransferVoucher{
				Proposal: expectedProposalCID,
			},
			expectedID:    expectedProposalCID,
			expectedEvent: storagemarket.ProviderEventDataTransferCompleted,
		},
		"error event": {
			code:   datatransfer.Error,
			called: true,
			voucher: &requestvalidation.StorageDataTransferVoucher{
				Proposal: expectedProposalCID,
			},
			expectedID:    expectedProposalCID,
			expectedEvent: storagemarket.ProviderEventDataTransferFailed,
			expectedArgs:  []interface{}{dtutils.ErrDataTransferFailed},
		},
		"other event": {
			code:   datatransfer.Progress,
			called: false,
			voucher: &requestvalidation.StorageDataTransferVoucher{
				Proposal: expectedProposalCID,
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			fdg := &fakeDealGroup{}
			subscriber := dtutils.ProviderDataTransferSubscriber(fdg)
			subscriber(datatransfer.Event{Code: data.code}, datatransfer.ChannelState{
				Channel: datatransfer.NewChannel(datatransfer.TransferID(0), cid.Undef, nil, data.voucher, peer.ID(""), peer.ID(""), 0),
			})
			if data.called {
				require.True(t, fdg.called)
				require.Equal(t, fdg.lastID, data.expectedID)
				require.Equal(t, fdg.lastEvent, data.expectedEvent)
				require.Equal(t, fdg.lastArgs, data.expectedArgs)
			} else {
				require.False(t, fdg.called)
			}
		})
	}
}

func TestClientDataTransferSubscriber(t *testing.T) {
	expectedProposalCID := shared_testutil.GenerateCids(1)[0]
	tests := map[string]struct {
		code          datatransfer.EventCode
		called        bool
		voucher       datatransfer.Voucher
		expectedID    interface{}
		expectedEvent fsm.EventName
		expectedArgs  []interface{}
	}{
		"not a storage voucher": {
			called:  false,
			voucher: nil,
		},
		"completion event": {
			code:   datatransfer.Complete,
			called: true,
			voucher: &requestvalidation.StorageDataTransferVoucher{
				Proposal: expectedProposalCID,
			},
			expectedID:    expectedProposalCID,
			expectedEvent: storagemarket.ClientEventDataTransferComplete,
		},
		"error event": {
			code:   datatransfer.Error,
			called: true,
			voucher: &requestvalidation.StorageDataTransferVoucher{
				Proposal: expectedProposalCID,
			},
			expectedID:    expectedProposalCID,
			expectedEvent: storagemarket.ClientEventDataTransferFailed,
			expectedArgs:  []interface{}{dtutils.ErrDataTransferFailed},
		},
		"other event": {
			code:   datatransfer.Progress,
			called: false,
			voucher: &requestvalidation.StorageDataTransferVoucher{
				Proposal: expectedProposalCID,
			},
		},
	}
	for test, data := range tests {
		t.Run(test, func(t *testing.T) {
			fdg := &fakeDealGroup{}
			subscriber := dtutils.ClientDataTransferSubscriber(fdg)
			subscriber(datatransfer.Event{Code: data.code}, datatransfer.ChannelState{
				Channel: datatransfer.NewChannel(datatransfer.TransferID(0), cid.Undef, nil, data.voucher, peer.ID(""), peer.ID(""), 0),
			})
			if data.called {
				require.True(t, fdg.called)
				require.Equal(t, fdg.lastID, data.expectedID)
				require.Equal(t, fdg.lastEvent, data.expectedEvent)
				require.Equal(t, fdg.lastArgs, data.expectedArgs)
			} else {
				require.False(t, fdg.called)
			}
		})
	}
}

type fakeDealGroup struct {
	returnedErr error
	called      bool
	lastID      interface{}
	lastEvent   fsm.EventName
	lastArgs    []interface{}
}

func (fdg *fakeDealGroup) Send(id interface{}, name fsm.EventName, args ...interface{}) (err error) {
	fdg.lastID = id
	fdg.lastEvent = name
	fdg.lastArgs = args
	fdg.called = true
	return fdg.returnedErr
}
