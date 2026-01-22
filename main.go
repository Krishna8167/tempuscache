package main

import (
	"fmt"
	"sync"
	"time"
)

// Item is representing a single cache entry.
type Item struct {
	Value      interface{}
	ExpiryTime time.Time
}

/*
	Value Interface{}
	- stores the actual cached value.
	- can be of any type (string, struct, int, etc.)

	ExpiryTime time.Time
	- the exact timewhen this item should expire.
	- Can be any type (string, int, struct, etc).

		Also we can use generics (for go 1.18+):

			type Item[T any] struct {
				Value      	T
				ExpiryTime 	time.Time
			}


*/

type Cache struct {
	data map[string]Item
	mu   sync.RWMutex
	ttl  time.Duration
	stop chan struct{}
}

/*
	Classic in-memory TTL cache pattern in Go.

	This structure represents a thread-safe, time-based cache with
	automatic expiration support.

	------------------------------------------------------------
	1. data map[string]Item
	------------------------------------------------------------
	- Key:   string — cache key
	- Value: Item   — stored value + expiration metadata

	The built-in map provides O(1) average-time lookups, making it
	ideal for in-memory caching workloads.

	------------------------------------------------------------
	2. mu sync.RWMutex
	------------------------------------------------------------
	Used to make the cache safe for concurrent access.

	Without synchronization, concurrent reads and writes to a map
	will cause runtime panics.

	RWMutex is chosen over Mutex because it better matches cache
	access patterns:

	- Multiple readers can access the cache simultaneously (Get)
	- Writers (Set, Delete, eviction) acquire exclusive access

	This significantly improves performance under read-heavy loads.

	------------------------------------------------------------
	3. ttl time.Duration
	------------------------------------------------------------
	Defines the default Time-To-Live applied to all cache entries.

	Design rationale:
	- A single expiration policy keeps the API simple
	- Callers don’t need to specify TTL on every Set
	- Centralizes expiration behavior

	------------------------------------------------------------
	4. stop chan struct{}
	------------------------------------------------------------
	Used to signal background goroutines (e.g. eviction workers)
	to shut down gracefully.

	Closing this channel:
	- Broadcasts a stop signal to all listeners
	- Prevents goroutine leaks
	- Enables clean shutdowns in tests and production

	This is a standard Go pattern for lifecycle management of
	long-running background processes.

	------------------------------------------------------------
	Modern generic version (Go 1.18+)
	------------------------------------------------------------

	type Cache[T any] struct {
		data map[string]Item[T]
		mu   sync.RWMutex
		ttl  time.Duration
		stop chan struct{}
	}

*/

//Core Ops - API

func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = Item{
		Value:      value,
		ExpiryTime: time.Now().Add(c.ttl),
	}
}

/*
		func (c *Cache) Set(key string, value interface{})
			{}

	c.mu.Lock() - map write safely
	- Acquires a write (exclusive) lock
	- Blocks all other readers and writers

	defer c.mu.Unlock()   - panic-safe
	- Ensures the lock is released even if the function exits early

	c.data[key] = ...
	- Modifies shared state → must be exclusive
	- TTL is calculated once only.

*/

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, found := c.data[key]
	c.mu.RUnlock()

	if !found {
		return nil, false
	}

	if time.Now().After(item.ExpiryTime) {
		c.mu.Lock()
		// re-check after upgrading to write lock
		item, found = c.data[key]
		if found && time.Now().After(item.ExpiryTime) {
			delete(c.data, key)
		}
		c.mu.Unlock()
		return nil, false
	}

	return item.Value, true
}

/*

		Get retrieves a value associated with a key.
		It returns the value and a boolean indicating whether the key exists and the item has not expired.

	Timeline intuition:
									Past ---------------- ExpiryTime -------- Now
																		↑
																		Now.After(ExpiryTime) == true

	Locking Timeline:
						RLock → Read item
						Unlock
						Check expiry
						Lock → Re-check item
						Delete safely
						Unlock


	Get returns (value, bool):
		value → the actual data
		bool → whether the value is valid
		This avoids ambiguity when the value could be empty or zero.
*/

func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		data: make(map[string]Item),
		ttl:  ttl,
		stop: make(chan struct{}),
	}
}

