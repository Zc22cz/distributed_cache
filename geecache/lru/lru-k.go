package lru

import "container/list"

// lru 缓存淘汰策略
// Cache is a LRU cache. It is not safe for concurrent access.
type Cache struct {
	maxBytes int64 //允许使用的最大内存
	useBytes int64 //当前已使用的内存
	ll       *list.List
	mp       map[string]*list.Element //键是字符串，值是双向链表中对应节点的指针
	// optional and executed when an entry is purged.
	OnEvicted    func(key string, value Value)
	historyCache HistoryCache // 历史队列，只有访问次数达到k次后才会加入到缓存中
}

type HistoryCache struct {
	k        int // k次访问后加入缓存
	maxBytes int64
	useBytes int64
	ll       *list.List // 历史队列是以FIFO为淘汰策略
	mp       map[string]*list.Element
	cnt      map[string]int // 对每个节点访问次数的统计
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
func New(maxBytes int64, onEvicted func(string, Value), k int) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		mp:        make(map[string]*list.Element),
		OnEvicted: onEvicted,
		//将某个函数传递给 New 函数，并赋给 OnEvicted 字段，你可以在缓存中的条目被移除时执行自定义的操作，
		//比如释放资源、记录日志等，可以让 Cache 结构体更加通用和可扩展。

		historyCache: HistoryCache{
			k:        k, // 可以改为New()传入参，一般用2次命中率和适应性综合考虑最优
			maxBytes: maxBytes,
			ll:       list.New(),
			mp:       make(map[string]*list.Element),
			cnt:      make(map[string]int),
		},
	}
}

// Get look ups a key's value
func (c *Cache) Get(key string) (value Value, ok bool) {
	if _, ok = c.mp[key]; ok {
		// 缓存命中了就挪到前面
		ele := c.mp[key]
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	} else {
		// 缓存未命中，去历史队列查看，如果访问次数达到k次需要加入到缓存中
		if _, ok = c.historyCache.mp[key]; ok {
			// 有就根据访问次数看是否要加到缓存中,没达到次数也要将该节点挪到最后,即最晚被FIFO淘汰
			c.historyCache.cnt[key]++
			ele := c.historyCache.mp[key]
			kv := ele.Value.(*entry)

			if c.historyCache.cnt[key] >= c.historyCache.k {
				c.AddToCache(key, value)
				// 加入缓存后，将该节点从历史队列中删除
				c.historyCache.ll.Remove(ele)
				c.historyCache.useBytes -= int64(kv.value.Len()) + int64(len(kv.key))
				delete(c.historyCache.mp, kv.key)
				delete(c.historyCache.cnt, kv.key)
			} else {
				c.historyCache.ll.MoveToBack(ele)
			}

			return kv.value, true
		} else {
			// 历史队列也没有就直接返回
			return
		}
	}

	return
}

// Add adds a value to the cache.
func (c *Cache) Add(key string, value Value) {
	if _, ok := c.mp[key]; ok {
		// 缓存命中了就挪到前面，更新value
		ele := c.mp[key]
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.useBytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		// 缓存未命中，则去历史队列查看是否存在
		if _, ok = c.historyCache.mp[key]; !ok {
			// 没有就新增
			ele := c.historyCache.ll.PushBack(&entry{key, value})
			c.historyCache.cnt[key]++
			c.historyCache.mp[key] = ele
			c.historyCache.useBytes += int64(len(key)) + int64(value.Len())

			// 判断历史队列内存是否用完，历史队列的淘汰策略为FIFO
			if c.historyCache.maxBytes != 0 && c.historyCache.maxBytes < c.historyCache.useBytes {
				c.RemoveHistoryCacheOldest()
			}
		} else {
			// 有就更新value，并移到队尾
			c.historyCache.cnt[key]++
			ele := c.historyCache.mp[key]
			c.historyCache.ll.MoveToBack(ele)
			kv := ele.Value.(*entry)
			c.historyCache.useBytes += int64(value.Len()) - int64(kv.value.Len())
			kv.value = value
		}

		// 判断是否达到加入缓存标准
		if c.historyCache.cnt[key] >= c.historyCache.k {
			c.AddToCache(key, value)
			ele := c.historyCache.mp[key]
			kv := ele.Value.(*entry)
			// 加入缓存后，将该节点从历史队列中删除
			c.historyCache.ll.Remove(ele)
			c.historyCache.useBytes -= int64(kv.value.Len()) + int64(len(kv.key))
			delete(c.historyCache.mp, kv.key)
			delete(c.historyCache.cnt, kv.key)
		}
	}
}

func (c *Cache) AddToCache(key string, value Value) {
	ele := c.ll.PushFront(&entry{key, value})
	c.mp[key] = ele
	c.useBytes += int64(len(key)) + int64(value.Len())

	//保证内存不超过最大值 ps:maxBytes为0表示无限制
	for c.maxBytes != 0 && c.maxBytes < c.useBytes {
		c.RemoveCacheOldest()
	}
}

// RemoveCacheOldest removes the oldest item
func (c *Cache) RemoveCacheOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.mp, kv.key)
		c.useBytes -= int64(kv.value.Len()) + int64(len(kv.key))
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) RemoveHistoryCacheOldest() {
	ele := c.historyCache.ll.Front()
	if ele != nil {
		c.historyCache.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.historyCache.mp, kv.key)
		delete(c.historyCache.cnt, kv.key)
		c.historyCache.useBytes -= int64(kv.value.Len()) + int64(len(kv.key))
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Len is the number of cache entries
func (c *Cache) Len() int {
	return c.ll.Len()
}
