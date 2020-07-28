package storagemarket_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"path/filepath"
	"sync"
	"testing"

	"github.com/filecoin-project/go-address"
	graphsync "github.com/filecoin-project/go-data-transfer/impl/graphsync"
	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/pieceio"
	"github.com/filecoin-project/go-fil-markets/pieceio/cario"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/discovery"
	"github.com/filecoin-project/go-fil-markets/shared"
	"github.com/filecoin-project/go-fil-markets/shared_testutil"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	storageimpl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/requestvalidation"
	"github.com/filecoin-project/go-fil-markets/storagemarket/impl/storedask"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/go-fil-markets/storagemarket/testnodes"
)

func TestMakeDeal(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, ctx)
	require.NoError(t, h.Provider.Start(ctx))
	require.NoError(t, h.Client.Start(ctx))

	// set up a subscriber
	providerDealChan := make(chan storagemarket.MinerDeal)
	var checkedUnmarshalling bool
	subscriber := func(event storagemarket.ProviderEvent, deal storagemarket.MinerDeal) {
		if !checkedUnmarshalling {
			// test that deal created can marshall and unmarshalled
			jsonBytes, err := json.Marshal(deal)
			require.NoError(t, err)
			var unmDeal storagemarket.MinerDeal
			err = json.Unmarshal(jsonBytes, &unmDeal)
			require.NoError(t, err)
			require.Equal(t, deal, unmDeal)
			checkedUnmarshalling = true
		}
		providerDealChan <- deal
	}
	_ = h.Provider.SubscribeToEvents(subscriber)

	clientDealChan := make(chan storagemarket.ClientDeal)
	clientSubscriber := func(event storagemarket.ClientEvent, deal storagemarket.ClientDeal) {
		clientDealChan <- deal
	}
	_ = h.Client.SubscribeToEvents(clientSubscriber)

	// set ask price where we'll accept any price
	err := h.Provider.SetAsk(big.NewInt(0), 50_000)
	assert.NoError(t, err)

	result := h.ProposeStorageDeal(t, &storagemarket.DataRef{TransferType: storagemarket.TTGraphsync, Root: h.PayloadCid}, true, false)
	proposalCid := result.ProposalCid

	dealStatesToStrings := func(states []storagemarket.StorageDealStatus) []string {
		var out []string
		for _, state := range states {
			out = append(out, storagemarket.DealStates[state])
		}
		return out
	}

	var providerSeenDeal storagemarket.MinerDeal
	var clientSeenDeal storagemarket.ClientDeal
	var providerstates, clientstates []storagemarket.StorageDealStatus
	for providerSeenDeal.State != storagemarket.StorageDealExpired ||
		clientSeenDeal.State != storagemarket.StorageDealExpired {
		select {
		case clientSeenDeal = <-clientDealChan:
			if len(clientstates) == 0 || clientSeenDeal.State != clientstates[len(clientstates)-1] {
				clientstates = append(clientstates, clientSeenDeal.State)
			}
		case providerSeenDeal = <-providerDealChan:
			if len(providerstates) == 0 || providerSeenDeal.State != providerstates[len(providerstates)-1] {
				providerstates = append(providerstates, providerSeenDeal.State)
			}
		}
	}

	expProviderStates := []storagemarket.StorageDealStatus{
		storagemarket.StorageDealValidating,
		storagemarket.StorageDealAcceptWait,
		storagemarket.StorageDealWaitingForData,
		storagemarket.StorageDealTransferring,
		storagemarket.StorageDealVerifyData,
		storagemarket.StorageDealEnsureProviderFunds,
		storagemarket.StorageDealPublish,
		storagemarket.StorageDealPublishing,
		storagemarket.StorageDealStaged,
		storagemarket.StorageDealSealing,
		storagemarket.StorageDealRecordPiece,
		storagemarket.StorageDealActive,
		storagemarket.StorageDealExpired,
	}

	expClientStates := []storagemarket.StorageDealStatus{
		storagemarket.StorageDealEnsureClientFunds,
		//storagemarket.StorageDealClientFunding,  // skipped because funds available
		storagemarket.StorageDealFundsEnsured,
		storagemarket.StorageDealStartDataTransfer,
		storagemarket.StorageDealTransferring,
		storagemarket.StorageDealCheckForAcceptance,
		storagemarket.StorageDealProposalAccepted,
		storagemarket.StorageDealSealing,
		storagemarket.StorageDealActive,
		storagemarket.StorageDealExpired,
	}

	assert.Equal(t, dealStatesToStrings(expProviderStates), dealStatesToStrings(providerstates))
	assert.Equal(t, dealStatesToStrings(expClientStates), dealStatesToStrings(clientstates))

	// check a couple of things to make sure we're getting the whole deal
	assert.Equal(t, h.TestData.Host1.ID(), providerSeenDeal.Client)
	assert.Empty(t, providerSeenDeal.Message)
	assert.Equal(t, proposalCid, providerSeenDeal.ProposalCid)
	assert.Equal(t, h.ProviderAddr, providerSeenDeal.ClientDealProposal.Proposal.Provider)

	cd, err := h.Client.GetLocalDeal(ctx, proposalCid)
	assert.NoError(t, err)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealExpired, cd.State)
	assert.True(t, cd.FastRetrieval)

	providerDeals, err := h.Provider.ListLocalDeals()
	assert.NoError(t, err)

	pd := providerDeals[0]
	assert.Equal(t, proposalCid, pd.ProposalCid)
	assert.True(t, pd.FastRetrieval)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealExpired, pd.State)

	// test out query protocol
	status, err := h.Client.GetProviderDealState(ctx, proposalCid)
	assert.NoError(t, err)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealExpired, status.State)
	assert.True(t, status.FastRetrieval)

	// ensure that the handoff has fast retrieval info
	assert.Len(t, h.ProviderNode.OnDealCompleteCalls, 1)
	assert.True(t, h.ProviderNode.OnDealCompleteCalls[0].FastRetrieval)
}

