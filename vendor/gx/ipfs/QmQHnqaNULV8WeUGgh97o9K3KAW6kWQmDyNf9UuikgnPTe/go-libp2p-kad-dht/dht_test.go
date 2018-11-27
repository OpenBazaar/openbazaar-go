package dht

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	opts "gx/ipfs/QmQHnqaNULV8WeUGgh97o9K3KAW6kWQmDyNf9UuikgnPTe/go-libp2p-kad-dht/opts"
	pb "gx/ipfs/QmQHnqaNULV8WeUGgh97o9K3KAW6kWQmDyNf9UuikgnPTe/go-libp2p-kad-dht/pb"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	ma "gx/ipfs/QmT4U94DnD8FRfqr21obWY32HLM5VExccPKMjQHofeYqr9/go-multiaddr"
	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	bhost "gx/ipfs/QmUDTcnDp2WssbmiDLC6aYurUeyt7QeRakHUQMxA2mZ5iB/go-libp2p/p2p/host/basic"
	kb "gx/ipfs/QmUmemULEGWabBBZxczWCS3AF9g5jDFcxfMXw9iQkZ3EdD/go-libp2p-kbucket"
	swarmt "gx/ipfs/QmVHhT8NxtApPTndiZPe4JNGNUxGWtJe3ebyxtRz4HnbEp/go-libp2p-swarm/testing"
	ci "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil/ci"
	travisci "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil/ci/travis"
	record "gx/ipfs/Qma9Eqp16mNHDX1EL73pcxhFfzbyXVcAYtaDd1xdmDRDtL/go-libp2p-record"
	routing "gx/ipfs/QmcQ81jSyWCp1jpkQ8CMbtpXT3jK7Wg6ZtYmoyWFgBoF9c/go-libp2p-routing"
)

var testCaseValues = map[string][]byte{}
var testCaseCids []cid.Cid

func init() {
	for i := 0; i < 100; i++ {
		v := fmt.Sprintf("%d -- value", i)

		mhv := u.Hash([]byte(v))
		testCaseCids = append(testCaseCids, cid.NewCidV0(mhv))
	}
}

type blankValidator struct{}

func (blankValidator) Validate(_ string, _ []byte) error        { return nil }
func (blankValidator) Select(_ string, _ [][]byte) (int, error) { return 0, nil }

type testValidator struct{}

func (testValidator) Select(_ string, bs [][]byte) (int, error) {
	index := -1
	for i, b := range bs {
		if bytes.Compare(b, []byte("newer")) == 0 {
			index = i
		} else if bytes.Compare(b, []byte("valid")) == 0 {
			if index == -1 {
				index = i
			}
		}
	}
	if index == -1 {
		return -1, errors.New("no rec found")
	}
	return index, nil
}
func (testValidator) Validate(_ string, b []byte) error {
	if bytes.Compare(b, []byte("expired")) == 0 {
		return errors.New("expired")
	}
	return nil
}

func setupDHT(ctx context.Context, t *testing.T, client bool) *IpfsDHT {
	d, err := New(
		ctx,
		bhost.New(swarmt.GenSwarm(t, ctx, swarmt.OptDisableReuseport)),
		opts.Client(client),
		opts.NamespacedValidator("v", blankValidator{}),
	)

	if err != nil {
		t.Fatal(err)
	}
	return d
}

func setupDHTS(ctx context.Context, n int, t *testing.T) ([]ma.Multiaddr, []peer.ID, []*IpfsDHT) {
	addrs := make([]ma.Multiaddr, n)
	dhts := make([]*IpfsDHT, n)
	peers := make([]peer.ID, n)

	sanityAddrsMap := make(map[string]struct{})
	sanityPeersMap := make(map[string]struct{})

	for i := 0; i < n; i++ {
		dhts[i] = setupDHT(ctx, t, false)
		peers[i] = dhts[i].self
		addrs[i] = dhts[i].peerstore.Addrs(dhts[i].self)[0]

		if _, lol := sanityAddrsMap[addrs[i].String()]; lol {
			t.Fatal("While setting up DHTs address got duplicated.")
		} else {
			sanityAddrsMap[addrs[i].String()] = struct{}{}
		}
		if _, lol := sanityPeersMap[peers[i].String()]; lol {
			t.Fatal("While setting up DHTs peerid got duplicated.")
		} else {
			sanityPeersMap[peers[i].String()] = struct{}{}
		}
	}

	return addrs, peers, dhts
}

