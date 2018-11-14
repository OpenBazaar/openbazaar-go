package wire

import (
	"bytes"

	"gx/ipfs/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/protocol"
	"gx/ipfs/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/utils"
)

// A MaxStreamIDFrame is a MAX_STREAM_ID frame
type MaxStreamIDFrame struct {
	StreamID protocol.StreamID
}

// parseMaxStreamIDFrame parses a MAX_STREAM_ID frame
func parseMaxStreamIDFrame(r *bytes.Reader, _ protocol.VersionNumber) (*MaxStreamIDFrame, error) {
	// read the Type byte
	if _, err := r.ReadByte(); err != nil {
		return nil, err
	}
	streamID, err := utils.ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	return &MaxStreamIDFrame{StreamID: protocol.StreamID(streamID)}, nil
}

func (f *MaxStreamIDFrame) Write(b *bytes.Buffer, _ protocol.VersionNumber) error {
	b.WriteByte(0x6)
	utils.WriteVarInt(b, uint64(f.StreamID))
	return nil
}

// Length of a written frame
func (f *MaxStreamIDFrame) Length(protocol.VersionNumber) protocol.ByteCount {
	return 1 + utils.VarIntLen(uint64(f.StreamID))
}