func TestMakeDealOffline(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, ctx)
	require.NoError(t, h.Client.Start(ctx))

	carBuf := new(bytes.Buffer)

	err := cario.NewCarIO().WriteCar(ctx, h.TestData.Bs1, h.PayloadCid, shared.AllSelector(), carBuf)
	require.NoError(t, err)

	commP, size, err := pieceio.GeneratePieceCommitment(abi.RegisteredSealProof_StackedDrg2KiBV1, carBuf, uint64(carBuf.Len()))
	assert.NoError(t, err)

	dataRef := &storagemarket.DataRef{
		TransferType: storagemarket.TTManual,
		Root:         h.PayloadCid,
		PieceCid:     &commP,
		PieceSize:    size,
	}

	result := h.ProposeStorageDeal(t, dataRef, false, false)
	proposalCid := result.ProposalCid

	wg := sync.WaitGroup{}

	h.WaitForClientEvent(&wg, storagemarket.ClientEventDataTransferComplete)
	h.WaitForProviderEvent(&wg, storagemarket.ProviderEventDataRequested)
	wg.Wait()

	cd, err := h.Client.GetLocalDeal(ctx, proposalCid)
	assert.NoError(t, err)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealCheckForAcceptance, cd.State)

	providerDeals, err := h.Provider.ListLocalDeals()
	assert.NoError(t, err)

	pd := providerDeals[0]
	assert.True(t, pd.ProposalCid.Equals(proposalCid))
	shared_testutil.AssertDealState(t, storagemarket.StorageDealWaitingForData, pd.State)

	err = cario.NewCarIO().WriteCar(ctx, h.TestData.Bs1, h.PayloadCid, shared.AllSelector(), carBuf)
	require.NoError(t, err)
	err = h.Provider.ImportDataForDeal(ctx, pd.ProposalCid, carBuf)
	require.NoError(t, err)

	h.WaitForClientEvent(&wg, storagemarket.ClientEventDealExpired)
	h.WaitForProviderEvent(&wg, storagemarket.ProviderEventDealExpired)
	wg.Wait()

	cd, err = h.Client.GetLocalDeal(ctx, proposalCid)
	assert.NoError(t, err)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealExpired, cd.State)

	providerDeals, err = h.Provider.ListLocalDeals()
	assert.NoError(t, err)

	pd = providerDeals[0]
	assert.True(t, pd.ProposalCid.Equals(proposalCid))
	shared_testutil.AssertDealState(t, storagemarket.StorageDealExpired, pd.State)
}