func connectNoSync(t *testing.T, ctx context.Context, a, b *IpfsDHT) {
	t.Helper()

	idB := b.self
	addrB := b.peerstore.Addrs(idB)
	if len(addrB) == 0 {
		t.Fatal("peers setup incorrectly: no local address")
	}

	a.peerstore.AddAddrs(idB, addrB, pstore.TempAddrTTL)
	pi := pstore.PeerInfo{ID: idB}
	if err := a.host.Connect(ctx, pi); err != nil {
		t.Fatal(err)
	}
}

func wait(t *testing.T, ctx context.Context, a, b *IpfsDHT) {
	t.Helper()

	// loop until connection notification has been received.
	// under high load, this may not happen as immediately as we would like.
	for a.routingTable.Find(b.self) == "" {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond * 5):
		}
	}
}

func connect(t *testing.T, ctx context.Context, a, b *IpfsDHT) {
	t.Helper()
	connectNoSync(t, ctx, a, b)
	wait(t, ctx, a, b)
	wait(t, ctx, b, a)
}

func bootstrap(t *testing.T, ctx context.Context, dhts []*IpfsDHT) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Debugf("Bootstrapping DHTs...")

	// tried async. sequential fares much better. compare:
	// 100 async https://gist.github.com/jbenet/56d12f0578d5f34810b2
	// 100 sync https://gist.github.com/jbenet/6c59e7c15426e48aaedd
	// probably because results compound

	var cfg BootstrapConfig
	cfg = DefaultBootstrapConfig
	cfg.Queries = 3

	start := rand.Intn(len(dhts)) // randomize to decrease bias.
	for i := range dhts {
		dht := dhts[(start+i)%len(dhts)]
		dht.runBootstrap(ctx, cfg)
	}
}

func TestValueGetSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var dhts [5]*IpfsDHT

	for i := range dhts {
		dhts[i] = setupDHT(ctx, t, false)
		defer dhts[i].Close()
		defer dhts[i].host.Close()
	}

	connect(t, ctx, dhts[0], dhts[1])

	t.Log("adding value on: ", dhts[0].self)
	ctxT, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	err := dhts[0].PutValue(ctxT, "/v/hello", []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("requesting value on dhts: ", dhts[1].self)
	ctxT, cancel = context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	val, err := dhts[1].GetValue(ctxT, "/v/hello")
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatalf("Expected 'world' got '%s'", string(val))
	}

	// late connect

	connect(t, ctx, dhts[2], dhts[0])
	connect(t, ctx, dhts[2], dhts[1])

	t.Log("requesting value (offline) on dhts: ", dhts[2].self)
	vala, err := dhts[2].GetValue(ctxT, "/v/hello", Quorum(0))
	if vala != nil {
		t.Fatalf("offline get should have failed, got %s", string(vala))
	}
	if err != routing.ErrNotFound {
		t.Fatalf("offline get should have failed with ErrNotFound, got: %s", err)
	}

	t.Log("requesting value (online) on dhts: ", dhts[2].self)
	val, err = dhts[2].GetValue(ctxT, "/v/hello")
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatalf("Expected 'world' got '%s'", string(val))
	}

	for _, d := range dhts[:3] {
		connect(t, ctx, dhts[3], d)
	}
	connect(t, ctx, dhts[4], dhts[3])

	t.Log("requesting value (requires peer routing) on dhts: ", dhts[4].self)
	val, err = dhts[4].GetValue(ctxT, "/v/hello")
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Fatalf("Expected 'world' got '%s'", string(val))
	}
}

func TestValueSetInvalid(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dhtA := setupDHT(ctx, t, false)
	dhtB := setupDHT(ctx, t, false)

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.host.Close()
	defer dhtB.host.Close()

	dhtA.Validator.(record.NamespacedValidator)["v"] = testValidator{}
	dhtB.Validator.(record.NamespacedValidator)["v"] = blankValidator{}

	connect(t, ctx, dhtA, dhtB)

	testSetGet := func(val string, failset bool, exp string, experr error) {
		t.Helper()

		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		err := dhtA.PutValue(ctxT, "/v/hello", []byte(val))
		if failset {
			if err == nil {
				t.Error("expected set to fail")
			}
		} else {
			if err != nil {
				t.Error(err)
			}
		}

		ctxT, cancel = context.WithTimeout(ctx, time.Second*2)
		defer cancel()
		valb, err := dhtB.GetValue(ctxT, "/v/hello")
		if err != experr {
			t.Errorf("Set/Get %v: Expected %v error but got %v", val, experr, err)
		} else if err == nil && string(valb) != exp {
			t.Errorf("Expected '%v' got '%s'", exp, string(valb))
		}
	}

	// Expired records should not be set
	testSetGet("expired", true, "", routing.ErrNotFound)
	// Valid record should be returned
	testSetGet("valid", false, "valid", nil)
	// Newer record should supersede previous record
	testSetGet("newer", false, "newer", nil)
	// Attempt to set older record again should be ignored
	testSetGet("valid", true, "newer", nil)
}

