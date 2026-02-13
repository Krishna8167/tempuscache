package tempuscache

import "time"

/*
startJanitor initializes and launches the background expiration worker.

================================================================================
ROLE IN CACHE LIFECYCLE
================================================================================

TempusCache implements a dual-expiration strategy:

1. Lazy Expiration
   - Expired keys are removed during Get() calls.

2. Active Expiration (Janitor)
   - Periodically scans and removes expired entries,
     even if they are never accessed again.

The janitor ensures bounded memory growth in workloads
where expired keys are rarely read.

================================================================================
EXECUTION MODEL
================================================================================

- If interval <= 0:
    → Active cleanup is disabled.
    → Cache relies solely on lazy expiration.

- If interval > 0:
    → A time.Ticker is created.
    → A dedicated goroutine is launched.
    → On each tick:
          deleteExpired() is executed.

The goroutine runs independently of caller threads
and operates asynchronously.

================================================================================
CONCURRENCY & SAFETY
================================================================================

- deleteExpired() acquires an exclusive Lock()
  because it mutates internal structures.

- stopChan is used as a lifecycle control signal
  for graceful shutdown.

- The ticker is explicitly stopped before exit
  to prevent resource leakage.

================================================================================
PERFORMANCE CHARACTERISTICS
================================================================================

Each cleanup cycle performs an O(n) scan
over cache entries (via LRU traversal).

This approach is acceptable for moderate cache sizes.
For large-scale systems, further optimization strategies
could include:

- Min-heap scheduling by expiration
- Time-wheel algorithms
- Sharded expiration workers

================================================================================
DESIGN PHILOSOPHY
================================================================================

The janitor is intentionally simple and predictable,
favoring clarity and correctness over premature optimization.
*/

func (c *Cache) startJanitor() {
	if c.interval <= 0 {
		return
	}

	ticker := time.NewTicker(c.interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				c.deleteExpired()
			case <-c.stopChan:
				ticker.Stop() //You stop the ticker before returning , because ticker leaks resources if not stopped.
				return
			}
		}
	}()
}

/*
Stop gracefully terminates the background janitor goroutine.

================================================================================
SHUTDOWN MECHANISM
================================================================================

- Closing stopChan signals the janitor to exit.
- The goroutine responds by:
    1. Stopping the ticker.
    2. Returning cleanly.

This prevents:

- Goroutine leaks
- Ticker resource leaks
- Background CPU usage after cache disposal

================================================================================
USAGE CONTRACT
================================================================================

Stop should be called once per Cache lifecycle.

IMPORTANT:
Calling Stop multiple times will cause a panic,
since closing an already closed channel is illegal in Go.

================================================================================
WHY THIS MATTERS
================================================================================

In long-running applications (e.g., HTTP servers,
microservices, background workers), failing to stop
background routines can lead to subtle resource leaks.

This method enables safe integration into
production-grade systems.
*/

func (c *Cache) Stop() {
	close(c.stopChan)
}
