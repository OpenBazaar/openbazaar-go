package flow

import (
	"fmt"
	"math"
	"time"
)

func ExampleMeter() {
	meter := new(Meter)
	t := time.NewTicker(100 * time.Millisecond)
	for i := 0; i < 100; i++ {
		<-t.C
		meter.Mark(30)
	}

	// Get the current rate. This will be accurate *now* but not after we
	// sleep (because we calculate it using EWMA).
	rate := meter.Snapshot().Rate

	// Sleep 2 seconds to allow the total to catch up. We snapshot every
	// second so the total may not yet be accurate.
	time.Sleep(2 * time.Second)

	// Get the current total.
	total := meter.Snapshot().Total

	fmt.Printf("%d (%d/s)\n", total, int64(math.Round(float64(rate)/10)))
	// Output: 3000 (300/s)
}
