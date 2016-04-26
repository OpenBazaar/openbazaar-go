package ipfs

import (
	"sync"
	"crypto/sha256"
	"encoding/hex"
	"github.com/ipfs/go-ipfs/core"
	"gx/ipfs/QmYgaiNVVL7f2nydijAwpDRunRkmxfu3PoK87Y3pH84uAW/go-libp2p/p2p/host"
	routing "github.com/ipfs/go-ipfs/routing/dht"
	pb "github.com/ipfs/go-ipfs/routing/dht/pb"
	multihash "gx/ipfs/QmYf7ng2hG5XBtJA3tN34DQ2GUN5HNksEw1rLDkmr6vGku/go-multihash"
	ma "gx/ipfs/QmcobAGsCjYt5DXoq9et9L8yR8er7o7Cu3DTvpaq12jYSz/go-multiaddr"
	peer "gx/ipfs/QmZwZjMVGss5rqYsJVGy18gNbkTJffFyq2x1uJ4e4p3ZAt/go-libp2p-peer"
	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	ctxio "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-context/io"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	key "github.com/ipfs/go-ipfs/blocks/key"
)

const MAGIC string = "0000000000000000000000000000000000000000000000000000000000000000"

// A pointer is a custom provider inserted into the dht which points to a location of a file.
// For offline messaging purposes we use a hash of the recipient's ID as the key and set the
// provider to the location of the ciphertext. We set the Peer ID of the provider object to
// a magic number so we distinguish it from regular providers and use a longer ttl.
// Note this will only be compatible with the OpenBazaar/go-ipfs fork.
func AddPointer(node *core.IpfsNode, ctx context.Context, skey string, addr ma.Multiaddr) error {
	keyhash := hashStringToKey(skey)
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
			err := putPointer(ctx, peerHosts, p, string(k), addr)
			if err != nil {
				log.Debug(err)
			}
		}(p)
	}
	wg.Wait()
	return nil
}

// Fetch pointers from the dht. They will be returned asynchronously.
func FindPointersAsync(dht *routing.IpfsDHT, ctx context.Context, skey string) <-chan peer.PeerInfo {
	keyhash := hashStringToKey(skey)
	peerout := dht.FindProvidersAsync(ctx, key.Key(keyhash.B58String()), 100000)
	return peerout
}

func putPointer(ctx context.Context, peerHosts host.Host, p peer.ID, skey string, addr ma.Multiaddr) error{
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
	log.Debugf("Sending PutProvider message to %s", p)
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

func hashStringToKey(s string) multihash.Multihash {
	hash := sha256.New()
	hash.Write([]byte(s))
	md := hash.Sum(nil)
	keyHash, _ := multihash.Encode(md, multihash.SHA2_256)
	var mhKey multihash.Multihash = keyHash
	return mhKey
}

func getMagicID() (peer.ID, error){
	magicBytes, err := hex.DecodeString(MAGIC)
	if err != nil {
		return "", err
	}
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