package factory

import (
	"encoding/base64"
	"fmt"

	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/repo"
)

func MustNewPeerInfo() *repo.PeerInfo {
	return repo.NewPeerInfoFromIdentityKey(NewPubkeysIdentityKeyBytes())
}

func NewPeerInfo() *repo.PeerInfo {
	return repo.NewPeerInfoFromIdentityKey(NewPubkeysIdentityKeyBytes())
}

func NewPeerIDProtobuf() *pb.ID {
	return &pb.ID{
		PeerID:  "QmeJ3vRqsYVJXtFZr2MRo47KS9LStvW9g4LRK8uqGX2bt5",
		Handle:  "",
		Pubkeys: NewPubkeysProtobuf(),
		//BitcoinSig: []byte("MEQCIGqBDqGyLGs8tVewab+b8BIMCY73uGrxg7wPro3+JuVmAiBa2wk55FwGWWQoyLYW1mGhP62FLHyk6pfkAt59A1tU8Q=="),
	}
}

func NewPubkeysIdentityKeyBytes() []byte {
	keyBytes, err := base64.StdEncoding.DecodeString("CAESIII6nbBUBtCkK0blWtYRwm2lKS4kuAm36sElyoeC0n0u")
	if err != nil {
		panic(err)
	}
	return keyBytes
}

func NewPubkeysProtobuf() *pb.ID_Pubkeys {
	return &pb.ID_Pubkeys{
		Identity: NewPubkeysIdentityKeyBytes(),
		//Bitcoin:  []byte("AwD4y8eIx7F0bnwNmssZGi+XFqydypxuFRtA4TPyWiqJ"),
	}
}

// NewValidPeerIDProtobuf returns a PeerID protobuf example that is known to be valid
func MustNewValidPeerIDProtobuf() *pb.ID {
	validIdentity, err := base64.StdEncoding.DecodeString("CAESIII6nbBUBtCkK0blWtYRwm2lKS4kuAm36sElyoeC0n0u")
	if err != nil {
		panic(err)
	}
	return &pb.ID{
		PeerID: "QmeJ3vRqsYVJXtFZr2MRo47KS9LStvW9g4LRK8uqGX2bt5",
		Handle: "",
		Pubkeys: &pb.ID_Pubkeys{
			Identity: validIdentity,
			//Bitcoin:  []byte("Ai4YTSiFiBLqNxjV/iLcKilp4iaJCIvnatSf15EV25M2"),
		},
		//BitcoinSig: []byte("MEUCIQC7jvfG23aHIpPjvQjT1unn23PuKNSykh9v/Hc7v3vmoQIgMFI8BBtju7tAgpI66jKAL6PKWGb7jImVBo1DcDoNbpI="),
	}
}

// NewValidPeerInfo returns a PeerInfo example that is known to be valid
func MustNewValidPeerInfo() *repo.PeerInfo {
	var p, err = repo.NewPeerInfoFromProtobuf(MustNewValidPeerIDProtobuf())
	if err != nil {
		panic(err)
	}
	if isValid, errs := p.Valid(); !isValid {
		panic(fmt.Sprintf("invalid peer: %+v", errs))
	}
	return p
}
