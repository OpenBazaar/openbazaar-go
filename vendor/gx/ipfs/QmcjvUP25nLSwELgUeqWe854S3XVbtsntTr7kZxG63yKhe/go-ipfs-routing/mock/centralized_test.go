package mockrouting

import (
	"context"
	"testing"
	"time"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	u "gx/ipfs/QmPdKqUcHGFdeSpvjVoaTRPPstGif9GBZb5Q56RVw9o69A/go-ipfs-util"
	delay "gx/ipfs/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
	pstore "gx/ipfs/QmTTJcDL3gsnGDALjh2fDGg1onGRUdVgNL2hU2WEZcVrMX/go-libp2p-peerstore"
	testutil "gx/ipfs/Qma6ESRQTf1ZLPgzpCwDTqQJefPnU6uLvMjP18vK8EWp8L/go-testutil"
)

func TestKeyNotFound(t *testing.T) {

	var pi = testutil.RandIdentityOrFatal(t)
	var key = cid.NewCidV0(u.Hash([]byte("mock key")))
	var ctx = context.Background()

	rs := NewServer()
	providers := rs.Client(pi).FindProvidersAsync(ctx, key, 10)
	_, ok := <-providers
	if ok {
		t.Fatal("should be closed")
	}
}

func TestClientFindProviders(t *testing.T) {
	pi := testutil.RandIdentityOrFatal(t)
	rs := NewServer()
	client := rs.Client(pi)

	k := cid.NewCidV0(u.Hash([]byte("hello")))
	err := client.Provide(context.Background(), k, true)
	if err != nil {
		t.Fatal(err)
	}

	// This is bad... but simulating networks is hard
	time.Sleep(time.Millisecond * 300)
	max := 100

	providersFromClient := client.FindProvidersAsync(context.Background(), k, max)
	isInClient := false
	for pi := range providersFromClient {
		if pi.ID == pi.ID { // <-- typo?
			isInClient = true
		}
	}
	if !isInClient {
		t.Fatal("Despite client providing key, client didn't receive peer when finding providers")
	}
}

func TestClientOverMax(t *testing.T) {
	rs := NewServer()
	k := cid.NewCidV0(u.Hash([]byte("hello")))
	numProvidersForHelloKey := 100
	for i := 0; i < numProvidersForHelloKey; i++ {
		pi := testutil.RandIdentityOrFatal(t)
		err := rs.Client(pi).Provide(context.Background(), k, true)
		if err != nil {
			t.Fatal(err)
		}
	}

	max := 10
	pi := testutil.RandIdentityOrFatal(t)
	client := rs.Client(pi)

	providersFromClient := client.FindProvidersAsync(context.Background(), k, max)
	i := 0
	for range providersFromClient {
		i++
	}
	if i != max {
		t.Fatal("Too many providers returned")
	}
}

// TODO does dht ensure won't receive self as a provider? probably not.
func TestCanceledContext(t *testing.T) {
	rs := NewServer()
	k := cid.NewCidV0(u.Hash([]byte("hello")))

	// avoid leaking goroutine, without using the context to signal
	// (we want the goroutine to keep trying to publish on a
	// cancelled context until we've tested it doesnt do anything.)
	done := make(chan struct{})
	defer func() { done <- struct{}{} }()

	t.Log("async'ly announce infinite stream of providers for key")
	i := 0
	go func() { // infinite stream
		for {
			select {
			case <-done:
				t.Log("exiting async worker")
				return
			default:
			}

			pi, err := testutil.RandIdentity()
			if err != nil {
				t.Error(err)
			}
			err = rs.Client(pi).Provide(context.Background(), k, true)
			if err != nil {
				t.Error(err)
			}
			i++
		}
	}()

	local := testutil.RandIdentityOrFatal(t)
	client := rs.Client(local)

	t.Log("warning: max is finite so this test is non-deterministic")
	t.Log("context cancellation could simply take lower priority")
	t.Log("and result in receiving the max number of results")
	max := 1000

	t.Log("cancel the context before consuming")
	ctx, cancelFunc := context.WithCancel(context.Background())
	cancelFunc()
	providers := client.FindProvidersAsync(ctx, k, max)

	numProvidersReturned := 0
	for range providers {
		numProvidersReturned++
	}
	t.Log(numProvidersReturned)

	if numProvidersReturned == max {
		t.Fatal("Context cancel had no effect")
	}
}

func TestValidAfter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pi := testutil.RandIdentityOrFatal(t)
	key := cid.NewCidV0(u.Hash([]byte("mock key")))
	conf := DelayConfig{
		ValueVisibility: delay.Fixed(1 * time.Hour),
		Query:           delay.Fixed(0),
	}

	rs := NewServerWithDelay(conf)

	rs.Client(pi).Provide(ctx, key, true)

	var providers []pstore.PeerInfo
	max := 100
	providersChan := rs.Client(pi).FindProvidersAsync(ctx, key, max)
	for p := range providersChan {
		providers = append(providers, p)
	}
	if len(providers) > 0 {
		t.Fail()
	}

	conf.ValueVisibility.Set(0)
	time.Sleep(100 * time.Millisecond)

	providersChan = rs.Client(pi).FindProvidersAsync(ctx, key, max)
	t.Log("providers", providers)
	for p := range providersChan {
		providers = append(providers, p)
	}
	if len(providers) != 1 {
		t.Fail()
	}
}
