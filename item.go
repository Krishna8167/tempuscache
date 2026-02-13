package tempuscache

import (
	"time"
)

/*
Item represents the atomic storage unit inside TempusCache.

================================================================================
ROLE IN ARCHITECTURE
================================================================================

Each cache key maps to an *Item rather than directly storing a value.
This abstraction enables the cache to attach metadata (such as TTL)
alongside the stored data.

By separating value storage from expiration metadata,
the design achieves:

- Clean separation of concerns
- Efficient expiration checks
- Flexible future extensibility (e.g., size tracking, access counters)

================================================================================
STRUCTURE FIELDS
================================================================================

key        -> Stored key reference (used during eviction removal)
value      -> Actual user data (generic via interface{})
expiration -> Expiration timestamp in Unix nanoseconds (int64)

================================================================================
EXPIRATION MODEL
================================================================================

Expiration is represented as a UnixNano timestamp.

- expiration == 0
    → The item never expires (infinite lifetime).

- expiration > 0
    → The item is considered expired when:
        time.Now().UnixNano() > expiration

================================================================================
WHY int64 (UnixNano) INSTEAD OF time.Time?
================================================================================

Using int64 provides:

- Faster numeric comparisons
- Lower memory footprint
- Avoidance of additional struct overhead
- Cache-friendly representation
- No method dispatch required for comparison

This keeps the cache lean and optimized for high-throughput workloads.

================================================================================
DESIGN PHILOSOPHY
================================================================================

Item is intentionally minimal:

- No internal locking
- No complex state
- Pure data container

Concurrency control is handled at the Cache level,
ensuring single-responsibility separation.
*/

type Item struct {
	key        string
	value      interface{} //Atomic unit of storage in cache.
	expiration int64       //stored UnixNano Meaning: Number of nanoseconds since January 1, 1970 UTC (Unix epoch).
}

/*
int64 timestamp range capacity:

An int64 can represent approximately ±292 years
when storing nanosecond-resolution Unix timestamps.

Range:
    -9,223,372,036,854,775,808
    to
    +9,223,372,036,854,775,807

This makes it more than sufficient for
practical TTL use cases.
*/

/*
Expired determines whether the item has exceeded its TTL.

================================================================================
RETURN VALUE
================================================================================

true  -> Item has expired and should be removed.
false -> Item is still valid.

================================================================================
LOGIC FLOW
================================================================================

1. If expiration == 0:
   - The item has no TTL.
   - It is considered permanently valid.

2. If expiration > 0:
   - Compare current UnixNano timestamp with stored expiration.
   - If current time exceeds expiration → expired.

================================================================================
USAGE CONTEXT
================================================================================

This method is used in:

- Lazy expiration (checked during Get())
- Active expiration (checked by janitor process)

================================================================================
RECEIVER DESIGN DECISION
================================================================================

Pointer receiver (*Item) is used because:

- The struct is stored inside a linked list element.
- Avoids unnecessary copying.
- Maintains consistent reference semantics.

The method does not mutate state,
but pointer usage keeps the implementation efficient
within LRU-linked structures.
*/

func (i *Item) Expired() bool {
	if i.expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > i.expiration
}
