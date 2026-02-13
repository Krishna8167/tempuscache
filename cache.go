package tempuscache

import (
	"container/list"
	"sync"
	"time"
)

/*
Cache implements a thread-safe, in-memory key-value store with:

- Per-key TTL (Time-To-Live)
- LRU (Least Recently Used) eviction
- Active + Lazy expiration
- Configurable capacity limits
- Runtime statistics tracking

================================================================================
ARCHITECTURAL OVERVIEW
================================================================================

TempusCache combines two core data structures:

1. Hash Map (map[string]*list.Element)
   - Provides O(1) key lookup.
   - Maps keys to their corresponding LRU list elements.

2. Doubly Linked List (*list.List)
   - Maintains LRU ordering.
   - Most recently used items are moved to the front.
   - Oldest items remain at the back for eviction.

================================================================================
CONCURRENCY MODEL
================================================================================

- sync.RWMutex protects all shared state.
- Write operations use Lock().
- Read-only operations use RLock().
- Internal modifications (LRU movement, expiration cleanup) are performed
  under exclusive locking to prevent race conditions.

This guarantees safe usage in highly concurrent, multi-goroutine environments.

================================================================================
EXPIRATION STRATEGY
================================================================================

TempusCache uses a dual expiration model:

1. Lazy Expiration
   - Expired keys are removed during Get() operations.
   - Ensures expired data is never returned to callers.

2. Active Expiration
   - A background janitor periodically scans and removes expired entries.
   - Prevents memory buildup from stale keys.

================================================================================
STRUCTURE FIELDS
================================================================================

data       -> Primary storage map (key → *list.Element)
lru        -> Doubly linked list maintaining LRU ordering
mu         -> Read-write mutex for concurrency control
maxEntries -> Maximum allowed entries before LRU eviction
interval   -> Background cleanup interval
stopChan   -> Graceful shutdown signal for janitor goroutine
stats      -> Cache performance metrics (hits/misses)

The design prioritizes:
- Predictable performance
- Deterministic eviction behavior
- Minimal memory overhead
*/

type Cache struct {
	data       map[string]*list.Element
	lru        *list.List //where each element stores an Item.
	mu         sync.RWMutex
	maxEntries int
	interval   time.Duration
	stopChan   chan struct{}
	stats      Stats
	// graceful shutdown pattern, and struct{} uses zero memory.
}

/*
New initializes and returns a configured Cache instance.

CONFIGURATION MODEL:
Uses the functional options pattern to allow extensible configuration
without modifying the constructor signature.

INITIALIZATION STEPS:
1. Allocate internal map.
2. Initialize LRU list.
3. Create stop channel for graceful shutdown.
4. Apply user-provided options.
5. Start background janitor (if cleanup interval is set).

If no cleanup interval is configured, the janitor will not run.

This pattern ensures forward compatibility and API stability.
*/

func New(opts ...Option) *Cache {
	c := &Cache{
		data:     make(map[string]*list.Element),
		lru:      list.New(),
		stopChan: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	c.startJanitor()

	return c
}

/*
Set inserts or updates a key in the cache.

PARAMETERS:
- key   : Unique identifier
- value : Arbitrary data (stored as interface{})
- ttl   : Time-To-Live duration

BEHAVIOR:

1. If key already exists:
   - Update its value.
   - Recalculate expiration (if ttl > 0).
   - Move item to front of LRU list.

2. If key does not exist:
   - If maxEntries limit is reached → evict oldest entry (LRU policy).
   - Create new Item with optional expiration timestamp.
   - Insert at front of LRU list.
   - Store reference in map.

TTL IMPLEMENTATION:
Expiration time is stored as UnixNano (int64) for:
- Fast numeric comparison
- Reduced object overhead

TIME COMPLEXITY:
O(1) average case

This operation is fully protected by exclusive locking to ensure consistency.
*/

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, found := c.data[key]; found {
		item := elem.Value.(*Item)
		item.value = value
		if ttl > 0 {
			item.expiration = time.Now().Add(ttl).UnixNano()
		}
		c.lru.MoveToFront(elem)
		return
	}

	if c.maxEntries > 0 && c.lru.Len() >= c.maxEntries {
		c.evictOldest()
	}

	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	item := &Item{
		key:        key,
		value:      value,
		expiration: exp,
	}

	elem := c.lru.PushFront(item)
	c.data[key] = elem
}

/*
Get retrieves a value from the cache.

RETURNS:
- (interface{}, true)  -> If key exists and is not expired
- (nil, false)         -> If key does not exist or is expired

EXECUTION FLOW:

1. Lookup key in O(1) using map.
2. If not found:
   - Increment Miss counter.
   - Return immediately.

3. If found:
   - Check expiration (lazy expiration).
   - If expired:
       - Remove element from LRU + map.
       - Increment Miss counter.
       - Return false.

4. If valid:
   - Move item to front of LRU (mark as recently used).
   - Increment Hit counter.
   - Return value.

LRU UPDATE:
Every successful access updates recency ordering,
ensuring accurate eviction decisions.

TIME COMPLEXITY:
O(1) average case

This method acquires exclusive Lock() because it may:
- Modify LRU ordering
- Remove expired entries
- Update statistics
*/

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, found := c.data[key]
	if !found {
		c.stats.Misses++
		return nil, false
	}

	item := elem.Value.(*Item)

	if item.Expired() {
		c.removeElement(elem)
		c.stats.Misses++
		return nil, false
	}

	c.lru.MoveToFront(elem)
	c.stats.Hits++
	return item.value, true
}

/*
Delete removes a key from the cache.

BEHAVIOR:
- If key exists → remove from map.
- If key does not exist → operation is safely ignored.

This operation does not panic on missing keys.

CONCURRENCY:
Uses exclusive locking to ensure safe mutation of shared state.

TIME COMPLEXITY:
O(1) average case
*/

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
}

func (c *Cache) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

/*
deleteExpired performs active expiration by scanning the LRU list
and removing expired entries.

This method is invoked by the background janitor at configured intervals.

ALGORITHM:
- Iterate from the back (oldest entries).
- Check expiration status.
- Remove expired elements using removeElement().

TIME COMPLEXITY:
O(n) — full scan of entries.

CONCURRENCY:
Acquires exclusive Lock() since it mutates internal structures.

DESIGN RATIONALE:
Active expiration prevents memory accumulation from expired keys
that are not accessed frequently enough to trigger lazy deletion.
*/

func (c *Cache) deleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for elem := c.lru.Back(); elem != nil; {
		prev := elem.Prev()
		item := elem.Value.(*Item)
		if item.Expired() {
			c.removeElement(elem)
		}
		elem = prev
	}
}