/*
	NewCache constructs and initializes a new in-memory TTL cache.

	Parameters:
	------------------------------------------------------------
	ttl time.Duration
		Defines the default Time-To-Live applied to all cache entries.
		Each item inserted into the cache will automatically expire
		after this duration.

		Typical values:
			5 * time.Minute
			30 * time.Second

	Internal Initialization:
	------------------------------------------------------------
	make(map[string]Item)
		Allocates the internal storage map.
		This ensures:
			- O(1) average-time lookups
			- No nil-map panics
			- Immediate readiness for inserts

	make(chan struct{})
		Initializes the shutdown signal channel.
		This channel is used to:
			- Gracefully stop background goroutines (e.g. eviction worker)
			- Prevent goroutine leaks
			- Support clean shutdowns in tests and production

	Return Value:
	------------------------------------------------------------
	*Cache
		Returns a pointer to the initialized cache instance.

		Using a pointer:
			- Avoids copying internal state (maps, mutexes)
			- Enables shared access across goroutines
			- Preserves correctness and performance

*/

/*

	Background Eviction Workflow.

	- Expired Keys are removed only on Get
	- That's lazy eviction

	ADDING : - Active Eviction ..

	Eviction strategy (Choosen):

	- time.Ticker
	- Fixed interval(eg: every 2 seconds)

	to: predictable, Simple , production-friendly

*/

func (c *Cache) startEviction(interval time.Duration) {

	ticker := time.NewTicker(interval)

	go func() {
		// The eviction loop runs in a dedicated background goroutine.
		// This prevents eviction work from blocking the main application
		// and allows cache operations (Get / Set) to continue concurrently.
		//
		// The goroutine wakes up periodically based on the provided
		// interval and removes expired entries from the cache.
		//
		// Lifecycle:
		// - Starts when startEviction is called
		// - Runs until the stop channel is closed
		// - Exits cleanly on shutdown to avoid goroutine leaks
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Periodic eviction pass
				now := time.Now()

				// Acquire exclusive lock since the map is being modified
				c.mu.Lock()
				for key, item := range c.data {
					if now.After(item.ExpiryTime) {
						delete(c.data, key)
					}
				}
				c.mu.Unlock()

			case <-c.stop:
				// Shutdown signal received
				// Exit the goroutine gracefully
				return
			}
		}
	}()
}

/*
	startEviction starts a background eviction worker that periodically
	removes expired entries from the cache.

	A time.Ticker triggers at the specified interval. On each tick, the
	worker scans the cache and deletes items whose ExpiryTime has passed.

	Concurrency & Safety:
	- The eviction logic runs in its own goroutine and never blocks
	  the main application.
	- An exclusive lock is acquired while modifying the cache to
	  ensure thread-safe access to the underlying map.

	Lifecycle Management:
	- The worker starts when startEviction is called.
	- It listens on the stop channel for shutdown signals.
	- When the stop channel is closed, the goroutine exits cleanly,
	  preventing goroutine leaks.

	IN BRIEF:
	Provides automatic, thread-safe, and gracefully stoppable cleanup
	of expired cache entries at regular intervals.
*/

/*
	startEviction starts a background eviction worker that periodically
	removes expired entries from the cache.

	A time.Ticker triggers at the specified interval. On each tick, the
	worker scans the cache and deletes items whose ExpiryTime has passed.

	Concurrency & Safety:
	- The eviction logic runs in its own goroutine and never blocks
	  the main application.
	- An exclusive lock is acquired while modifying the cache to
	  ensure thread-safe access to the underlying map.

	Lifecycle Management:
	- The worker starts when startEviction is called.
	- It listens on the stop channel for shutdown signals.
	- When the stop channel is closed, the goroutine exits cleanly,
	  preventing goroutine leaks.

	IN BRIEF:
	Provides automatic, thread-safe, and gracefully stoppable cleanup
	of expired cache entries at regular intervals.
*/

func (c *Cache) Stop() {
	close(c.stop)
}

/*
	Stop gracefully shuts down all background workers associated
	with the cache.

	Behavior:
	- Closes the stop channel to broadcast a shutdown signal.
	- Unblocks any goroutines listening for the stop signal
	  (e.g. the eviction worker).
	- Allows in-flight work to exit cleanly.

	Guarantees:
	- Prevents goroutine leaks.
	- Safe to call once during cache teardown.
	- Intended to be used during application shutdown,
	  test cleanup, or controlled lifecycle management.

	Note:
	Calling Stop more than once will cause a panic due to
	closing an already-closed channel. It should be invoked
	exactly once by the cache owner.
*/

func main() {
	cache := NewCache(5 * time.Second)
	cache.startEviction(2 * time.Second)

	cache.Set("name", "krishna")

	time.Sleep(6 * time.Second)

	if _, ok := cache.Get("name"); !ok {
		fmt.Println("Expired (Cleaned by eviction workflow)")
	}

	cache.Stop()

}
