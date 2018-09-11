package ipfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"encoding/binary"
	ma "gx/ipfs/QmWWQ2Txc2c6tqjsBpzg5Ar652cHPGNsQQp2SejkNmkUMb/go-multiaddr"
	ps "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	multihash "gx/ipfs/QmZyZDi491cCNTLfAhwcaDii2Kg4pwKRkhqQzURGDvY6ua/go-multihash"

	"github.com/ipfs/go-ipfs/core"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"time"

	routing "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht"
	dhtpb "gx/ipfs/QmRaVcGchmC1stHHK7YhcgEuTk5k1JiGS568pfYWMgT91H/go-libp2p-kad-dht/pb"
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
	Cid       *cid.Cid
	Value     ps.PeerInfo
	Purpose   Purpose
	Timestamp time.Time
	CancelID  *peer.ID
}

// entropy is a sequence of bytes that should be deterministic based on the content of the pointer
// it is hashed and used to fill the remaining 20 bytes of the magic id
func NewPointer(mhKey multihash.Multihash, prefixLen int, addr ma.Multiaddr, entropy []byte) (Pointer, error) {
	keyhash := CreatePointerKey(mhKey, prefixLen)
	k, err := cid.Decode(keyhash.B58String())
	if err != nil {
		return Pointer{}, err
	}

	magicID, err := getMagicID(entropy)
	if err != nil {
		return Pointer{}, err
	}
	pi := ps.PeerInfo{
		ID:    magicID,
		Addrs: []ma.Multiaddr{addr},
	}
	return Pointer{Cid: k, Value: pi}, nil
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
	pointerCh := FindPointersAsync(dht, ctx, mhKey, prefixLen)
	providers := make([]ps.PeerInfo, 0, len(pointerCh))
	for p := range pointerCh {
		providers = append(providers, p)
	}
	return providers, nil
}

func PutPointerToPeer(node *core.IpfsNode, ctx context.Context, peer peer.ID, pointer Pointer) error {
	dht := node.Routing.(*routing.IpfsDHT)
	return putPointer(ctx, dht, peer, pointer.Value, pointer.Cid.KeyString())
}

func GetPointersFromPeer(node *core.IpfsNode, ctx context.Context, p peer.ID, key *cid.Cid) ([]*ps.PeerInfo, error) {
	dht := node.Routing.(*routing.IpfsDHT)
	pmes := dhtpb.NewMessage(dhtpb.Message_GET_PROVIDERS, key.KeyString(), 0)
	resp, err := dht.SendRequest(ctx, p, pmes)
	if err != nil {
		return []*ps.PeerInfo{}, err
	}
	return dhtpb.PBPeersToPeerInfos(resp.GetProviderPeers()), nil
}

func addPointer(node *core.IpfsNode, ctx context.Context, k *cid.Cid, pi ps.PeerInfo) error {
	dht := node.Routing.(*routing.IpfsDHT)
	peers, err := dht.GetClosestPeers(ctx, k.KeyString())
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for p := range peers {
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			putPointer(ctx, dht, p, pi, k.KeyString())
		}(p)
	}
	wg.Wait()
	return nil
}

func putPointer(ctx context.Context, dht *routing.IpfsDHT, p peer.ID, pi ps.PeerInfo, skey string) error {
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
