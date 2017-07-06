package testutil

import (
	"context"
	"testing"

	inet "gx/ipfs/QmRscs8KxrSmSv4iuevHv8JfuUzHBMoqiaHzxfDRiksd6e/go-libp2p-net"
	swarm "gx/ipfs/QmVkDnNm71vYyY6s6rXwtmyDYis3WkKyrEhMECwT6R12uJ/go-libp2p-swarm"
	pstore "gx/ipfs/QmXZSd1qR5BxZkPyuwfT5jpqQFScZccoZvDneXsKzCNHWX/go-libp2p-peerstore"
	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	tu "gx/ipfs/QmdVnYKahrvndXhyreWhY8YT3a5chJoWv8b4wVyH9JG2KB/go-testutil"
)

func GenSwarmNetwork(t *testing.T, ctx context.Context) *swarm.Network {
	p := tu.RandPeerNetParamsOrFatal(t)
	ps := pstore.NewPeerstore()
	ps.AddPubKey(p.ID, p.PubKey)
	ps.AddPrivKey(p.ID, p.PrivKey)
	n, err := swarm.NewNetwork(ctx, []ma.Multiaddr{p.Addr}, p.ID, ps, nil)
	if err != nil {
		t.Fatal(err)
	}
	ps.AddAddrs(p.ID, n.ListenAddresses(), pstore.PermanentAddrTTL)
	return n
}

func DivulgeAddresses(a, b inet.Network) {
	id := a.LocalPeer()
	addrs := a.Peerstore().Addrs(id)
	b.Peerstore().AddAddrs(id, addrs, pstore.PermanentAddrTTL)
}

var RandPeerID = tu.RandPeerID
