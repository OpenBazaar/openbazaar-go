package requestvalidation

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-statestore"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-fil-markets/storagemarket"
)

// ValidatePush validates a push request received from the peer that will send data
// Will succeed only if:
// - voucher has correct type
// - voucher references an active deal
// - referenced deal matches the client
// - referenced deal matches the given base CID
// - referenced deal is in an acceptable state
func ValidatePush(
	deals *statestore.StateStore,
	sender peer.ID,
	voucher datatransfer.Voucher,
	baseCid cid.Cid,
	Selector ipld.Node) error {
	dealVoucher, ok := voucher.(*StorageDataTransferVoucher)
	if !ok {
		return xerrors.Errorf("voucher type %s: %w", voucher.Type(), ErrWrongVoucherType)
	}

	var deal storagemarket.MinerDeal
	err := deals.Get(dealVoucher.Proposal).Get(&deal)
	if err != nil {
		return xerrors.Errorf("Proposal CID %s: %w", dealVoucher.Proposal.String(), ErrNoDeal)
	}
	if deal.Client != sender {
		return xerrors.Errorf("Deal Peer %s, Data Transfer Peer %s: %w", deal.Client.String(), sender.String(), ErrWrongPeer)
	}

	if !deal.Ref.Root.Equals(baseCid) {
		return xerrors.Errorf("Deal Payload CID %s, Data Transfer CID %s: %w", deal.Proposal.PieceCID.String(), baseCid.String(), ErrWrongPiece)
	}
	for _, state := range DataTransferStates {
		if deal.State == state {
			return nil
		}
	}
	return xerrors.Errorf("Deal State %s: %w", deal.State, ErrInacceptableDealState)
}

// ValidatePull validates a pull request received from the peer that will receive data
// Will succeed only if:
// - voucher has correct type
// - voucher references an active deal
// - referenced deal matches the receiver (miner)
// - referenced deal matches the given base CID
// - referenced deal is in an acceptable state
func ValidatePull(
	deals *statestore.StateStore,
	receiver peer.ID,
	voucher datatransfer.Voucher,
	baseCid cid.Cid,
	Selector ipld.Node) error {
	dealVoucher, ok := voucher.(*StorageDataTransferVoucher)
	if !ok {
		return xerrors.Errorf("voucher type %s: %w", voucher.Type(), ErrWrongVoucherType)
	}

	var deal storagemarket.ClientDeal
	err := deals.Get(dealVoucher.Proposal).Get(&deal)
	if err != nil {
		return xerrors.Errorf("Proposal CID %s: %w", dealVoucher.Proposal.String(), ErrNoDeal)
	}

	if deal.Miner != receiver {
		return xerrors.Errorf("Deal Peer %s, Data Transfer Peer %s: %w", deal.Miner.String(), receiver.String(), ErrWrongPeer)
	}
	if !deal.DataRef.Root.Equals(baseCid) {
		return xerrors.Errorf("Deal Payload CID %s, Data Transfer CID %s: %w", deal.Proposal.PieceCID.String(), baseCid.String(), ErrWrongPiece)
	}
	for _, state := range DataTransferStates {
		if deal.State == state {
			return nil
		}
	}
	return xerrors.Errorf("Deal State %s: %w", deal.State, ErrInacceptableDealState)
}