func TestSearchValue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dhtA := setupDHT(ctx, t, false)
	dhtB := setupDHT(ctx, t, false)

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.host.Close()
	defer dhtB.host.Close()

	connect(t, ctx, dhtA, dhtB)

	dhtA.Validator.(record.NamespacedValidator)["v"] = testValidator{}
	dhtB.Validator.(record.NamespacedValidator)["v"] = testValidator{}

	ctxT, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := dhtA.PutValue(ctxT, "/v/hello", []byte("valid"))
	if err != nil {
		t.Error(err)
	}

	ctxT, cancel = context.WithTimeout(ctx, time.Second*2)
	defer cancel()
	valCh, err := dhtA.SearchValue(ctxT, "/v/hello", Quorum(-1))
	if err != nil {
		t.Fatal(err)
	}

	select {
	case v := <-valCh:
		if string(v) != "valid" {
			t.Errorf("expected 'valid', got '%s'", string(v))
		}
	case <-ctxT.Done():
		t.Fatal(ctxT.Err())
	}

	err = dhtB.PutValue(ctxT, "/v/hello", []byte("newer"))
	if err != nil {
		t.Error(err)
	}

	select {
	case v := <-valCh:
		if string(v) != "newer" {
			t.Errorf("expected 'newer', got '%s'", string(v))
		}
	case <-ctxT.Done():
		t.Fatal(ctxT.Err())
	}
}

func TestGetValues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dhtA := setupDHT(ctx, t, false)
	dhtB := setupDHT(ctx, t, false)

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.host.Close()
	defer dhtB.host.Close()

	connect(t, ctx, dhtA, dhtB)

	ctxT, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := dhtB.PutValue(ctxT, "/v/hello", []byte("newer"))
	if err != nil {
		t.Error(err)
	}

	err = dhtA.PutValue(ctxT, "/v/hello", []byte("valid"))
	if err != nil {
		t.Error(err)
	}

	ctxT, cancel = context.WithTimeout(ctx, time.Second*2)
	defer cancel()
	vals, err := dhtA.GetValues(ctxT, "/v/hello", 16)
	if err != nil {
		t.Fatal(err)
	}

	if len(vals) != 2 {
		t.Fatalf("expected to get 2 values, got %d", len(vals))
	}

	sort.Slice(vals, func(i, j int) bool { return string(vals[i].Val) < string(vals[j].Val) })

	if string(vals[0].Val) != "valid" {
		t.Errorf("unexpected vals[0]: %s", string(vals[0].Val))
	}
	if string(vals[1].Val) != "valid" {
		t.Errorf("unexpected vals[1]: %s", string(vals[1].Val))
	}
}

func TestValueGetInvalid(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dhtA := setupDHT(ctx, t, false)
	dhtB := setupDHT(ctx, t, false)

	defer dhtA.Close()
	defer dhtB.Close()
	defer dhtA.host.Close()
	defer dhtB.host.Close()

	dhtA.Validator.(record.NamespacedValidator)["v"] = blankValidator{}
	dhtB.Validator.(record.NamespacedValidator)["v"] = testValidator{}

	connect(t, ctx, dhtA, dhtB)

	testSetGet := func(val string, exp string, experr error) {
		t.Helper()

		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		err := dhtA.PutValue(ctxT, "/v/hello", []byte(val))
		if err != nil {
			t.Error(err)
		}

		ctxT, cancel = context.WithTimeout(ctx, time.Second*2)
		defer cancel()
		valb, err := dhtB.GetValue(ctxT, "/v/hello")
		if err != experr {
			t.Errorf("Set/Get %v: Expected '%v' error but got '%v'", val, experr, err)
		} else if err == nil && string(valb) != exp {
			t.Errorf("Expected '%v' got '%s'", exp, string(valb))
		}
	}

	// Expired records should not be returned
	testSetGet("expired", "", routing.ErrNotFound)
	// Valid record should be returned
	testSetGet("valid", "valid", nil)
	// Newer record should supersede previous record
	testSetGet("newer", "newer", nil)
	// Attempt to set older record again should be ignored
	testSetGet("valid", "newer", nil)
}

