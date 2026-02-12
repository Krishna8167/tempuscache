package tempuscache

import (
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
	data     map[string]Item
	mu       sync.RWMutex
	interval time.Duration
	stopChan chan struct{}
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
		data:     make(map[string]Item),
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
	var expiration int64
	/*
		 	faster comparison than time.Time
			Avoid time object overhead
			UnixNano is monotonic numeric.
	*/

	if ttl > 0 {
		expiration = time.Now().Add(ttl).UnixNano()
	}

	c.mu.Lock()
	c.data[key] = Item{
		value:      value,
		expiration: expiration,
	}
	c.mu.Unlock()
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
	// only reading So , RLock()
	c.mu.RLock()
	item, found := c.data[key]
	c.mu.RUnlock()

	if !found {
		return nil, false
	}

	if item.Expired() {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return nil, false
	}

	/*
	   Delete removes a key from the cache.

	   Write access is protected using Lock().
	   If the key does not exist, delete is safely ignored.
	*/

	return item.value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
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

	for key, item := range c.data {
		if item.Expired() {
			delete(c.data, key)
		}
	}
}
