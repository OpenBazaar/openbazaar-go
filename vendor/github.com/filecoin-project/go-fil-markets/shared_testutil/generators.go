package shared_testutil

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/market"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/test"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	smnet "github.com/filecoin-project/go-fil-markets/storagemarket/network"
)

// MakeTestSignedVoucher generates a random SignedVoucher that has all non-zero fields
func MakeTestSignedVoucher() *paych.SignedVoucher {
	return &paych.SignedVoucher{
		TimeLockMin:    abi.ChainEpoch(rand.Int63()),
		TimeLockMax:    0,
		SecretPreimage: []byte("secret-preimage"),
		Extra:          MakeTestModVerifyParams(),
		Lane:           rand.Uint64(),
		Nonce:          rand.Uint64(),
		Amount:         MakeTestTokenAmount(),
		Merges:         []paych.Merge{MakeTestMerge()},
		Signature:      MakeTestSignature(),
	}
}

// MakeTestModVerifyParams generates a random ModVerifyParams that has all non-zero fields
func MakeTestModVerifyParams() *paych.ModVerifyParams {
	return &paych.ModVerifyParams{
		Actor:  address.TestAddress,
		Method: abi.MethodNum(rand.Int63()),
		Data:   []byte("ModVerifyParams data"),
	}
}

// MakeTestMerge generates a random Merge that has all non-zero fields
func MakeTestMerge() paych.Merge {
	return paych.Merge{
		Lane:  rand.Uint64(),
		Nonce: rand.Uint64(),
	}
}

// MakeTestSignature generates a valid yet random Signature with all non-zero fields
func MakeTestSignature() *crypto.Signature {
	return &crypto.Signature{
		Type: crypto.SigTypeSecp256k1,
		Data: []byte("signature data"),
	}
}

// MakeTestTokenAmount generates a valid yet random TokenAmount with a non-zero value.
func MakeTestTokenAmount() abi.TokenAmount {
	return abi.TokenAmount{Int: big.NewInt(rand.Int63())}
}

// MakeTestQueryResponse generates a valid, random QueryResponse with no non-zero fields
func MakeTestQueryResponse() retrievalmarket.QueryResponse {
	return retrievalmarket.QueryResponse{
		Status:                     retrievalmarket.QueryResponseUnavailable,
		Size:                       rand.Uint64(),
		PaymentAddress:             address.TestAddress2,
		MinPricePerByte:            MakeTestTokenAmount(),
		MaxPaymentInterval:         rand.Uint64(),
		MaxPaymentIntervalIncrease: rand.Uint64(),
	}
}

// MakeTestDealProposal generates a valid, random DealProposal
func MakeTestDealProposal() retrievalmarket.DealProposal {
	cid := GenerateCids(1)[0]
	return retrievalmarket.DealProposal{
		PayloadCID: cid,
		ID:         retrievalmarket.DealID(rand.Uint64()),
		Params:     retrievalmarket.NewParamsV0(MakeTestTokenAmount(), rand.Uint64(), rand.Uint64()),
	}
}

// MakeTestDealProposal generates a valid, random DealResponse
func MakeTestDealResponse() retrievalmarket.DealResponse {
	fakeBlk := retrievalmarket.Block{
		Prefix: []byte("prefix"),
		Data:   []byte("data"),
	}

	return retrievalmarket.DealResponse{
		Status:      retrievalmarket.DealStatusOngoing,
		ID:          retrievalmarket.DealID(rand.Uint64()),
		PaymentOwed: MakeTestTokenAmount(),
		Message:     "deal response message",
		Blocks:      []retrievalmarket.Block{fakeBlk},
	}
}

// MakeTestDealPayment generates a valid, random DealPayment
func MakeTestDealPayment() retrievalmarket.DealPayment {
	return retrievalmarket.DealPayment{
		ID:             retrievalmarket.DealID(rand.Uint64()),
		PaymentChannel: address.TestAddress,
		PaymentVoucher: MakeTestSignedVoucher(),
	}
}

// MakeTestUnsignedDealProposal generates a deal proposal with no signature
func MakeTestUnsignedDealProposal() market.DealProposal {
	start := uint64(rand.Int31())
	end := start + uint64(rand.Int31())

	return market.DealProposal{
		PieceCID:  GenerateCids(1)[0],
		PieceSize: abi.PaddedPieceSize(rand.Int63()),

		Client:   address.TestAddress,
		Provider: address.TestAddress2,

		StartEpoch: abi.ChainEpoch(start),
		EndEpoch:   abi.ChainEpoch(end),

		StoragePricePerEpoch: MakeTestTokenAmount(),
		ProviderCollateral:   MakeTestTokenAmount(),
		ClientCollateral:     MakeTestTokenAmount(),
	}
}

// MakeTestClientDealProposal generates a valid storage deal proposal
func MakeTestClientDealProposal() *market.ClientDealProposal {
	return &market.ClientDealProposal{
		Proposal:        MakeTestUnsignedDealProposal(),
		ClientSignature: *MakeTestSignature(),
	}
}

// MakeTestDataRef returns a storage market data ref
func MakeTestDataRef(manualXfer bool) *storagemarket.DataRef {
	out := &storagemarket.DataRef{
		Root: GenerateCids(1)[0],
	}

	if manualXfer {
		out.TransferType = storagemarket.TTManual
	}

	return out
}

