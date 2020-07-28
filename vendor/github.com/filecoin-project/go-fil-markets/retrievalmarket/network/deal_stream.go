package network

import (
	"bufio"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

type dealStream struct {
	p        peer.ID
	rw       mux.MuxedStream
	buffered *bufio.Reader
}

var _ RetrievalDealStream = (*dealStream)(nil)

func (d *dealStream) ReadDealProposal() (retrievalmarket.DealProposal, error) {
	var ds retrievalmarket.DealProposal

	if err := ds.UnmarshalCBOR(d.buffered); err != nil {
		log.Warn(err)
		return retrievalmarket.DealProposalUndefined, err
	}
	return ds, nil
}

func (d *dealStream) WriteDealProposal(dp retrievalmarket.DealProposal) error {
	return cborutil.WriteCborRPC(d.rw, &dp)
}

func (d *dealStream) ReadDealResponse() (retrievalmarket.DealResponse, error) {
	var dr retrievalmarket.DealResponse

	if err := dr.UnmarshalCBOR(d.buffered); err != nil {
		return retrievalmarket.DealResponseUndefined, err
	}
	return dr, nil
}

func (d *dealStream) WriteDealResponse(dr retrievalmarket.DealResponse) error {
	return cborutil.WriteCborRPC(d.rw, &dr)
}

func (d *dealStream) ReadDealPayment() (retrievalmarket.DealPayment, error) {
	var ds retrievalmarket.DealPayment

	if err := ds.UnmarshalCBOR(d.rw); err != nil {
		return retrievalmarket.DealPaymentUndefined, err
	}
	return ds, nil
}

func (d *dealStream) WriteDealPayment(dpy retrievalmarket.DealPayment) error {
	return cborutil.WriteCborRPC(d.rw, &dpy)
}

func (d *dealStream) Receiver() peer.ID {
	return d.p
}

func (d *dealStream) Close() error {
	return d.rw.Close()
}
