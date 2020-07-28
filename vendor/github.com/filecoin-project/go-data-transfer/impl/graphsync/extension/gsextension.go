package extension

import (
	"bytes"

	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/ipfs/go-graphsync"
	"github.com/libp2p/go-libp2p-core/peer"
)

const (
	// ExtensionDataTransfer is the identifier for the data transfer extension to graphsync
	ExtensionDataTransfer = graphsync.ExtensionName("fil/data-transfer")
)

//go:generate cbor-gen-for TransferData

// TransferData is the extension data for
// the graphsync extension.
type TransferData struct {
	TransferID uint64
	Initiator  peer.ID
	IsPull     bool
}

// GetChannelID gets the channelID for this extension, given the peers on either side
func (e TransferData) GetChannelID() datatransfer.ChannelID {
	return datatransfer.ChannelID{Initiator: e.Initiator, ID: datatransfer.TransferID(e.TransferID)}
}

// NewTransferData returns transfer data to encode in a graphsync request
func NewTransferData(transferID datatransfer.TransferID, initiator peer.ID, isPull bool) TransferData {
	return TransferData{
		TransferID: uint64(transferID),
		Initiator:  initiator,
		IsPull:     isPull,
	}
}

// GsExtended is a small interface used by getExtensionData
type GsExtended interface {
	Extension(name graphsync.ExtensionName) ([]byte, bool)
}

// GetTransferData unmarshals extension data.
// Returns:
//    * nil + nil if the extension is not found
//    * nil + error if the extendedData fails to unmarshal
//    * unmarshaled ExtensionDataTransferData + nil if all goes well
func GetTransferData(extendedData GsExtended) (*TransferData, error) {
	data, ok := extendedData.Extension(ExtensionDataTransfer)
	if !ok {
		return nil, nil
	}
	var extStruct TransferData

	reader := bytes.NewReader(data)
	if err := extStruct.UnmarshalCBOR(reader); err != nil {
		return nil, err
	}
	return &extStruct, nil
}
