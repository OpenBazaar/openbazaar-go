package metrics

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	peer "gx/ipfs/QmTRhk7cgjUf2gfQ3p2M9KPECNZEW9XUrmHcFCgog4cPgB/go-libp2p-peer"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
)

func BenchmarkBandwidthCounter(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		bwc := NewBandwidthCounter()
		round(bwc, b)
	}
}

func round(bwc *BandwidthCounter, b *testing.B) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(10000)
	for i := 0; i < 1000; i++ {
		p := peer.ID(fmt.Sprintf("peer-%d", i))
		for j := 0; j < 10; j++ {
			proto := protocol.ID(fmt.Sprintf("bitswap-%d", j))
			go func() {
				defer wg.Done()
				<-start

				for i := 0; i < 1000; i++ {
					bwc.LogSentMessage(100)
					bwc.LogSentMessageStream(100, proto, p)
					time.Sleep(1 * time.Millisecond)
				}
			}()
		}
	}

	b.StartTimer()
	close(start)
	wg.Wait()
	b.StopTimer()
}

// Allow 7% errors for bw calculations.
const acceptableError = 0.07

func TestBandwidthCounter(t *testing.T) {
	bwc := NewBandwidthCounter()
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(200)
	for i := 0; i < 100; i++ {
		p := peer.ID(fmt.Sprintf("peer-%d", i))
		for j := 0; j < 2; j++ {
			proto := protocol.ID(fmt.Sprintf("proto-%d", j))
			go func() {
				defer wg.Done()
				<-start

				t := time.NewTicker(100 * time.Millisecond)
				defer t.Stop()

				for i := 0; i < 40; i++ {
					bwc.LogSentMessage(100)
					bwc.LogRecvMessage(50)
					bwc.LogSentMessageStream(100, proto, p)
					bwc.LogRecvMessageStream(50, proto, p)
					<-t.C
				}
			}()
		}
	}

	close(start)
	time.Sleep(2*time.Second + 100*time.Millisecond)

	for i := 0; i < 100; i++ {
		stats := bwc.GetBandwidthForPeer(peer.ID(fmt.Sprintf("peer-%d", i)))
		assertApproxEq(t, 2000, stats.RateOut)
		assertApproxEq(t, 1000, stats.RateIn)
	}

	for i := 0; i < 2; i++ {
		stats := bwc.GetBandwidthForProtocol(protocol.ID(fmt.Sprintf("proto-%d", i)))
		assertApproxEq(t, 100000, stats.RateOut)
		assertApproxEq(t, 50000, stats.RateIn)
	}

	{
		stats := bwc.GetBandwidthTotals()
		assertApproxEq(t, 200000, stats.RateOut)
		assertApproxEq(t, 100000, stats.RateIn)
	}

	wg.Wait()
	time.Sleep(1 * time.Second)
	for i := 0; i < 100; i++ {
		stats := bwc.GetBandwidthForPeer(peer.ID(fmt.Sprintf("peer-%d", i)))
		assertEq(t, 8000, stats.TotalOut)
		assertEq(t, 4000, stats.TotalIn)
	}

	for i := 0; i < 2; i++ {
		stats := bwc.GetBandwidthForProtocol(protocol.ID(fmt.Sprintf("proto-%d", i)))
		assertEq(t, 400000, stats.TotalOut)
		assertEq(t, 200000, stats.TotalIn)
	}

	{
		stats := bwc.GetBandwidthTotals()
		assertEq(t, 800000, stats.TotalOut)
		assertEq(t, 400000, stats.TotalIn)
	}
}

func assertEq(t *testing.T, expected, actual int64) {
	if expected != actual {
		t.Errorf("expected  %d, got %d", expected, actual)
	}
}

func assertApproxEq(t *testing.T, expected, actual float64) {
	margin := expected * acceptableError
	if !(math.Abs(expected-actual) <= margin) {
		t.Errorf("expected %f (±%f), got %f", expected, margin, actual)
	}
}
