package tempuscache

import (
	"time"
)

/*
Option defines a functional configuration modifier for Cache.

DESIGN PATTERN

This file implements the Functional Options Pattern, a common
idiomatic Go design used for flexible and extensible configuration.

Instead of passing multiple parameters to the constructor,
New() accepts a variadic list of Option functions:

    cache := New(
        WithCleanupInterval(10 * time.Second),
    )

Each Option modifies the Cache instance before it becomes active.

BENEFITS

1. API Stability:
   Adding new configuration options does not change the New() signature.

2. Readability:
   Configuration is self-documenting and explicit.

3. Extensibility:
   Future features (e.g., max capacity, eviction policy, logging)
   can be added without breaking existing code.

Each Option is simply a function that mutates the Cache struct.
*/

type Option func(*Cache)

func WithCleanupInterval(d time.Duration) Option {
	return func(c *Cache) {
		c.interval = d
	}
}
