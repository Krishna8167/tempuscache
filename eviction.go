package tempuscache

import "container/list"

func (c *Cache) evictOldest() {
	elem := c.lru.Back()
	if elem != nil {
		c.removeElement(elem)
		c.stats.Evictions++
	}
}

func (c *Cache) removeElement(e *list.Element) {
	c.lru.Remove(e)
	item := e.Value.(*Item)
	delete(c.data, item.key)
}
