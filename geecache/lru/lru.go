package lru

import "container/list"

// lru 缓存淘汰策略
// Cache is a LRU cache. It is not safe for concurrent access.
type Cache struct {
	maxBytes int64 //允许使用的最大内存
	nbyBytes int64 //当前已使用的内存
	ll       *list.List
	cache    map[string]*list.Element //键是字符串，值是双向链表中对应节点的指针
	// optional and executed when an entry is purged.
	OnEvicted func(key string, value Value)
}

type entry struct {
	key   string
	value Value
}

// Value use Len to count how many bytes it takes
type Value interface {
	Len() int
}

// New is the Constructor of Cache
func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
		//将某个函数传递给 New 函数，并赋给 OnEvicted 字段，你可以在缓存中的条目被移除时执行自定义的操作，
		//比如释放资源、记录日志等，可以让 Cache 结构体更加通用和可扩展。
	}
}

// Get look ups a key's value
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

// RemoveOldest removes the oldest item
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbyBytes -= int64(kv.value.Len()) + int64(len(kv.key))
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Add adds a value to the cache.
func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbyBytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		ele := c.ll.PushFront(&entry{key, value})
		c.cache[key] = ele
		c.nbyBytes += int64(len(key)) + int64(value.Len())
	}
	//保证内存不超过最大值 ps:maxBytes为0表示无限制
	for c.maxBytes != 0 && c.maxBytes < c.nbyBytes {
		c.RemoveOldest()
	}
}

// Len is the number of cache entries
func (c *Cache) Len() int {
	return c.ll.Len()
}
