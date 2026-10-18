package avatar_generator

import (
	"container/list"
	"strconv"
	"sync"
	"time"
)

// CacheConfig configures a Cache.
//
// TTL answers "how long is this result valid". MaxItems and MaxBytes answer
// "what do I drop when memory fills". Both limits are enforced; whichever is
// hit first evicts the least recently used entry.
type CacheConfig struct {
	// TTL is how long a generated avatar stays valid. Zero or negative means
	// entries never expire (bounded only by MaxItems/MaxBytes).
	TTL time.Duration

	// MaxItems caps the number of stored avatars.
	MaxItems int

	// MaxBytes caps total PNG bytes held. Zero disables the byte cap.
	MaxBytes int64
}

// DefaultCacheConfig is a sensible production default: five minutes, ten
// thousand avatars, and a 256MB ceiling.
//
// The byte cap matters because item count alone says nothing about memory: at
// 256px a PNG is roughly 5-10KB, but a 512px avatar is 4x the pixels.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		TTL:      5 * time.Minute,
		MaxItems: 10_000,
		MaxBytes: 256 << 20, // 256 MiB
	}
}

// Stats is a point-in-time snapshot of cache counters.
type Stats struct {
	Items     int
	Bytes     int64
	Hits      uint64
	Misses    uint64
	Evictions uint64
}

// cacheEntry is one stored avatar. key is kept so eviction can find it in the
// index without scanning the list element's payload for it.
type cacheEntry struct {
	key     string
	png     []byte
	expires time.Time
}

// Cache is a TTL + LRU cache of generated avatars. It is safe for concurrent
// use. A nil *Cache is valid and simply generates without caching, which keeps
// call sites free of nil checks.
type Cache struct {
	mu    sync.Mutex
	items map[string]*list.Element
	order *list.List // front = most recently used, back = next eviction

	ttl      time.Duration
	maxItems int
	maxBytes int64

	bytes     int64
	hits      uint64
	misses    uint64
	evictions uint64
}

// NewCache builds a cache. Non-positive limits fall back to
// DefaultCacheConfig, so a zero-value config is usable.
func NewCache(cfg CacheConfig) *Cache {
	def := DefaultCacheConfig()

	if cfg.TTL == 0 {
		cfg.TTL = def.TTL
	}
	if cfg.MaxItems <= 0 {
		cfg.MaxItems = def.MaxItems
	}
	if cfg.MaxBytes < 0 {
		cfg.MaxBytes = def.MaxBytes
	}

	return &Cache{
		items:    make(map[string]*list.Element, 256),
		order:    list.New(),
		ttl:      cfg.TTL,
		maxItems: cfg.MaxItems,
		maxBytes: cfg.MaxBytes,
	}
}

// PNG returns the avatar for (id, size, paddingPct), generating it on a miss.
// The bool reports whether the result came from the cache.
//
// The returned slice is shared with the cache and must not be modified. Copy it
// if you need to mutate it.
func (c *Cache) PNG(id string, size int, paddingPct float64) ([]byte, bool, error) {
	if len(id) > MaxIDLength {
		return nil, false, ErrIDTooLong
	}

	size = clampSize(size)
	paddingPct = clampPadding(paddingPct)
	key := cacheKey(id, size, paddingPct)

	if png, ok := c.get(key); ok {
		return png, true, nil
	}

	// Generate outside the lock: drawing must never serialize the whole server
	// behind one avatar. Two concurrent misses for the same key both draw and
	// write the same bytes, which is harmless.
	png, err := renderPNG(specFromID(id), size, paddingPct)
	if err != nil {
		return nil, false, err
	}

	c.put(key, png)
	return png, false, nil
}

// get moves a live entry to the front and reports a hit. Expired entries are
// removed lazily, which avoids a janitor goroutine; MaxItems already bounds
// memory so stale entries cannot accumulate without limit.
func (c *Cache) get(key string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		c.misses++
		return nil, false
	}

	e := el.Value.(*cacheEntry)
	if c.ttl > 0 && time.Now().After(e.expires) {
		c.remove(el)
		c.evictions++
		c.misses++
		return nil, false
	}

	c.order.MoveToFront(el)
	c.hits++
	return e.png, true
}

func (c *Cache) put(key string, png []byte) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// A concurrent miss may have inserted the same key first; keep that one and
	// avoid double counting bytes.
	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		return
	}

	e := &cacheEntry{key: key, png: png}
	if c.ttl > 0 {
		e.expires = time.Now().Add(c.ttl)
	}

	c.items[key] = c.order.PushFront(e)
	c.bytes += int64(len(png))

	c.evict()
}

// evict drops entries from the back until both limits are satisfied. Callers
// must hold the mutex.
func (c *Cache) evict() {
	for c.order.Len() > 0 && (c.order.Len() > c.maxItems || (c.maxBytes > 0 && c.bytes > c.maxBytes)) {
		back := c.order.Back()
		if back == nil {
			return
		}
		c.remove(back)
		c.evictions++
	}
}

// remove deletes one element. Callers must hold the mutex.
func (c *Cache) remove(el *list.Element) {
	e := el.Value.(*cacheEntry)
	c.order.Remove(el)
	delete(c.items, e.key)
	c.bytes -= int64(len(e.png))
}

// Len reports the number of cached avatars.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Stats returns counters and current occupancy. Hits, Misses and Evictions are
// cumulative since the last Purge.
func (c *Cache) Stats() Stats {
	if c == nil {
		return Stats{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return Stats{
		Items:     c.order.Len(),
		Bytes:     c.bytes,
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
	}
}

// Purge drops every entry and resets the counters. Useful after a visual
// redesign, where every previously generated avatar is now stale.
func (c *Cache) Purge() {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element, 256)
	c.order.Init()
	c.bytes = 0
	c.hits, c.misses, c.evictions = 0, 0, 0
}

// cacheKey must include every input that changes the pixels. Padding is
// formatted at fixed precision so what the generator clamps is what gets keyed.
func cacheKey(id string, size int, paddingPct float64) string {
	var b []byte
	b = append(b, id...)
	b = append(b, 0)
	b = strconv.AppendInt(b, int64(size), 10)
	b = append(b, 0)
	b = strconv.AppendFloat(b, paddingPct, 'f', 4, 64)
	return string(b)
}
