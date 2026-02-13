package tempuscache

/*
Stats represents runtime performance metrics of the cache.

================================================================================
PURPOSE
================================================================================

This structure tracks key operational indicators:

- Hits      → Successful retrievals (valid key found)
- Misses    → Failed lookups (missing or expired key)
- Evictions → Entries removed due to LRU capacity constraints

These metrics provide visibility into cache effectiveness
and operational behavior.

================================================================================
OBSERVABILITY VALUE
================================================================================

Tracking cache statistics enables:

- Cache hit ratio analysis
- Performance tuning
- Capacity planning
- Debugging production behavior
- Evaluating TTL configuration effectiveness

For example:

    hit_ratio = Hits / (Hits + Misses)

================================================================================
CONCURRENCY MODEL
================================================================================

Stats fields are modified under Cache-level locking.
The Stats() method returns a snapshot under read lock,
ensuring consistent reads without race conditions.

================================================================================
DESIGN SIMPLICITY
================================================================================

The struct is intentionally minimal:

- No internal locking
- No atomic counters
- Synchronization handled at Cache level

This keeps the data structure lightweight
and avoids unnecessary complexity.
*/

type Stats struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
}
