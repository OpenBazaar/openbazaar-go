package autonat

import (
	pb "gx/ipfs/QmZjvy5eqc1T9VbFe5KBiDryUJSRNbSbHZD7JMc6CeUXuu/go-libp2p-autonat/pb"

	pstore "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"
)

const AutoNATProto = "/libp2p/autonat/1.0.0"

var log = logging.Logger("autonat")

func newDialMessage(pi pstore.PeerInfo) *pb.Message {
	msg := new(pb.Message)
	msg.Type = pb.Message_DIAL.Enum()
	msg.Dial = new(pb.Message_Dial)
	msg.Dial.Peer = new(pb.Message_PeerInfo)
	msg.Dial.Peer.Id = []byte(pi.ID)
	msg.Dial.Peer.Addrs = make([][]byte, len(pi.Addrs))
	for i, addr := range pi.Addrs {
		msg.Dial.Peer.Addrs[i] = addr.Bytes()
	}

	return msg
}
