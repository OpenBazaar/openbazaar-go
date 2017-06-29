package ipfs

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"sync"

	ma "gx/ipfs/QmSWLfmj5frN9xVLMMN846dMDriy5wN5jeghUm7aTW3DAG/go-multiaddr"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
	host "gx/ipfs/QmXzeAcmKDTfNZQBiyF22hQKuTK7P5z6MBBQLTk9bbiSUc/go-libp2p-host"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	multihash "gx/ipfs/QmbZ6Cee2uHjG7hf19qLHppgKDRtaG4CVtMzdmK9VCVqLu/go-multihash"
	ps "gx/ipfs/Qme1g4e3m2SmdiSGGU3vSWmUStwUjc5oECnEriaK9Xa1HU/go-libp2p-peerstore"

	"github.com/ipfs/go-ipfs/core"

	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
	"time"

	routing "github.com/ipfs/go-ipfs/routing/dht"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	ctxio "github.com/jbenet/go-context/io"
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
func PublishPointer(node *core.IpfsNode, ctx context.Context, mhKey multihash.Multihash, prefixLen int, addr ma.Multiaddr, entropy []byte) (Pointer, error) {
	keyhash := createKey(mhKey, prefixLen)
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
	return Pointer{Cid: k, Value: pi}, addPointer(node, ctx, k, pi)
}

func RePublishPointer(node *core.IpfsNode, ctx context.Context, pointer Pointer) error {
	return addPointer(node, ctx, pointer.Cid, pointer.Value)
}

// Fetch pointers from the dht. They will be returned asynchronously.
func FindPointersAsync(dht *routing.IpfsDHT, ctx context.Context, mhKey multihash.Multihash, prefixLen int) <-chan ps.PeerInfo {
	keyhash := createKey(mhKey, prefixLen)
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

func addPointer(node *core.IpfsNode, ctx context.Context, k *cid.Cid, pi ps.PeerInfo) error {
	dht := node.Routing.(*routing.IpfsDHT)
	peerHosts := node.PeerHost
	peers, err := dht.GetClosestPeers(ctx, k.KeyString())
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for p := range peers {
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			putPointer(ctx, peerHosts.(host.Host), p, pi, k.KeyString())
		}(p)
	}
	wg.Wait()
	return nil
}

func putPointer(ctx context.Context, peerHosts host.Host, p peer.ID, pi ps.PeerInfo, skey string) error {
	pmes := pb.NewMessage(pb.Message_ADD_PROVIDER, skey, 0)
	pmes.ProviderPeers = pb.RawPeerInfosToPBPeers([]ps.PeerInfo{pi})

	err := sendMessage(ctx, peerHosts, p, pmes)
	if err != nil {
		return err
	}
	return nil
}

func sendMessage(ctx context.Context, host host.Host, p peer.ID, pmes *pb.Message) error {
	s, err := host.NewStream(ctx, p, routing.ProtocolDHT)
	if err != nil {
		return err
	}
	defer s.Close()

	cw := ctxio.NewWriter(ctx, s)
	w := ggio.NewDelimitedWriter(cw)

	if err := w.WriteMsg(pmes); err != nil {
		return err
	}
	return nil
}

func createKey(mh multihash.Multihash, prefixLen int) multihash.Multihash {
	// Grab the first 8 bytes from the multihash digest
	m, _ := multihash.Decode(mh)
	prefix64 := binary.BigEndian.Uint64(m.Digest[:8])

	// Convert to binary string
	bin := strconv.FormatUint(prefix64, 2)

	// Pad with leading zeros
	leadingZeros := 64 - len(bin)
	for i := 0; i < leadingZeros; i++ {
		bin = "0" + bin
	}

	// Grab the bits corresponding to the prefix length and convert to int
	intPrefix, _ := strconv.ParseUint(bin[:prefixLen], 2, 64)

	// Convert to 8 byte array
	bs := make([]byte, 8)
	binary.BigEndian.PutUint64(bs, intPrefix)

	// Hash the array
	hash := sha256.New()
	hash.Write(bs)
	md := hash.Sum(nil)

	// Encode as multihash
	keyHash, _ := multihash.Encode(md, multihash.SHA2_256)
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
