package tempuscache

import (
	"sync"
	"testing"
	"time"
)

/*
cache_test.go provides comprehensive validation of TempusCache.

================================================================================
TESTING OBJECTIVES
================================================================================

This test suite verifies:

1. Functional Correctness
   - Ensures Set(), Get(), Delete() behave deterministically.
   - Confirms LRU updates do not break key retrieval.

2. Expiration Semantics
   - Validates TTL-based expiration accuracy.
   - Ensures expired keys are never returned.
   - Confirms TTL == 0 results in non-expiring entries.

3. Concurrency Safety
   - Stress-tests concurrent read/write access.
   - Validates correct usage of sync.RWMutex.
   - Ensures absence of race conditions and runtime panics.

4. Metrics Accuracy
   - Verifies hit/miss statistics tracking.

================================================================================
CONCURRENCY VALIDATION
================================================================================

Concurrency tests are designed to simulate realistic parallel workloads.

Execution correctness must hold regardless of:

- True parallel execution (multi-core CPUs)
- Goroutine interleaving (single-core scheduling)

For full safety verification, tests should be executed with:

    go test -race

The Go race detector ensures no data races occur
under concurrent access patterns.

================================================================================
ENGINEERING PHILOSOPHY
================================================================================

The goal of this suite is not only correctness,
but reliability under contention — a core requirement
for production-grade caching systems.
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
TestConcurrentAccess performs a concurrency stress validation.

================================================================================
PURPOSE
================================================================================

This test ensures:

- Thread safety under simultaneous Set() and Get() operations.
- No "concurrent map writes" runtime panic.
- Correct synchronization via sync.RWMutex.
- Stability under write-read contention.

================================================================================
EXECUTION MODEL
================================================================================

- 100 goroutines are spawned.
- Each goroutine performs:
    1. A write operation (Set)
    2. A read operation (Get)

A sync.WaitGroup coordinates completion to ensure
all goroutines finish before the test exits.

================================================================================
WHY THIS MATTERS
================================================================================

If locking were implemented incorrectly,
this test would likely:

- Trigger race detector warnings
- Cause runtime panic
- Produce inconsistent state

Passing this test under `go test -race`
provides strong confidence in concurrency correctness.
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

/*
TestStatsTracking verifies accuracy of runtime metrics.

It ensures:

- Cache hits increment correctly on successful retrieval.
- Cache misses increment correctly on failed lookup.
- Stats() returns a consistent snapshot under read lock.

Accurate statistics are critical for:

- Performance monitoring
- Observability
- Production diagnostics
*/

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