func TestInvalidMessageSenderTracking(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dht := setupDHT(ctx, t, false)
	defer dht.Close()

	foo := peer.ID("asdasd")
	_, err := dht.messageSenderForPeer(foo)
	if err == nil {
		t.Fatal("that shouldnt have succeeded")
	}

	dht.smlk.Lock()
	mscnt := len(dht.strmap)
	dht.smlk.Unlock()

	if mscnt > 0 {
		t.Fatal("should have no message senders in map")
	}
}

func TestProvides(t *testing.T) {
	// t.Skip("skipping test to debug another")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[1], dhts[3])

	for _, k := range testCaseCids {
		log.Debugf("announcing provider for %s", k)
		if err := dhts[3].Provide(ctx, k, true); err != nil {
			t.Fatal(err)
		}
	}

	// what is this timeout for? was 60ms before.
	time.Sleep(time.Millisecond * 6)

	n := 0
	for _, c := range testCaseCids {
		n = (n + 1) % 3

		log.Debugf("getting providers for %s from %d", c, n)
		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		provchan := dhts[n].FindProvidersAsync(ctxT, c, 1)

		select {
		case prov := <-provchan:
			if prov.ID == "" {
				t.Fatal("Got back nil provider")
			}
			if prov.ID != dhts[3].self {
				t.Fatal("Got back wrong provider")
			}
		case <-ctxT.Done():
			t.Fatal("Did not get a provider back.")
		}
	}
}

