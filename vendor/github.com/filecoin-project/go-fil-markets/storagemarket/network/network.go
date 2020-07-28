package network

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"

	ma "github.com/multiformats/go-multiaddr"
)

// These are the required interfaces that must be implemented to send and receive data
// for storage deals.

// StorageAskStream is a stream for reading/writing requests &
// responses on the Storage Ask protocol
type StorageAskStream interface {
	ReadAskRequest() (AskRequest, error)
	WriteAskRequest(AskRequest) error
	ReadAskResponse() (AskResponse, error)
	WriteAskResponse(AskResponse) error
	Close() error
}

// StorageDealStream is a stream for reading and writing requests
// and responses on the storage deal protocol
type StorageDealStream interface {
	ReadDealProposal() (Proposal, error)
	WriteDealProposal(Proposal) error
	ReadDealResponse() (SignedResponse, error)
	WriteDealResponse(SignedResponse) error
	RemotePeer() peer.ID
	TagProtectedConnection(identifier string)
	UntagProtectedConnection(identifier string)
	Close() error
}

// DealStatusStream is a stream for reading and writing requests
// and responses on the deal status protocol
type DealStatusStream interface {
	ReadDealStatusRequest() (DealStatusRequest, error)
	WriteDealStatusRequest(DealStatusRequest) error
	ReadDealStatusResponse() (DealStatusResponse, error)
	WriteDealStatusResponse(DealStatusResponse) error
	Close() error
}

// StorageReceiver implements functions for receiving
// incoming data on storage protocols
type StorageReceiver interface {
	HandleAskStream(StorageAskStream)
	HandleDealStream(StorageDealStream)
	HandleDealStatusStream(DealStatusStream)
}

// StorageMarketNetwork is a network abstraction for the storage market
type StorageMarketNetwork interface {
	NewAskStream(context.Context, peer.ID) (StorageAskStream, error)
	NewDealStream(context.Context, peer.ID) (StorageDealStream, error)
	NewDealStatusStream(context.Context, peer.ID) (DealStatusStream, error)
	SetDelegate(StorageReceiver) error
	StopHandlingRequests() error
	ID() peer.ID
	AddAddrs(peer.ID, []ma.Multiaddr)
}
