package avatar_generator

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestFlightCoalescing covers leader election directly: one caller renders,
// everyone else waits on the same flight, and the key is released afterwards.
func TestFlightCoalescing(t *testing.T) {
	c := NewCache(DefaultCacheConfig())

	leader, isLeader := c.beginFlight("key")
	if !isLeader {
		t.Fatal("first caller was not made leader")
	}

	waiter, isLeader := c.beginFlight("key")
	if isLeader {
		t.Fatal("second caller became a second leader")
	}
	if waiter != leader {
		t.Fatal("waiters did not join the same flight")
	}

	c.endFlight("key", []byte("avatar-bytes"), nil)
	<-waiter.done // returned immediately because endFlight closed it

	if string(waiter.png) != "avatar-bytes" || waiter.err != nil {
		t.Errorf("waiter got (%q, %v)", waiter.png, waiter.err)
	}

	if _, free := c.beginFlight("key"); !free {
		t.Error("key was not released after the flight finished")
	}
}

// TestCacheCoalescesConcurrentMisses proves a burst of identical requests never
// produces different bytes, and that every request after the burst hits.
func TestCacheCoalescesConcurrentMisses(t *testing.T) {
	c := NewCache(CacheConfig{TTL: time.Minute, MaxItems: 16})

	const workers = 10

	var (
		start   = make(chan struct{})
		wg      sync.WaitGroup
		results = make([][]byte, workers)
		errs    = make([]error, workers)
		hits    atomic.Int32
	)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			results[w], _, errs[w] = c.PNG("burst", 256, DefaultPaddingPct)
			if _, hit, _ := c.PNG("burst", 256, DefaultPaddingPct); hit {
				hits.Add(1)
			}
		}(w)
	}

	close(start)
	wg.Wait()

	for w, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", w, err)
		}
		if !bytes.Equal(results[w], results[0]) {
			t.Errorf("worker %d got different bytes during coalescing", w)
		}
	}
	if hits.Load() != workers {
		t.Errorf("hits after the burst = %d, want %d", hits.Load(), workers)
	}

	c.flightsMu.Lock()
	open := len(c.flights)
	c.flightsMu.Unlock()
	if open != 0 {
		t.Errorf("%d flights still open after the burst", open)
	}
}

// TestCacheFlightClearedOnError makes sure a failed render does not leave the
// key permanently locked.
func TestCacheFlightClearedOnError(t *testing.T) {
	c := NewCache(DefaultCacheConfig())

	long := make([]byte, MaxIDLength+1)
	for i := range long {
		long[i] = 'a'
	}

	// Rejected before the flight begins, so the map stays empty.
	if _, _, err := c.PNG(string(long), 64, DefaultPaddingPct); err == nil {
		t.Fatal("expected ErrIDTooLong")
	}
	c.flightsMu.Lock()
	open := len(c.flights)
	c.flightsMu.Unlock()
	if open != 0 {
		t.Errorf("flights = %d after a rejected request, want 0", open)
	}

	if _, _, err := c.PNG("after-error", 64, DefaultPaddingPct); err != nil {
		t.Fatalf("cache still usable: %v", err)
	}
}
