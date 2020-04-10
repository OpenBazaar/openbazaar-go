package ipfs

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	routing "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht"
	dhtpb "gx/ipfs/QmSY3nkMNLzh9GdbFKK5tT7YMfLpf52iUZ8ZRkr29MJaa5/go-libp2p-kad-dht/pb"
	ma "gx/ipfs/QmTZBfrPJmjWsCvHEtX5FE6KimVJhsJg5sBbqEFYf4UZtL/go-multiaddr"
	"gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	"gx/ipfs/QmYVXrKrKHDC9FobgmcmshCDyWwdrfwfanNQN4oxJ9Fk3h/go-libp2p-peer"
	ps "gx/ipfs/QmaCTz9RkrU13bm9kMB54f7atgqM4qkjDZpRwRoJiWXEqs/go-libp2p-peerstore"
	"gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"sync"
	"time"
)

const MagicPointerID string = "000000000000000000000000"

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
	return Pointer{Cid: &k, Value: pi}, nil
}

func PublishPointer(dht *routing.IpfsDHT, ctx context.Context, pointer Pointer) error {
	return addPointer(dht, ctx, pointer.Cid, pointer.Value)
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

func PutPointerToPeer(dht *routing.IpfsDHT, ctx context.Context, peer peer.ID, pointer Pointer) error {
	return putPointer(ctx, dht, peer, pointer.Value, pointer.Cid.Bytes())
}

func GetPointersFromPeer(dht *routing.IpfsDHT, ctx context.Context, p peer.ID, key *cid.Cid) ([]*ps.PeerInfo, error) {
	pmes := dhtpb.NewMessage(dhtpb.Message_GET_PROVIDERS, key.Bytes(), 0)
	log.Debugf("Fetching pointers from: %v\n", p.Pretty())
	resp, err := dht.SendRequest(ctx, p, pmes)
	if err != nil {
		return []*ps.PeerInfo{}, err
	}
	return dhtpb.PBPeersToPeerInfos(resp.GetProviderPeers()), nil
}

func addPointer(dht *routing.IpfsDHT, ctx context.Context, k *cid.Cid, pi ps.PeerInfo) error {
	peers, err := dht.GetClosestPeers(ctx, k.KeyString())
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for p := range peers {
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			err := putPointer(ctx, dht, p, pi, k.Bytes())
			if err != nil {
				log.Error(err)
			}
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
	magicBytes, err := hex.DecodeString(MagicPointerID)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, err = hash.Write(entropy)
	if err != nil {
		return "", err
	}
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
