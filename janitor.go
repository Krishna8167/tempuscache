package tempuscache

import "time"

/*
startJanitor launches a background cleanup goroutine responsible
for periodically removing expired items from the cache.

PURPOSE

The janitor implements the active expiration strategy.
While lazy expiration ensures expired items are never returned,
the janitor ensures memory is reclaimed even if expired keys
are never accessed again.

BEHAVIOR

1. If cleanup interval <= 0:
   - The janitor is disabled.
   - The cache relies entirely on lazy expiration.

2. If cleanup interval > 0:
   - A time.Ticker is created with the specified interval.
   - A goroutine is launched.
   - On every tick, deleteExpired() is executed.

CONCURRENCY MODEL

- deleteExpired() acquires an exclusive Lock() since it modifies the map.
- The goroutine runs independently of caller threads.
- stopChan is used to gracefully terminate the goroutine.

RESOURCE MANAGEMENT

The ticker is stopped before the goroutine exits to prevent
resource leakage.

TIME COMPLEXITY

Each cleanup cycle performs a full scan of the map (O(n)).
This is acceptable for moderate sizes but may require optimization
(e.g., heap scheduling or sharding) for very large datasets.
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
Stop gracefully terminates the background janitor.

BEHAVIOR

- Closing stopChan signals the janitor goroutine to exit.
- The goroutine stops the ticker before returning.
- This prevents memory leaks and dangling timers.

IMPORTANT NOTE

Stop should ideally be called once per cache lifecycle.
Calling Stop multiple times will panic because a closed
channel cannot be closed again.

This method allows controlled shutdown in long-running
applications such as servers or background workers.
*/

func (c *Cache) Stop() {
	close(c.stopChan)
}