// MakeTestClientDeal returns a storage market client deal
func MakeTestClientDeal(state storagemarket.StorageDealStatus, clientDealProposal *market.ClientDealProposal, manualXfer bool) (*storagemarket.ClientDeal, error) {
	proposalNd, err := cborutil.AsIpld(clientDealProposal)

	if err != nil {
		return nil, err
	}

	p, err := test.RandPeerID()
	if err != nil {
		return nil, err
	}
	return &storagemarket.ClientDeal{
		ProposalCid:        proposalNd.Cid(),
		ClientDealProposal: *clientDealProposal,
		State:              state,
		Miner:              p,
		MinerWorker:        address.TestAddress2,
		DataRef:            MakeTestDataRef(manualXfer),
	}, nil
}

// MakeTestMinerDeal returns a storage market provider deal
func MakeTestMinerDeal(state storagemarket.StorageDealStatus, clientDealProposal *market.ClientDealProposal, dataRef *storagemarket.DataRef) (*storagemarket.MinerDeal, error) {
	proposalNd, err := cborutil.AsIpld(clientDealProposal)

	if err != nil {
		return nil, err
	}

	p, err := test.RandPeerID()
	if err != nil {
		return nil, err
	}

	return &storagemarket.MinerDeal{
		ProposalCid:        proposalNd.Cid(),
		ClientDealProposal: *clientDealProposal,
		State:              state,
		Client:             p,
		Ref:                dataRef,
	}, nil
}

// MakeTestStorageAsk generates a storage ask
func MakeTestStorageAsk() *storagemarket.StorageAsk {
	return &storagemarket.StorageAsk{
		Price:        MakeTestTokenAmount(),
		MinPieceSize: abi.PaddedPieceSize(rand.Uint64()),
		Miner:        address.TestAddress2,
		Timestamp:    abi.ChainEpoch(rand.Int63()),
		Expiry:       abi.ChainEpoch(rand.Int63()),
		SeqNo:        rand.Uint64(),
	}
}

// MakeTestSignedStorageAsk generates a signed storage ask
func MakeTestSignedStorageAsk() *storagemarket.SignedStorageAsk {
	return &storagemarket.SignedStorageAsk{
		Ask:       MakeTestStorageAsk(),
		Signature: MakeTestSignature(),
	}
}

// MakeTestStorageNetworkProposal generates a proposal that can be sent over the
// network to a provider
func MakeTestStorageNetworkProposal() smnet.Proposal {
	return smnet.Proposal{
		DealProposal: MakeTestClientDealProposal(),
		Piece:        &storagemarket.DataRef{Root: GenerateCids(1)[0]},
	}
}

// MakeTestStorageNetworkResponse generates a response to a proposal sent over
// the network
func MakeTestStorageNetworkResponse() smnet.Response {
	return smnet.Response{
		State:          storagemarket.StorageDealSealing,
		Proposal:       GenerateCids(1)[0],
		PublishMessage: &(GenerateCids(1)[0]),
	}
}

// MakeTestStorageNetworkSignedResponse generates a response to a proposal sent over
// the network that is signed
func MakeTestStorageNetworkSignedResponse() smnet.SignedResponse {
	return smnet.SignedResponse{
		Response:  MakeTestStorageNetworkResponse(),
		Signature: MakeTestSignature(),
	}
}

// MakeTestStorageAskRequest generates a request to get a provider's ask
func MakeTestStorageAskRequest() smnet.AskRequest {
	return smnet.AskRequest{
		Miner: address.TestAddress2,
	}
}

// MakeTestStorageAskResponse generates a response to an ask request
func MakeTestStorageAskResponse() smnet.AskResponse {
	return smnet.AskResponse{
		Ask: MakeTestSignedStorageAsk(),
	}
}

// MakeTestDealStatusRequest generates a request to get a provider's query
func MakeTestDealStatusRequest() smnet.DealStatusRequest {
	return smnet.DealStatusRequest{
		Proposal:  GenerateCids(1)[0],
		Signature: *MakeTestSignature(),
	}
}

// MakeTestDealStatusResponse generates a response to an query request
func MakeTestDealStatusResponse() smnet.DealStatusResponse {
	proposal := MakeTestUnsignedDealProposal()

	ds := storagemarket.ProviderDealState{
		Proposal:    &proposal,
		ProposalCid: &GenerateCids(1)[0],
		State:       storagemarket.StorageDealActive,
	}

	return smnet.DealStatusResponse{
		DealState: ds,
		Signature: *MakeTestSignature(),
	}
}

func RequireGenerateRetrievalPeers(t *testing.T, numPeers int) []retrievalmarket.RetrievalPeer {
	peers := make([]retrievalmarket.RetrievalPeer, numPeers)
	for i := range peers {
		pid, err := test.RandPeerID()
		require.NoError(t, err)
		addr, err := address.NewIDAddress(rand.Uint64())
		require.NoError(t, err)
		peers[i] = retrievalmarket.RetrievalPeer{
			Address: addr,
			ID:      pid,
		}
	}
	return peers
}

type FakeDTValidator struct{}

func (v *FakeDTValidator) ValidatePush(sender peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) error {
	return nil
}

func (v *FakeDTValidator) ValidatePull(receiver peer.ID, voucher datatransfer.Voucher, baseCid cid.Cid, selector ipld.Node) error {
	return nil
}

var _ datatransfer.RequestValidator = (*FakeDTValidator)(nil)
