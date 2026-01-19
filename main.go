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
}

/*
	Classic In-memory TTL cache pattern in go.

	1. data map[string]Item :
		- Key:		String - cache key
		- Value:	Item   - Stored value + expiration metadata

		Note:	map : O(1) average lookup, Perfect for in-memory caches

	2. mu sync.RWMutex:
		required to make it thread-safe.
		exclusive for eradicating runtime panics: concurrent map read and map write

		We use here, RWMutex instead of the Mutex in general :
		- Sync.RWMutex allows ,
			- many readers at the same time.
			- only one writer at a time.

		this matches the cache access patterns:
			. Reads (Get)			 - are frequent
			. Writes (Set, Delete )  - less

	3. ttl time.Duration:
		this defines the default time-to-live for cache enteries.

		Reason:
		- all items share the same expiration policy
		- simple API structure(Set(key, value))
		- Avoids repeating TTL everywhere

	All this will work together when transactions starts.

	Modern generic version for go 1.18+ :

	type Cache[T any] struct {
		data map[string]Item[T]
		mu   sync.RWMutex
		ttl  time.Duration
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
	}
}

/*
	NewCache is a constructor function that creates and initializes a new cache instance with a default time-to-live (TTL) for cached items.
	ttl time.Duration

	Specifies how long each item in the cache remains valid
		Common examples: 5 * time.Minute, 30 * time.Second

	make(map[string]Item)
		Initializes the internal storage map
		Prevents nil map runtime panics
		Allows immediate insertion of items

	Returned value: *Cache
		Returns a pointer to the newly created cache
		Ensures:
			Efficient memory usage
			Shared state when passed around

*/

func main() {
	cache := NewCache(5 * time.Second)

	cache.Set("name", "krishna")

	if val, ok := cache.Get("name"); ok {
		fmt.Println("Found:", val)
	}

	time.Sleep(6 * time.Second)

	if _, ok := cache.Get("name"); !ok {
		fmt.Println("Expired")
	}
}
