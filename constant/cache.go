package constant

import "time"

// Cache configuration constants
const (
	// CacheTTL defines the time-to-live for cached validation results
	CacheTTL = 24 * time.Hour
	// CacheNumCounters is the number of keys to track frequency (10M)
	CacheNumCounters = 1e7
	// CacheMaxCost is the maximum cost of cache (1MB)
	CacheMaxCost = 1 << 20
	// CacheBufferItems is the number of keys per Get buffer
	CacheBufferItems = 64
	// CacheTtlTickerDurationInSec is the duration of the TTL ticker
	CacheTtlTickerDurationInSec = 60
)
