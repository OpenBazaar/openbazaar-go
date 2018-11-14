package ackhandler

import (
	"gx/ipfs/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/protocol"
	"gx/ipfs/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/wire"
)

// This stopWaitingManager is not supposed to satisfy the StopWaitingManager interface, which is a remnant of the legacy AckHandler, and should be remove once we drop support for QUIC 33
type stopWaitingManager struct {
	largestLeastUnackedSent protocol.PacketNumber
	nextLeastUnacked        protocol.PacketNumber

	lastStopWaitingFrame *wire.StopWaitingFrame
}

func (s *stopWaitingManager) GetStopWaitingFrame(force bool) *wire.StopWaitingFrame {
	if s.nextLeastUnacked <= s.largestLeastUnackedSent {
		if force {
			return s.lastStopWaitingFrame
		}
		return nil
	}

	s.largestLeastUnackedSent = s.nextLeastUnacked
	swf := &wire.StopWaitingFrame{
		LeastUnacked: s.nextLeastUnacked,
	}
	s.lastStopWaitingFrame = swf
	return swf
}

func (s *stopWaitingManager) ReceivedAck(ack *wire.AckFrame) {
	largestAcked := ack.LargestAcked()
	if largestAcked >= s.nextLeastUnacked {
		s.nextLeastUnacked = largestAcked + 1
	}
}

func (s *stopWaitingManager) QueuedRetransmissionForPacketNumber(p protocol.PacketNumber) {
	if p >= s.nextLeastUnacked {
		s.nextLeastUnacked = p + 1
	}
}