func TestLocalProvides(t *testing.T) {
	// t.Skip("skipping test to debug another")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[1], dhts[3])

	for _, k := range testCaseCids {
		log.Debugf("announcing provider for %s", k)
		if err := dhts[3].Provide(ctx, k, false); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(time.Millisecond * 10)

	for _, c := range testCaseCids {
		for i := 0; i < 3; i++ {
			provs := dhts[i].providers.GetProviders(ctx, c)
			if len(provs) > 0 {
				t.Fatal("shouldnt know this")
			}
		}
	}
}

// if minPeers or avgPeers is 0, dont test for it.
func waitForWellFormedTables(t *testing.T, dhts []*IpfsDHT, minPeers, avgPeers int, timeout time.Duration) bool {
	// test "well-formed-ness" (>= minPeers peers in every routing table)

	checkTables := func() bool {
		totalPeers := 0
		for _, dht := range dhts {
			rtlen := dht.routingTable.Size()
			totalPeers += rtlen
			if minPeers > 0 && rtlen < minPeers {
				t.Logf("routing table for %s only has %d peers (should have >%d)", dht.self, rtlen, minPeers)
				return false
			}
		}
		actualAvgPeers := totalPeers / len(dhts)
		t.Logf("avg rt size: %d", actualAvgPeers)
		if avgPeers > 0 && actualAvgPeers < avgPeers {
			t.Logf("avg rt size: %d < %d", actualAvgPeers, avgPeers)
			return false
		}
		return true
	}

	timeoutA := time.After(timeout)
	for {
		select {
		case <-timeoutA:
			log.Debugf("did not reach well-formed routing tables by %s", timeout)
			return false // failed
		case <-time.After(5 * time.Millisecond):
			if checkTables() {
				return true // succeeded
			}
		}
	}
}

func printRoutingTables(dhts []*IpfsDHT) {
	// the routing tables should be full now. let's inspect them.
	fmt.Printf("checking routing table of %d\n", len(dhts))
	for _, dht := range dhts {
		fmt.Printf("checking routing table of %s\n", dht.self)
		dht.routingTable.Print()
		fmt.Println("")
	}
}

func TestBootstrap(t *testing.T) {
	// t.Skip("skipping test to debug another")
	if testing.Short() {
		t.SkipNow()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nDHTs := 30
	_, _, dhts := setupDHTS(ctx, nDHTs, t)
	defer func() {
		for i := 0; i < nDHTs; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	t.Logf("connecting %d dhts in a ring", nDHTs)
	for i := 0; i < nDHTs; i++ {
		connect(t, ctx, dhts[i], dhts[(i+1)%len(dhts)])
	}

	<-time.After(100 * time.Millisecond)
	// bootstrap a few times until we get good tables.
	stop := make(chan struct{})
	go func() {
		for {
			t.Logf("bootstrapping them so they find each other %d", nDHTs)
			ctxT, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			bootstrap(t, ctxT, dhts)

			select {
			case <-time.After(50 * time.Millisecond):
				continue // being explicit
			case <-stop:
				return
			}
		}
	}()

	waitForWellFormedTables(t, dhts, 7, 10, 20*time.Second)
	close(stop)

	if u.Debug {
		// the routing tables should be full now. let's inspect them.
		printRoutingTables(dhts)
	}
}

func TestPeriodicBootstrap(t *testing.T) {
	// t.Skip("skipping test to debug another")
	if ci.IsRunning() {
		t.Skip("skipping on CI. highly timing dependent")
	}
	if testing.Short() {
		t.SkipNow()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nDHTs := 30
	_, _, dhts := setupDHTS(ctx, nDHTs, t)
	defer func() {
		for i := 0; i < nDHTs; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	signals := []chan time.Time{}

	var cfg BootstrapConfig
	cfg = DefaultBootstrapConfig
	cfg.Queries = 5

	// kick off periodic bootstrappers with instrumented signals.
	for _, dht := range dhts {
		s := make(chan time.Time)
		signals = append(signals, s)
		proc, err := dht.BootstrapOnSignal(cfg, s)
		if err != nil {
			t.Fatal(err)
		}
		defer proc.Close()
	}

	t.Logf("dhts are not connected. %d", nDHTs)
	for _, dht := range dhts {
		rtlen := dht.routingTable.Size()
		if rtlen > 0 {
			t.Errorf("routing table for %s should have 0 peers. has %d", dht.self, rtlen)
		}
	}

	for i := 0; i < nDHTs; i++ {
		connect(t, ctx, dhts[i], dhts[(i+1)%len(dhts)])
	}

	t.Logf("DHTs are now connected to 1-2 others. %d", nDHTs)
	for _, dht := range dhts {
		rtlen := dht.routingTable.Size()
		if rtlen > 2 {
			t.Errorf("routing table for %s should have at most 2 peers. has %d", dht.self, rtlen)
		}
	}

	if u.Debug {
		printRoutingTables(dhts)
	}

	t.Logf("bootstrapping them so they find each other. %d", nDHTs)
	now := time.Now()
	for _, signal := range signals {
		go func(s chan time.Time) { s <- now }(signal)
	}

	// this is async, and we dont know when it's finished with one cycle, so keep checking
	// until the routing tables look better, or some long timeout for the failure case.
	waitForWellFormedTables(t, dhts, 7, 10, 20*time.Second)

	if u.Debug {
		printRoutingTables(dhts)
	}
}

func TestProvidesMany(t *testing.T) {
	t.Skip("this test doesn't work")
	// t.Skip("skipping test to debug another")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nDHTs := 40
	_, _, dhts := setupDHTS(ctx, nDHTs, t)
	defer func() {
		for i := 0; i < nDHTs; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	t.Logf("connecting %d dhts in a ring", nDHTs)
	for i := 0; i < nDHTs; i++ {
		connect(t, ctx, dhts[i], dhts[(i+1)%len(dhts)])
	}

	<-time.After(100 * time.Millisecond)
	t.Logf("bootstrapping them so they find each other. %d", nDHTs)
	ctxT, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	bootstrap(t, ctxT, dhts)

	if u.Debug {
		// the routing tables should be full now. let's inspect them.
		t.Logf("checking routing table of %d", nDHTs)
		for _, dht := range dhts {
			fmt.Printf("checking routing table of %s\n", dht.self)
			dht.routingTable.Print()
			fmt.Println("")
		}
	}

	providers := make(map[string]peer.ID)

	d := 0
	for _, c := range testCaseCids {
		d = (d + 1) % len(dhts)
		dht := dhts[d]
		providers[c.KeyString()] = dht.self

		t.Logf("announcing provider for %s", c)
		if err := dht.Provide(ctx, c, true); err != nil {
			t.Fatal(err)
		}
	}

	// what is this timeout for? was 60ms before.
	time.Sleep(time.Millisecond * 6)

	errchan := make(chan error)

	ctxT, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	getProvider := func(dht *IpfsDHT, k cid.Cid) {
		defer wg.Done()

		expected := providers[k.KeyString()]

		provchan := dht.FindProvidersAsync(ctxT, k, 1)
		select {
		case prov := <-provchan:
			actual := prov.ID
			if actual == "" {
				errchan <- fmt.Errorf("Got back nil provider (%s at %s)", k, dht.self)
			} else if actual != expected {
				errchan <- fmt.Errorf("Got back wrong provider (%s != %s) (%s at %s)",
					expected, actual, k, dht.self)
			}
		case <-ctxT.Done():
			errchan <- fmt.Errorf("Did not get a provider back (%s at %s)", k, dht.self)
		}
	}

	for _, c := range testCaseCids {
		// everyone should be able to find it...
		for _, dht := range dhts {
			log.Debugf("getting providers for %s at %s", c, dht.self)
			wg.Add(1)
			go getProvider(dht, c)
		}
	}

	// we need this because of printing errors
	go func() {
		wg.Wait()
		close(errchan)
	}()

	for err := range errchan {
		t.Error(err)
	}
}

func TestProvidesAsync(t *testing.T) {
	// t.Skip("skipping test to debug another")
	if testing.Short() {
		t.SkipNow()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[1], dhts[3])

	err := dhts[3].Provide(ctx, testCaseCids[0], true)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 60)

	ctxT, cancel := context.WithTimeout(ctx, time.Millisecond*300)
	defer cancel()
	provs := dhts[0].FindProvidersAsync(ctxT, testCaseCids[0], 5)
	select {
	case p, ok := <-provs:
		if !ok {
			t.Fatal("Provider channel was closed...")
		}
		if p.ID == "" {
			t.Fatal("Got back nil provider!")
		}
		if p.ID != dhts[3].self {
			t.Fatalf("got a provider, but not the right one. %s", p)
		}
	case <-ctxT.Done():
		t.Fatal("Didnt get back providers")
	}
}

func TestLayeredGet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[2], dhts[3])

	err := dhts[3].PutValue(ctx, "/v/hello", []byte("world"))
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond * 6)

	ctxT, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	val, err := dhts[0].GetValue(ctxT, "/v/hello")
	if err != nil {
		t.Fatal(err)
	}

	if string(val) != "world" {
		t.Error("got wrong value")
	}
}

func TestFindPeer(t *testing.T) {
	// t.Skip("skipping test to debug another")
	if testing.Short() {
		t.SkipNow()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			dhts[i].host.Close()
		}
	}()

	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[1], dhts[3])

	ctxT, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	p, err := dhts[0].FindPeer(ctxT, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	if p.ID == "" {
		t.Fatal("Failed to find peer.")
	}

	if p.ID != peers[2] {
		t.Fatal("Didnt find expected peer.")
	}
}

func TestFindPeersConnectedToPeer(t *testing.T) {
	t.Skip("not quite correct (see note)")

	if testing.Short() {
		t.SkipNow()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, peers, dhts := setupDHTS(ctx, 4, t)
	defer func() {
		for i := 0; i < 4; i++ {
			dhts[i].Close()
			dhts[i].host.Close()
		}
	}()

	// topology:
	// 0-1, 1-2, 1-3, 2-3
	connect(t, ctx, dhts[0], dhts[1])
	connect(t, ctx, dhts[1], dhts[2])
	connect(t, ctx, dhts[1], dhts[3])
	connect(t, ctx, dhts[2], dhts[3])

	// fmt.Println("0 is", peers[0])
	// fmt.Println("1 is", peers[1])
	// fmt.Println("2 is", peers[2])
	// fmt.Println("3 is", peers[3])

	ctxT, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	pchan, err := dhts[0].FindPeersConnectedToPeer(ctxT, peers[2])
	if err != nil {
		t.Fatal(err)
	}

	// shouldFind := []peer.ID{peers[1], peers[3]}
	var found []*pstore.PeerInfo
	for nextp := range pchan {
		found = append(found, nextp)
	}

	// fmt.Printf("querying 0 (%s) FindPeersConnectedToPeer 2 (%s)\n", peers[0], peers[2])
	// fmt.Println("should find 1, 3", shouldFind)
	// fmt.Println("found", found)

	// testPeerListsMatch(t, shouldFind, found)

	log.Warning("TestFindPeersConnectedToPeer is not quite correct")
	if len(found) == 0 {
		t.Fatal("didn't find any peers.")
	}
}

func testPeerListsMatch(t *testing.T, p1, p2 []peer.ID) {

	if len(p1) != len(p2) {
		t.Fatal("did not find as many peers as should have", p1, p2)
	}

	ids1 := make([]string, len(p1))
	ids2 := make([]string, len(p2))

	for i, p := range p1 {
		ids1[i] = string(p)
	}

	for i, p := range p2 {
		ids2[i] = string(p)
	}

	sort.Sort(sort.StringSlice(ids1))
	sort.Sort(sort.StringSlice(ids2))

	for i := range ids1 {
		if ids1[i] != ids2[i] {
			t.Fatal("Didnt find expected peer", ids1[i], ids2)
		}
	}
}

func TestConnectCollision(t *testing.T) {
	// t.Skip("skipping test to debug another")
	if testing.Short() {
		t.SkipNow()
	}
	if travisci.IsRunning() {
		t.Skip("Skipping on Travis-CI.")
	}

	runTimes := 10

	for rtime := 0; rtime < runTimes; rtime++ {
		log.Info("Running Time: ", rtime)

		ctx, cancel := context.WithCancel(context.Background())

		dhtA := setupDHT(ctx, t, false)
		dhtB := setupDHT(ctx, t, false)

		addrA := dhtA.peerstore.Addrs(dhtA.self)[0]
		addrB := dhtB.peerstore.Addrs(dhtB.self)[0]

		peerA := dhtA.self
		peerB := dhtB.self

		errs := make(chan error)
		go func() {
			dhtA.peerstore.AddAddr(peerB, addrB, pstore.TempAddrTTL)
			pi := pstore.PeerInfo{ID: peerB}
			err := dhtA.host.Connect(ctx, pi)
			errs <- err
		}()
		go func() {
			dhtB.peerstore.AddAddr(peerA, addrA, pstore.TempAddrTTL)
			pi := pstore.PeerInfo{ID: peerA}
			err := dhtB.host.Connect(ctx, pi)
			errs <- err
		}()

		timeout := time.After(5 * time.Second)
		select {
		case e := <-errs:
			if e != nil {
				t.Fatal(e)
			}
		case <-timeout:
			t.Fatal("Timeout received!")
		}
		select {
		case e := <-errs:
			if e != nil {
				t.Fatal(e)
			}
		case <-timeout:
			t.Fatal("Timeout received!")
		}

		dhtA.Close()
		dhtB.Close()
		dhtA.host.Close()
		dhtB.host.Close()
		cancel()
	}
}

func TestBadProtoMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := setupDHT(ctx, t, false)

	nilrec := new(pb.Message)
	if _, err := d.handlePutValue(ctx, "testpeer", nilrec); err == nil {
		t.Fatal("should have errored on nil record")
	}
}

func TestClientModeConnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := setupDHT(ctx, t, false)
	b := setupDHT(ctx, t, true)

	connectNoSync(t, ctx, a, b)

	c := testCaseCids[0]
	p := peer.ID("TestPeer")
	a.providers.AddProvider(ctx, c, p)
	time.Sleep(time.Millisecond * 5) // just in case...

	provs, err := b.FindProviders(ctx, c)
	if err != nil {
		t.Fatal(err)
	}

	if len(provs) == 0 {
		t.Fatal("Expected to get a provider back")
	}

	if provs[0].ID != p {
		t.Fatal("expected it to be our test peer")
	}
	if a.routingTable.Find(b.self) != "" {
		t.Fatal("DHT clients should not be added to routing tables")
	}
	if b.routingTable.Find(a.self) == "" {
		t.Fatal("DHT server should have been added to the dht client's routing table")
	}
}

