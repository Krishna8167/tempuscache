package tempuscache

import (
	"container/list"
	"sync"
	"time"
)

/*
Cache represents a concurrent in-memory key-value store with optional TTL support.

DESIGN PRINCIPLES

1. Thread Safety:
   The internal map is protected using sync.RWMutex to allow safe
   concurrent reads and writes.

2. TTL Support:
   Each key can have an independent expiration time. Expiration
   metadata is stored inside the Item struct.

3. Dual Expiration Strategy:
   - Lazy expiration: Expired items are checked and removed during Get().
   - Active expiration: A background janitor periodically scans and removes expired keys.

4. Extensibility:
   The cache is configured using the functional options pattern,
   allowing new configuration parameters without breaking the API.

STRUCTURE

data     -> Primary storage map (key -> Item)
mu       -> Read-write mutex for concurrency control
interval -> Cleanup interval for background janitor
stopChan -> Signal channel to gracefully stop background cleanup
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
New creates and initializes a new Cache instance.

It applies all provided functional options before starting
the background janitor.

If no cleanup interval is configured, the janitor will not start.
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
- key   : unique identifier
- value : any data type (stored as interface{})
- ttl   : time-to-live duration

BEHAVIOR:
- If ttl > 0, expiration timestamp is calculated.
- If ttl <= 0, the item does not expire.
- Write access is protected using Lock().

Expiration is stored as UnixNano for fast numeric comparison.
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
- value (interface{})
- boolean indicating whether the key exists and is valid

BEHAVIOR:
1. Uses RLock() for read access.
2. If key not found -> returns (nil, false).
3. If found but expired:
   - Removes the key using Lock().
   - Returns (nil, false).
4. Otherwise returns stored value.

This implements lazy expiration to ensure expired values
are never returned to callers.
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

   Write access is protected using Lock().
   If the key does not exist, delete is safely ignored.
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
deleteExpired scans the entire map and removes expired entries.

This function is called by the background janitor at configured intervals.

TIME COMPLEXITY:
O(n) — full scan of map

CONCURRENCY:
Acquires exclusive Lock() since it modifies the map.

This is part of the active expiration strategy.
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
