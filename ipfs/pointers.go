package ipfs

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	routing "gx/ipfs/QmQHnqaNULV8WeUGgh97o9K3KAW6kWQmDyNf9UuikgnPTe/go-libp2p-kad-dht"
	dhtpb "gx/ipfs/QmQHnqaNULV8WeUGgh97o9K3KAW6kWQmDyNf9UuikgnPTe/go-libp2p-kad-dht/pb"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	ps "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	multihash "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"sync"
	"time"

	"github.com/ipfs/go-ipfs/core"
)

const MAGIC string = "000000000000000000000000"

type Purpose int

const (
	MESSAGE   Purpose = 1
	MODERATOR Purpose = 2
	TAG       Purpose = 3
	CHANNEL   Purpose = 4
)

/* A pointer is a custom provider inserted into the DHT which points to a location of a file.
   For offline messaging purposes we use a hash of the recipient's ID as the key and set the
   provider to the location of the ciphertext. We set the Peer ID of the provider object to
   a magic number so we distinguish it from regular providers and use a longer ttl.
   Note this will only be compatible with the OpenBazaar/go-ipfs fork. */
type Pointer struct {
	Cid       cid.Cid
	Value     ps.PeerInfo
	Purpose   Purpose
	Timestamp time.Time
	CancelID  *peer.ID
}

// entropy is a sequence of bytes that should be deterministic based on the content of the pointer
// it is hashed and used to fill the remaining 20 bytes of the magic id
func NewPointer(mhKey multihash.Multihash, prefixLen int, addr ma.Multiaddr, entropy []byte) (Pointer, error) {
	keyhash := CreatePointerKey(mhKey, prefixLen)
	//k, err := cid.Decode(keyhash.B58String())
	//if err != nil {
	//	return Pointer{}, err
	//}

	magicID, err := getMagicID(entropy)
	if err != nil {
		return Pointer{}, err
	}

	arr := []ma.Multiaddr{addr}
	pi := ps.PeerInfo{
		ID:    magicID,
		Addrs: arr,
	}

	t := cid.NewCidV0(keyhash)
	return Pointer{Cid: t, Value: pi}, nil
}

func PublishPointer(node *core.IpfsNode, ctx context.Context, pointer Pointer) error {
	return addPointer(node, ctx, pointer.Cid, pointer.Value)
}

// Fetch pointers from the dht. They will be returned asynchronously.
func FindPointersAsync(dht *routing.IpfsDHT, ctx context.Context, mhKey multihash.Multihash, prefixLen int) <-chan ps.PeerInfo {
	keyhash := CreatePointerKey(mhKey, prefixLen)
	key, _ := cid.Decode(keyhash.B58String())
	peerout := dht.FindProvidersAsync(ctx, key, 100000)
	return peerout
}

// Fetch pointers from the dht
func FindPointers(dht *routing.IpfsDHT, ctx context.Context, mhKey multihash.Multihash, prefixLen int) ([]ps.PeerInfo, error) {
	var providers []ps.PeerInfo
	for p := range FindPointersAsync(dht, ctx, mhKey, prefixLen) {
		providers = append(providers, p)
	}
	return providers, nil
}

func PutPointerToPeer(node *core.IpfsNode, ctx context.Context, peer peer.ID, pointer Pointer) error {
	dht := node.DHT
	return putPointer(ctx, dht, peer, pointer.Value, pointer.Cid.Bytes())
}

func GetPointersFromPeer(node *core.IpfsNode, ctx context.Context, p peer.ID, key cid.Cid) ([]*ps.PeerInfo, error) {
	dht := node.DHT
	pmes := dhtpb.NewMessage(dhtpb.Message_GET_PROVIDERS, key.Bytes(), 0)
	resp, err := dht.SendRequest(ctx, p, pmes)
	if err != nil {
		return []*ps.PeerInfo{}, err
	}
	return dhtpb.PBPeersToPeerInfos(resp.GetProviderPeers()), nil
}

func addPointer(node *core.IpfsNode, ctx context.Context, k cid.Cid, pi ps.PeerInfo) error {
	dht := node.DHT
	peers, err := dht.GetClosestPeers(ctx, k.KeyString())
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for p := range peers {
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			putPointer(ctx, dht, p, pi, k.Bytes())
		}(p)
	}
	wg.Wait()
	return nil
}

func putPointer(ctx context.Context, dht *routing.IpfsDHT, p peer.ID, pi ps.PeerInfo, skey []byte) error {
	pmes := dhtpb.NewMessage(dhtpb.Message_ADD_PROVIDER, skey, 0)
	pmes.ProviderPeers = dhtpb.RawPeerInfosToPBPeers([]ps.PeerInfo{pi})

	err := dht.SendMessage(ctx, p, pmes)
	if err != nil {
		return err
	}
	return nil
}

func CreatePointerKey(mh multihash.Multihash, prefixLen int) multihash.Multihash {
	// Grab the first 8 bytes from the multihash digest
	m, _ := multihash.Decode(mh)
	prefix := m.Digest[:8]

	truncatedPrefix := make([]byte, 8)

	// Prefix to uint64 to shift bits to the right
	prefix64 := binary.BigEndian.Uint64(prefix)

	// Perform the bit shift
	binary.BigEndian.PutUint64(truncatedPrefix, prefix64>>uint(64-prefixLen))

	// Hash the array
	md := sha256.Sum256(truncatedPrefix)

	// Encode as multihash
	keyHash, _ := multihash.Encode(md[:], multihash.SHA2_256)
	return keyHash
}

func getMagicID(entropy []byte) (peer.ID, error) {
	magicBytes, err := hex.DecodeString(MAGIC)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	hash.Write(entropy)
	hashedEntropy := hash.Sum(nil)
	magicBytes = append(magicBytes, hashedEntropy[:20]...)
	h, err := multihash.Encode(magicBytes, multihash.SHA2_256)
	if err != nil {
		return "", err
	}
	id, err := peer.IDFromBytes(h)
	if err != nil {
		return "", err
	}
	return id, nil
}