func TestClientModeFindPeer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a := setupDHT(ctx, t, false)
	b := setupDHT(ctx, t, true)
	c := setupDHT(ctx, t, true)

	connectNoSync(t, ctx, b, a)
	connectNoSync(t, ctx, c, a)

	// Can't use `connect` because b and c are only clients.
	wait(t, ctx, b, a)
	wait(t, ctx, c, a)

	pi, err := c.FindPeer(ctx, b.self)
	if err != nil {
		t.Fatal(err)
	}
	if len(pi.Addrs) == 0 {
		t.Fatal("should have found addresses for node b")
	}

	err = c.host.Connect(ctx, pi)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFindPeerQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nDHTs := 101
	_, allpeers, dhts := setupDHTS(ctx, nDHTs, t)
	defer func() {
		for i := 0; i < nDHTs; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	mrand := rand.New(rand.NewSource(42))
	guy := dhts[0]
	others := dhts[1:]
	for i := 0; i < 20; i++ {
		for j := 0; j < 16; j++ { // 16, high enough to probably not have any partitions
			v := mrand.Intn(80)
			connect(t, ctx, others[i], others[20+v])
		}
	}

	for i := 0; i < 20; i++ {
		connect(t, ctx, guy, others[i])
	}

	val := "foobar"
	rtval := kb.ConvertKey(val)

	rtablePeers := guy.routingTable.NearestPeers(rtval, AlphaValue)
	if len(rtablePeers) != 3 {
		t.Fatalf("expected 3 peers back from routing table, got %d", len(rtablePeers))
	}

	netpeers := guy.host.Network().Peers()
	if len(netpeers) != 20 {
		t.Fatalf("expected 20 peers to be connected, got %d", len(netpeers))
	}

	rtableset := make(map[peer.ID]bool)
	for _, p := range rtablePeers {
		rtableset[p] = true
	}

	out, err := guy.GetClosestPeers(ctx, val)
	if err != nil {
		t.Fatal(err)
	}

	var notfromrtable int
	var count int
	var outpeers []peer.ID
	for p := range out {
		count++
		if !rtableset[p] {
			notfromrtable++
		}
		outpeers = append(outpeers, p)
	}

	if notfromrtable == 0 {
		t.Fatal("got entirely peers from our routing table")
	}

	if count != 20 {
		t.Fatal("should have only gotten 20 peers from getclosestpeers call")
	}

	sort.Sort(peer.IDSlice(allpeers[1:]))
	sort.Sort(peer.IDSlice(outpeers))

	actualclosest := kb.SortClosestPeers(allpeers[1:], rtval)
	exp := actualclosest[:20]
	got := kb.SortClosestPeers(outpeers, rtval)

	diffp := countDiffPeers(exp, got)
	if diffp > 0 {
		// could be a partition created during setup
		t.Fatal("didnt get expected closest peers")
	}
}

