package abi

import "strconv"

// A sequential number assigned to an actor when created by the InitActor.
// This ID is embedded in ID-type addresses.
type ActorID uint64

func (e ActorID) String() string {
	return strconv.FormatInt(int64(e), 10)
}

// MethodNum is an integer that represents a particular method
// in an actor's function table. These numbers are used to compress
// invocation of actor code, and to decouple human language concerns
// about method names from the ability to uniquely refer to a particular
// method.
//
// Consider MethodNum numbers to be similar in concerns as for
// offsets in function tables (in programming languages), and for
// tags in ProtocolBuffer fields. Tags in ProtocolBuffers recommend
// assigning a unique tag to a field and never reusing that tag.
// If a field is no longer used, the field name may change but should
// still remain defined in the code to ensure the tag number is not
// reused accidentally. The same should apply to the MethodNum
// associated with methods in Filecoin VM Actors.
type MethodNum uint64

func (e MethodNum) String() string {
	return strconv.FormatInt(int64(e), 10)
}

// Multiaddrs is a byte array representing a Libp2p MultiAddress
type Multiaddrs = []byte

// PeerID is a byte array representing a Libp2p PeerID
type PeerID = []byte

