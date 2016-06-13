package ipfs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	ctxio "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-context/io"
	key "github.com/ipfs/go-ipfs/blocks/key"
	"github.com/ipfs/go-ipfs/core"
	//notif "github.com/ipfs/go-ipfs/notifications"
	routing "github.com/ipfs/go-ipfs/routing/dht"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	host "gx/ipfs/QmVL44QeoQDTYK8RVdpkyja7uYcK3WDNoBNHVLonf9YDtm/go-libp2p/p2p/host"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmYzDkkgAEmrcNzFCiYo6L1dTX4EAG1gZkbtdbd9trL4vd/go-multiaddr"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
	"strconv"
	"sync"
)

const MAGIC string = "000000000000000000000000"

// A pointer is a custom provider inserted into the dht which points to a location of a file.
// For offline messaging purposes we use a hash of the recipient's ID as the key and set the
// provider to the location of the ciphertext. We set the Peer ID of the provider object to
// a magic number so we distinguish it from regular providers and use a longer ttl.
// Note this will only be compatible with the OpenBazaar/go-ipfs fork.
func AddPointer(node *core.IpfsNode, ctx context.Context, mhKey multihash.Multihash, prefixLen int, addr ma.Multiaddr) error {
	keyhash := createKey(mhKey, prefixLen)
	k := key.B58KeyDecode(keyhash.B58String())
	dht := node.Routing.(*routing.IpfsDHT)
	peerHosts := node.PeerHost
	peers, err := dht.GetClosestPeers(ctx, k)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for p := range peers {
		wg.Add(1)
		go func(p peer.ID) {
			defer wg.Done()
			err := putPointer(ctx, peerHosts.(host.Host), p, string(k), addr)
			if err != nil {
				log.Debug(err)
			}
		}(p)
	}
	wg.Wait()
	return nil
}

// Fetch pointers from the dht. They will be returned asynchronously.
func FindPointersAsync(dht *routing.IpfsDHT, ctx context.Context, mhKey multihash.Multihash, prefixLen int) <-chan peer.PeerInfo {
	keyhash := createKey(mhKey, prefixLen)
	peerout := dht.FindProvidersAsync(ctx, key.B58KeyDecode(keyhash.B58String()), 100000)
	return peerout
}

// Fetch pointers from the dht.
func FindPointers(dht *routing.IpfsDHT, ctx context.Context, mhKey multihash.Multihash, prefixLen int) ([]peer.PeerInfo, error) {
	var providers []peer.PeerInfo
	for p := range FindPointersAsync(dht, ctx, mhKey, prefixLen) {
		providers = append(providers, p)
	}
	return providers, nil
}

func putPointer(ctx context.Context, peerHosts host.Host, p peer.ID, skey string, addr ma.Multiaddr) error {
	magicID, err := getMagicID()
	if err != nil {
		return err
	}
	pi := peer.PeerInfo{
		ID:    magicID,
		Addrs: []ma.Multiaddr{addr},
	}

	pmes := pb.NewMessage(pb.Message_ADD_PROVIDER, skey, 0)
	pmes.ProviderPeers = pb.RawPeerInfosToPBPeers([]peer.PeerInfo{pi})
	err = sendMessage(ctx, peerHosts, p, pmes)
	if err != nil {
		return err
	}

	return nil
}

func sendMessage(ctx context.Context, host host.Host, p peer.ID, pmes *pb.Message) error {
	s, err := host.NewStream(ctx, routing.ProtocolDHT, p)
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

func getMagicID() (peer.ID, error) {
	magicBytes, err := hex.DecodeString(MAGIC)
	if err != nil {
		return "", err
	}
	randBytes := make([]byte, 20)
	rand.Read(randBytes)
	magicBytes = append(magicBytes, randBytes[:]...)
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
