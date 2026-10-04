package fhirpath

import (
	"sync"
	"sync/atomic"
)

// ExpressionCache provides thread-safe caching of compiled FHIRPath expressions
// with LRU eviction. Use this in production to avoid recompiling the same expressions.
//
// A hit takes only a read lock, so many goroutines can hit the cache at once,
// as a validator evaluating in parallel does: it marks the entry referenced,
// atomically, and nothing else. Eviction approximates least recently used the
// way CLOCK does: a hand sweeps the entries in a ring, sparing one referenced
// since it last passed and clearing the mark, and evicting the first that was
// not. That costs a miss on a full cache a few steps, not a pass over every
// entry, which matters to a caller compiling expressions built on the fly.
type ExpressionCache struct {
	mu     sync.RWMutex
	cache  map[string]*cacheEntry
	ring   []string // the cached expressions in the order the hand visits them
	hand   int
	limit  int
	hits   atomic.Int64
	misses atomic.Int64
}

type cacheEntry struct {
	expr       *Expression
	slot       int         // the entry's place in the ring
	referenced atomic.Bool // hit since the hand last passed
}

// CacheStats holds cache performance statistics.
type CacheStats struct {
	Size   int
	Limit  int
	Hits   int64
	Misses int64
}

// NewExpressionCache creates a new cache with the given size limit.
// If limit <= 0, the cache is unbounded.
func NewExpressionCache(limit int) *ExpressionCache {
	return &ExpressionCache{
		cache: make(map[string]*cacheEntry),
		limit: limit,
	}
}

// Get retrieves a compiled expression from the cache, compiling it if necessary.
func (c *ExpressionCache) Get(expr string) (*Expression, error) {
	c.mu.RLock()
	if entry, ok := c.cache[expr]; ok {
		entry.referenced.Store(true)
		c.hits.Add(1)
		c.mu.RUnlock()
		return entry.expr, nil
	}
	c.mu.RUnlock()

	// Compile the expression
	compiled, err := Compile(expr)
	if err != nil {
		return nil, err
	}

	// Store in cache with write lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, ok := c.cache[expr]; ok {
		entry.referenced.Store(true)
		return entry.expr, nil
	}

	c.misses.Add(1)

	entry := &cacheEntry{expr: compiled}
	if c.limit > 0 && len(c.cache) >= c.limit {
		// The new entry takes the evicted one's place in the ring.
		entry.slot = c.evict()
		c.ring[entry.slot] = expr
	} else {
		entry.slot = len(c.ring)
		c.ring = append(c.ring, expr)
	}
	c.cache[expr] = entry

	return compiled, nil
}

// evict removes the entry the hand settles on and returns its place in the
// ring. Must be called with write lock held, on a cache with entries.
func (c *ExpressionCache) evict() int {
	for {
		slot := c.hand
		c.hand = (c.hand + 1) % len(c.ring)

		entry := c.cache[c.ring[slot]]
		if entry.referenced.Swap(false) {
			continue
		}
		delete(c.cache, c.ring[slot])
		return slot
	}
}

// MustGet is like Get but panics on error.
func (c *ExpressionCache) MustGet(expr string) *Expression {
	compiled, err := c.Get(expr)
	if err != nil {
		panic(err)
	}
	return compiled
}

// Clear removes all cached expressions.
func (c *ExpressionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*cacheEntry)
	c.ring = nil
	c.hand = 0
	c.hits.Store(0)
	c.misses.Store(0)
}

// Size returns the number of cached expressions.
func (c *ExpressionCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Stats returns cache performance statistics.
func (c *ExpressionCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Size:   len(c.cache),
		Limit:  c.limit,
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
	}
}

// HitRate returns the cache hit rate as a percentage (0-100).
func (c *ExpressionCache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	hits := c.hits.Load()
	total := hits + c.misses.Load()
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// DefaultCache is a global expression cache for convenience.
// Use NewExpressionCache for finer control over cache lifetime.
var DefaultCache = NewExpressionCache(1000)

// GetCached retrieves or compiles an expression using the default cache.
func GetCached(expr string) (*Expression, error) {
	return DefaultCache.Get(expr)
}

// MustGetCached is like GetCached but panics on error.
func MustGetCached(expr string) *Expression {
	return DefaultCache.MustGet(expr)
}

// EvaluateCached compiles (with caching) and evaluates a FHIRPath expression.
// This is the recommended function for production use.
func EvaluateCached(resource []byte, expr string) (Collection, error) {
	compiled, err := DefaultCache.Get(expr)
	if err != nil {
		return nil, err
	}
	return compiled.Evaluate(resource)
}
