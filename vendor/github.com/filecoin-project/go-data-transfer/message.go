package datatransfer

import (
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/protocol"
	cborgen "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-data-transfer/encoding"
)

var (
	// ProtocolDataTransfer1_1 is the protocol identifier for graphsync messages
	ProtocolDataTransfer1_1 protocol.ID = "/fil/datatransfer/1.1.0"

	// ProtocolDataTransfer1_0 is the protocol identifier for legacy graphsync messages
	// This protocol does NOT support the `Restart` functionality for data transfer channels.
	ProtocolDataTransfer1_0 protocol.ID = "/fil/datatransfer/1.0.0"
)

// Message is a message for the data transfer protocol
// (either request or response) that can serialize to a protobuf
type Message interface {
	IsRequest() bool
	IsRestart() bool
	IsNew() bool
	IsUpdate() bool
	IsPaused() bool
	IsCancel() bool
	TransferID() TransferID
	cborgen.CBORMarshaler
	cborgen.CBORUnmarshaler
	ToNet(w io.Writer) error
	MessageForProtocol(targetProtocol protocol.ID) (newMsg Message, err error)
}

// Request is a response message for the data transfer protocol
type Request interface {
	Message
	IsPull() bool
	IsVoucher() bool
	VoucherType() TypeIdentifier
	Voucher(decoder encoding.Decoder) (encoding.Encodable, error)
	BaseCid() cid.Cid
	Selector() (ipld.Node, error)
	IsRestartExistingChannelRequest() bool
	RestartChannelId() (ChannelID, error)
}

// Response is a response message for the data transfer protocol
type Response interface {
	Message
	IsVoucherResult() bool
	IsComplete() bool
	Accepted() bool
	VoucherResultType() TypeIdentifier
	VoucherResult(decoder encoding.Decoder) (encoding.Encodable, error)
	EmptyVoucherResult() bool
}
