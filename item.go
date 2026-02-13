package tempuscache

import (
	"time"
)

/*
Item represents a single cache entry stored inside the Cache map.

DESIGN PURPOSE

Each cache key maps to an Item instead of directly storing the value.
This allows the cache to associate metadata (such as expiration time)
with each stored value.

STRUCTURE

value      -> The actual stored data (generic via interface{}).
expiration -> Unix timestamp in nanoseconds representing expiry time.

EXPIRATION STRATEGY

- If expiration == 0:
  The item does not expire (infinite lifetime).

- If expiration > 0:
  The item is considered expired once:
      time.Now().UnixNano() > expiration

WHY int64 (UnixNano)?

Using int64 instead of time.Time:
- Faster numeric comparison
- Lower memory overhead
- Avoids extra struct allocation
- Cache-friendly representation

This structure keeps the cache lightweight and efficient.
*/

type Item struct {
	key        string
	value      interface{} //Atomic unit of storage in cache.
	expiration int64       //stored UnixNano Meaning: Number of nanoseconds since January 1, 1970 UTC (Unix epoch).
}

/*
	int64 can safely represent timestamps for ~292 years

	−9,223,372,036,854,775,808
	to
	+9,223,372,036,854,775,807

*/

/*
Expired determines whether the item has exceeded its TTL.

RETURNS:
true  -> If the item is expired
false -> If the item is still valid

BEHAVIOR:

1. If expiration == 0:
   The item has no TTL and never expires.

2. Otherwise:
   Compares current time (UnixNano) with stored expiration.

This function supports both:
- Lazy expiration (checked during Get())
- Active expiration (checked during background cleanup)

VALUE RECEIVER JUSTIFICATION:

The method uses a value receiver because:
- Item is small (two fields)
- No mutation occurs
- Copy cost is minimal
- Keeps implementation simple and idiomatic
*/

func (i *Item) Expired() bool {
	if i.expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > i.expiration
}
