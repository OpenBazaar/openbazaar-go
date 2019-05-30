package factory

import (
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

func NewPeerID() *repo.PeerID {
	return &repo.PeerID{}
}

func NewPeerIDProtobuf() *pb.ID {
	return &pb.ID{
		PeerID:     "QmeJ3vRqsYVJXtFZr2MRo47KS9LStvW9g4LRK8uqGX2bt5",
		Handle:     "",
		Pubkeys:    NewPubkeysProtobuf(),
		BitcoinSig: []byte("MEQCIGqBDqGyLGs8tVewab+b8BIMCY73uGrxg7wPro3+JuVmAiBa2wk55FwGWWQoyLYW1mGhP62FLHyk6pfkAt59A1tU8Q=="),
	}
}

func NewPubkeysProtobuf() *pb.ID_Pubkeys {
	return &pb.ID_Pubkeys{
		Identity: []byte("CAESIII6nbBUBtCkK0blWtYRwm2lKS4kuAm36sElyoeC0n0u"),
		Bitcoin:  []byte("AwD4y8eIx7F0bnwNmssZGi+XFqydypxuFRtA4TPyWiqJ"),
	}
}
