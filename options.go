package tempuscache

import (
	"time"
)

/*
Option represents a functional configuration modifier for Cache.

================================================================================
DESIGN PATTERN: FUNCTIONAL OPTIONS
================================================================================

TempusCache uses the Functional Options Pattern — an idiomatic Go
approach for flexible and future-proof configuration.

Instead of passing multiple constructor parameters, New() accepts
a variadic list of Option functions:

    cache := New(
        WithCleanupInterval(10 * time.Second),
    )

Each Option is a function that mutates the Cache instance
during initialization.

================================================================================
WHY THIS PATTERN?
================================================================================

1. API STABILITY
   - Adding new configuration fields does not change the New() signature.
   - Prevents breaking changes.

2. READABILITY
   - Configuration is explicit and self-documenting.
   - Avoids confusing positional constructor arguments.

3. EXTENSIBILITY
   - New features (e.g., capacity limits, eviction strategies,
     metrics hooks, logging integrations) can be added seamlessly.

4. COMPOSABILITY
   - Multiple options can be combined in a clear and modular way.

================================================================================
ENGINEERING PHILOSOPHY
================================================================================

The constructor remains minimal and stable,
while configuration logic remains modular and isolated.

This pattern is widely used in production Go libraries
for long-term maintainability.
*/

type Option func(*Cache)

/*
WithCleanupInterval configures the active expiration frequency.

================================================================================
PARAMETER
================================================================================

d (time.Duration):
    Interval at which the background janitor scans
    and removes expired entries.

================================================================================
BEHAVIOR
================================================================================

If d > 0:
    - A background janitor goroutine is started.
    - Expired entries are periodically removed.
    - Enables active expiration strategy.

If d <= 0:
    - The janitor is disabled.
    - The cache relies solely on lazy expiration during Get().

================================================================================
PERFORMANCE TRADE-OFFS
================================================================================

Short intervals:
    - Faster cleanup of expired entries
    - Increased CPU usage due to frequent scans

Long intervals:
    - Lower CPU overhead
    - Expired items may occupy memory longer

================================================================================
SYSTEM DESIGN CONSIDERATION
================================================================================

Choosing an appropriate cleanup interval depends on:

- Cache size
- TTL distribution
- Memory sensitivity
- Throughput requirements

This option provides operational control over
the balance between performance and memory efficiency.
*/

func WithCleanupInterval(d time.Duration) Option {
	return func(c *Cache) {
		c.interval = d
	}
}

func WithMaxEntries(n int) Option {
	return func(c *Cache) {
		c.maxEntries = n
	}
}
