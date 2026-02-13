package tempuscache

import (
	"sync"
	"testing"
	"time"
)

/*
cache_test.go validates the correctness, reliability,
and concurrency safety of the TempusCache implementation.

TEST COVERAGE

1. Basic correctness:
   Ensures Set() and Get() behave as expected.

2. TTL behavior:
   Validates that keys expire correctly when TTL elapses.

3. No-expiration behavior:
   Confirms that TTL == 0 results in persistent entries.

4. Explicit deletion:
   Verifies Delete() removes keys safely.

5. Concurrency safety:
   Stress-tests the cache under concurrent read/write access
   to ensure no race conditions or runtime panics occur.

NOTE ON PARALLEL EXECUTION

Parallel execution depends on CPU core availability.
Goroutines may run concurrently (multi-core) or interleave
(multi-tasking on single core), but correctness must hold
in both cases.

These tests should be executed with:

    go test -race

to ensure no data races are present.
*/

func TestSetAndGet(t *testing.T) {
	cache := New()

	cache.Set("a", "b", 5*time.Second)

	val, found := cache.Get("a")
	if !found {
		t.Fatal("expected key to be found")
	}

	if val != "b" {
		t.Fatalf("expected 'b', got %v", val)
	}
}

func TestExpiration(t *testing.T) {
	cache := New()

	cache.Set("a", "b", 1*time.Millisecond)
	time.Sleep(2 * time.Millisecond)

	_, found := cache.Get("a")
	if found {
		t.Fatal("expected key to be expired")
	}
}

func TestNoExpiration(t *testing.T) {
	cache := New()

	cache.Set("a", "b", 0)

	time.Sleep(2 * time.Millisecond)

	val, found := cache.Get("a")
	if !found || val != "b" {
		t.Fatal("expected key to persist without TTL")
	}
}

func TestDelete(t *testing.T) {
	cache := New()

	cache.Set("a", "b", 5*time.Second)
	cache.Delete("a")

	_, found := cache.Get("a")
	if found {
		t.Fatal("expected key to be deleted")
	}
}

/*
TestConcurrentAccess performs a concurrency stress test.

PURPOSE

- Simulates multiple goroutines accessing the cache simultaneously.
- Validates correctness of sync.RWMutex usage.
- Ensures no concurrent map write panic occurs.
- Verifies system stability under parallel workload.

MECHANISM

A sync.WaitGroup is used to wait for all goroutines to finish.
Each goroutine:
    1. Performs a write (Set)
    2. Performs a read (Get)

This creates realistic read/write contention.

If the mutex protection were incorrect,
this test would likely panic or fail under race detection.
*/

func TestConcurrentAccess(t *testing.T) {
	cache := New()
	var wg sync.WaitGroup //WaitGroup waits for collection of goroutines to finish.

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cache.Set("key", i, 5*time.Second)
			cache.Get("key")
		}(i)
	}

	wg.Wait() // ENsures all goroutines cpmpletes before test exits.
}

func TestStatsTracking(t *testing.T) {
	cache := New()

	cache.Set("a", 1, 0)

	cache.Get("a") // hit
	cache.Get("b") // miss

	stats := cache.Stats()

	if stats.Hits != 1 {
		t.Fatalf("expected 1 hit, got %d", stats.Hits)
	}

	if stats.Misses != 1 {
		t.Fatalf("expected 1 miss, got %d", stats.Misses)
	}
}