func TestMakeDealNonBlocking(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, ctx)
	testCids := shared_testutil.GenerateCids(2)

	h.ProviderNode.WaitForMessageBlocks = true
	h.ProviderNode.AddFundsCid = testCids[1]
	require.NoError(t, h.Provider.Start(ctx))

	h.ClientNode.AddFundsCid = testCids[0]
	require.NoError(t, h.Client.Start(ctx))

	result := h.ProposeStorageDeal(t, &storagemarket.DataRef{TransferType: storagemarket.TTGraphsync, Root: h.PayloadCid}, false, false)

	wg := sync.WaitGroup{}
	h.WaitForClientEvent(&wg, storagemarket.ClientEventDataTransferComplete)
	h.WaitForProviderEvent(&wg, storagemarket.ProviderEventFundingInitiated)
	wg.Wait()

	cd, err := h.Client.GetLocalDeal(ctx, result.ProposalCid)
	assert.NoError(t, err)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealCheckForAcceptance, cd.State)

	providerDeals, err := h.Provider.ListLocalDeals()
	assert.NoError(t, err)

	// Provider should be blocking on waiting for funds to appear on chain
	pd := providerDeals[0]
	assert.Equal(t, result.ProposalCid, pd.ProposalCid)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealProviderFunding, pd.State)
}

func TestRestartClient(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, ctx)

	require.NoError(t, h.Provider.Start(ctx))
	require.NoError(t, h.Client.Start(ctx))

	// set ask price where we'll accept any price
	err := h.Provider.SetAsk(big.NewInt(0), 50_000)
	assert.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	_ = h.Client.SubscribeToEvents(func(event storagemarket.ClientEvent, deal storagemarket.ClientDeal) {
		if event == storagemarket.ClientEventFundsEnsured {
			// Stop the client and provider at some point during deal negotiation
			require.NoError(t, h.Client.Stop())
			require.NoError(t, h.Provider.Stop())
			wg.Done()
		}
	})

	result := h.ProposeStorageDeal(t, &storagemarket.DataRef{TransferType: storagemarket.TTGraphsync, Root: h.PayloadCid}, false, false)
	proposalCid := result.ProposalCid

	wg.Wait()

	cd, err := h.Client.GetLocalDeal(ctx, proposalCid)
	assert.NoError(t, err)
	assert.NotEqual(t, storagemarket.StorageDealActive, cd.State)

	h = newHarnessWithTestData(t, ctx, h.TestData, h.SMState)

	wg.Add(1)
	_ = h.Client.SubscribeToEvents(func(event storagemarket.ClientEvent, deal storagemarket.ClientDeal) {
		if event == storagemarket.ClientEventDealExpired {
			wg.Done()
		}
	})

	wg.Add(1)
	_ = h.Provider.SubscribeToEvents(func(event storagemarket.ProviderEvent, deal storagemarket.MinerDeal) {
		if event == storagemarket.ProviderEventDealExpired {
			wg.Done()
		}
	})

	require.NoError(t, h.Provider.Start(ctx))
	require.NoError(t, h.Client.Start(ctx))

	wg.Wait()

	cd, err = h.Client.GetLocalDeal(ctx, proposalCid)
	assert.NoError(t, err)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealExpired, cd.State)

	providerDeals, err := h.Provider.ListLocalDeals()
	assert.NoError(t, err)

	pd := providerDeals[0]
	assert.Equal(t, pd.ProposalCid, proposalCid)
	shared_testutil.AssertDealState(t, storagemarket.StorageDealExpired, pd.State)
}

type harness struct {
	Ctx          context.Context
	Epoch        abi.ChainEpoch
	PayloadCid   cid.Cid
	ProviderAddr address.Address
	Client       storagemarket.StorageClient
	ClientNode   *testnodes.FakeClientNode
	Provider     storagemarket.StorageProvider
	ProviderNode *testnodes.FakeProviderNode
	SMState      *testnodes.StorageMarketState
	ProviderInfo storagemarket.StorageProviderInfo
	TestData     *shared_testutil.Libp2pTestData
}

func newHarness(t *testing.T, ctx context.Context) *harness {
	smState := testnodes.NewStorageMarketState()
	return newHarnessWithTestData(t, ctx, shared_testutil.NewLibp2pTestData(ctx, t), smState)
}

