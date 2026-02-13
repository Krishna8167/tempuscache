package tempuscache

import "container/list"

/*
evictOldest removes the least recently used (LRU) entry
from the cache when capacity constraints are exceeded.

================================================================================
EVICTION POLICY
================================================================================

TempusCache uses a strict LRU (Least Recently Used) policy:

- Most recently accessed entries are moved to the front.
- Least recently used entries remain at the back.
- When maxEntries is reached, the oldest entry is evicted.

This guarantees predictable memory bounds and deterministic
eviction behavior.

================================================================================
ALGORITHM
================================================================================

1. Retrieve the last element from the LRU list.
2. If it exists:
   - Remove it from both:
       a) The linked list
       b) The hash map
   - Increment eviction statistics counter.

TIME COMPLEXITY:
O(1)

The use of a doubly linked list ensures constant-time removal.
*/

func (c *Cache) evictOldest() {
	elem := c.lru.Back()
	if elem != nil {
		c.removeElement(elem)
		c.stats.Evictions++
	}
}

/*
removeElement removes a given list element from both
the LRU list and the primary storage map.

================================================================================
RESPONSIBILITY
================================================================================

This is an internal helper method used by:

- LRU eviction
- Lazy expiration
- Active expiration (janitor)
- Explicit delete (if implemented using it)

================================================================================
CONSISTENCY GUARANTEE
================================================================================

To maintain structural integrity:

- The element is first removed from the linked list.
- The corresponding key is then deleted from the map.

This ensures there are no dangling references between
the list and the hash map.

TIME COMPLEXITY:
O(1)

NOTE:
This function assumes the caller already holds
the appropriate lock (Lock or RLock upgrade scenario).
It does NOT perform its own synchronization.
*/

func (c *Cache) removeElement(e *list.Element) {
	c.lru.Remove(e)
	item := e.Value.(*Item)
	delete(c.data, item.key)
}