func countDiffPeers(a, b []peer.ID) int {
	s := make(map[peer.ID]bool)
	for _, p := range a {
		s[p] = true
	}
	var out int
	for _, p := range b {
		if !s[p] {
			out++
		}
	}
	return out
}

func TestFindClosestPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nDHTs := 30
	_, _, dhts := setupDHTS(ctx, nDHTs, t)
	defer func() {
		for i := 0; i < nDHTs; i++ {
			dhts[i].Close()
			defer dhts[i].host.Close()
		}
	}()

	t.Logf("connecting %d dhts in a ring", nDHTs)
	for i := 0; i < nDHTs; i++ {
		connect(t, ctx, dhts[i], dhts[(i+1)%len(dhts)])
	}

	peers, err := dhts[1].GetClosestPeers(ctx, "foo")
	if err != nil {
		t.Fatal(err)
	}

	var out []peer.ID
	for p := range peers {
		out = append(out, p)
	}

	if len(out) != KValue {
		t.Fatalf("got wrong number of peers (got %d, expected %d)", len(out), KValue)
	}
}

func TestGetSetPluggedProtocol(t *testing.T) {
	t.Run("PutValue/GetValue - same protocol", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		os := []opts.Option{
			opts.Protocols("/esh/dht"),
			opts.Client(false),
			opts.NamespacedValidator("v", blankValidator{}),
		}

		dhtA, err := New(ctx, bhost.New(swarmt.GenSwarm(t, ctx, swarmt.OptDisableReuseport)), os...)
		if err != nil {
			t.Fatal(err)
		}

		dhtB, err := New(ctx, bhost.New(swarmt.GenSwarm(t, ctx, swarmt.OptDisableReuseport)), os...)
		if err != nil {
			t.Fatal(err)
		}

		connect(t, ctx, dhtA, dhtB)

		ctxT, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := dhtA.PutValue(ctxT, "/v/cat", []byte("meow")); err != nil {
			t.Fatal(err)
		}

		value, err := dhtB.GetValue(ctxT, "/v/cat")
		if err != nil {
			t.Fatal(err)
		}

		if string(value) != "meow" {
			t.Fatalf("Expected 'meow' got '%s'", string(value))
		}
	})

	t.Run("DHT routing table for peer A won't contain B if A and B don't use same protocol", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		dhtA, err := New(ctx, bhost.New(swarmt.GenSwarm(t, ctx, swarmt.OptDisableReuseport)), []opts.Option{
			opts.Protocols("/esh/dht"),
			opts.Client(false),
			opts.NamespacedValidator("v", blankValidator{}),
		}...)
		if err != nil {
			t.Fatal(err)
		}

		dhtB, err := New(ctx, bhost.New(swarmt.GenSwarm(t, ctx, swarmt.OptDisableReuseport)), []opts.Option{
			opts.Protocols("/lsr/dht"),
			opts.Client(false),
			opts.NamespacedValidator("v", blankValidator{}),
		}...)
		if err != nil {
			t.Fatal(err)
		}

		connectNoSync(t, ctx, dhtA, dhtB)

		// We don't expect connection notifications for A to reach B (or vice-versa), given
		// that they've been configured with different protocols - but we'll give them a
		// chance, anyhow.
		time.Sleep(time.Second * 2)

		err = dhtA.PutValue(ctx, "/v/cat", []byte("meow"))
		if err == nil || !strings.Contains(err.Error(), "failed to find any peer in table") {
			t.Fatalf("put should not have been able to find any peers in routing table, err:'%v'", err)
		}

		_, err = dhtB.GetValue(ctx, "/v/cat")
		if err == nil || !strings.Contains(err.Error(), "failed to find any peer in table") {
			t.Fatalf("get should not have been able to find any peers in routing table, err:'%v'", err)
		}
	})
}