func newHarnessWithTestData(t *testing.T, ctx context.Context, td *shared_testutil.Libp2pTestData, smState *testnodes.StorageMarketState) *harness {
	epoch := abi.ChainEpoch(100)
	fpath := filepath.Join("storagemarket", "fixtures", "payload.txt")
	rootLink := td.LoadUnixFSFile(t, fpath, false)
	payloadCid := rootLink.(cidlink.Link).Cid

	clientNode := testnodes.FakeClientNode{
		FakeCommonNode: testnodes.FakeCommonNode{SMState: smState},
		ClientAddr:     address.TestAddress,
	}

	expDealID := abi.DealID(rand.Uint64())
	psdReturn := market.PublishStorageDealsReturn{IDs: []abi.DealID{expDealID}}
	psdReturnBytes := bytes.NewBuffer([]byte{})
	err := psdReturn.MarshalCBOR(psdReturnBytes)
	assert.NoError(t, err)

	providerAddr := address.TestAddress2
	tempPath, err := ioutil.TempDir("", "storagemarket_test")
	assert.NoError(t, err)
	ps := piecestore.NewPieceStore(td.Ds2)
	providerNode := &testnodes.FakeProviderNode{
		FakeCommonNode: testnodes.FakeCommonNode{
			SMState:                smState,
			WaitForMessageRetBytes: psdReturnBytes.Bytes(),
		},
		MinerAddr: providerAddr,
	}
	fs, err := filestore.NewLocalFileStore(filestore.OsPath(tempPath))
	assert.NoError(t, err)

	// create provider and client
	dt1 := graphsync.NewGraphSyncDataTransfer(td.Host1, td.GraphSync1, td.DTStoredCounter1)
	rv1 := requestvalidation.NewUnifiedRequestValidator(nil, statestore.New(td.Ds1))
	require.NoError(t, dt1.RegisterVoucherType(&requestvalidation.StorageDataTransferVoucher{}, rv1))

	client, err := storageimpl.NewClient(
		network.NewFromLibp2pHost(td.Host1),
		td.Bs1,
		dt1,
		discovery.NewLocal(td.Ds1),
		td.Ds1,
		&clientNode,
		storageimpl.DealPollingInterval(0),
	)
	require.NoError(t, err)

	dt2 := graphsync.NewGraphSyncDataTransfer(td.Host2, td.GraphSync2, td.DTStoredCounter2)
	rv2 := requestvalidation.NewUnifiedRequestValidator(statestore.New(td.Ds2), nil)
	require.NoError(t, dt2.RegisterVoucherType(&requestvalidation.StorageDataTransferVoucher{}, rv2))

	storedAsk, err := storedask.NewStoredAsk(td.Ds2, datastore.NewKey("latest-ask"), providerNode, providerAddr)
	assert.NoError(t, err)
	provider, err := storageimpl.NewProvider(
		network.NewFromLibp2pHost(td.Host2),
		td.Ds2,
		td.Bs2,
		fs,
		ps,
		dt2,
		providerNode,
		providerAddr,
		abi.RegisteredSealProof_StackedDrg2KiBV1,
		storedAsk,
	)
	assert.NoError(t, err)

	// set ask price where we'll accept any price
	err = provider.SetAsk(big.NewInt(0), 50_000)
	assert.NoError(t, err)

	err = provider.Start(ctx)
	assert.NoError(t, err)

	// Closely follows the MinerInfo struct in the spec
	providerInfo := storagemarket.StorageProviderInfo{
		Address:    providerAddr,
		Owner:      providerAddr,
		Worker:     providerAddr,
		SectorSize: 1 << 20,
		PeerID:     td.Host2.ID(),
	}

	smState.Providers = []*storagemarket.StorageProviderInfo{&providerInfo}
	return &harness{
		Ctx:          ctx,
		Epoch:        epoch,
		PayloadCid:   payloadCid,
		ProviderAddr: providerAddr,
		Client:       client,
		ClientNode:   &clientNode,
		Provider:     provider,
		ProviderNode: providerNode,
		ProviderInfo: providerInfo,
		TestData:     td,
		SMState:      smState,
	}
}

func (h *harness) ProposeStorageDeal(t *testing.T, dataRef *storagemarket.DataRef, fastRetrieval, verifiedDeal bool) *storagemarket.ProposeStorageDealResult {
	result, err := h.Client.ProposeStorageDeal(h.Ctx, h.ProviderAddr, &h.ProviderInfo, dataRef, h.Epoch+100, h.Epoch+20100, big.NewInt(1), big.NewInt(0), abi.RegisteredSealProof_StackedDrg2KiBV1, fastRetrieval, verifiedDeal)
	assert.NoError(t, err)
	return result
}

func (h *harness) WaitForProviderEvent(wg *sync.WaitGroup, waitEvent storagemarket.ProviderEvent) {
	wg.Add(1)
	h.Provider.SubscribeToEvents(func(event storagemarket.ProviderEvent, deal storagemarket.MinerDeal) {
		if event == waitEvent {
			wg.Done()
		}
	})
}

func (h *harness) WaitForClientEvent(wg *sync.WaitGroup, waitEvent storagemarket.ClientEvent) {
	wg.Add(1)
	h.Client.SubscribeToEvents(func(event storagemarket.ClientEvent, deal storagemarket.ClientDeal) {
		if event == waitEvent {
			wg.Done()
		}
	})
}
