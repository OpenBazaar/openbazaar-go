package flow

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(40 * time.Millisecond)
			defer ticker.Stop()

			m := new(Meter)
			for i := 0; i < 100; i++ {
				m.Mark(1000)
				<-ticker.C
			}
			actual := m.Snapshot()
			if !approxEq(actual.Rate, 25000, 500) {
				t.Errorf("expected rate 25000 (±500), got %f", actual.Rate)
			}

			for i := 0; i < 200; i++ {
				m.Mark(200)
				<-ticker.C
			}

			// Adjusts
			actual = m.Snapshot()
			if !approxEq(actual.Rate, 5000, 200) {
				t.Errorf("expected rate 5000 (±200), got %f", actual.Rate)
			}

			// Let it settle.
			time.Sleep(2 * time.Second)

			// get the right total
			actual = m.Snapshot()
			if actual.Total != 140000 {
				t.Errorf("expected total %d, got %d", 120000, actual.Total)
			}
		}()
	}
	wg.Wait()
}

func TestShared(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(20 * 21)
	for i := 0; i < 20; i++ {
		m := new(Meter)
		for j := 0; j < 20; j++ {
			go func() {
				defer wg.Done()
				ticker := time.NewTicker(40 * time.Millisecond)
				defer ticker.Stop()
				for i := 0; i < 100; i++ {
					m.Mark(50)
					<-ticker.C
				}

				for i := 0; i < 200; i++ {
					m.Mark(10)
					<-ticker.C
				}
			}()
		}
		go func() {
			defer wg.Done()
			time.Sleep(40 * 100 * time.Millisecond)
			actual := m.Snapshot()
			if !approxEq(actual.Rate, 25000, 250) {
				t.Errorf("expected rate 25000 (±250), got %f", actual.Rate)
			}

			time.Sleep(40 * 200 * time.Millisecond)

			// Adjusts
			actual = m.Snapshot()
			if !approxEq(actual.Rate, 5000, 50) {
				t.Errorf("expected rate 5000 (±50), got %f", actual.Rate)
			}

			// Let it settle.
			time.Sleep(2 * time.Second)

			// get the right total
			actual = m.Snapshot()
			if actual.Total != 140000 {
				t.Errorf("expected total %d, got %d", 140000, actual.Total)
			}
		}()
	}
	wg.Wait()
}

func TestUnregister(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(100 * 2)
	pause := make(chan struct{})

	for i := 0; i < 100; i++ {
		m := new(Meter)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for i := 0; i < 40; i++ {
				m.Mark(1)
				<-ticker.C
			}

			<-pause
			time.Sleep(2 * time.Second)

			for i := 0; i < 40; i++ {
				m.Mark(2)
				<-ticker.C
			}
		}()
		go func() {
			defer wg.Done()
			time.Sleep(40 * 100 * time.Millisecond)

			actual := m.Snapshot()
			if !approxEq(actual.Rate, 10, 1) {
				t.Errorf("expected rate 10 (±1), got %f", actual.Rate)
			}

			<-pause

			actual = m.Snapshot()
			if actual.Total != 40 {
				t.Errorf("expected total 4000, got %d", actual.Total)
			}
			time.Sleep(2*time.Second + 40*100*time.Millisecond)

			actual = m.Snapshot()
			if !approxEq(actual.Rate, 20, 4) {
				t.Errorf("expected rate 20 (±4), got %f", actual.Rate)
			}
			time.Sleep(2 * time.Second)
			actual = m.Snapshot()
			if actual.Total != 120 {
				t.Errorf("expected total 120, got %d", actual.Total)
			}
		}()

	}
	time.Sleep(60 * time.Second)
	globalSweeper.mutex.Lock()
	if len(globalSweeper.meters) != 0 {
		t.Errorf("expected all sweepers to be unregistered: %d", len(globalSweeper.meters))
	}
	globalSweeper.mutex.Unlock()
	close(pause)

	wg.Wait()

	globalSweeper.mutex.Lock()
	if len(globalSweeper.meters) != 100 {
		t.Errorf("expected all sweepers to be registered: %d", len(globalSweeper.meters))
	}
	globalSweeper.mutex.Unlock()
}

func approxEq(a, b, err float64) bool {
	return math.Abs(a-b) < err
}
