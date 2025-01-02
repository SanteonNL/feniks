package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type BundleCache struct {
	entries  sync.Map // map[string]*ResultSetCache
	config   CacheConfig
	log      zerolog.Logger
	stopChan chan struct{}
}

// ResultSetCache holds the complete search results
type ResultSetCache struct {
	Resources    []interface{} // The actual FHIR resources
	Issues       []SearchIssue // Any issues encountered during search
	Total        int           // Total number of resources
	SearchParams string        // Original search parameters
	CreatedAt    time.Time     // When this cache entry was created
	ExpiresAt    time.Time     // When this cache entry expires
}
type CacheConfig struct {
	// Enabled determines if caching is active
	// When false, the service will bypass the cache completely
	Enabled bool

	// DefaultTTL is the default time-to-live for cached entries
	// After this duration, entries are considered expired and will be removed
	// Example: 15 * time.Minute for 15 minutes TTL
	DefaultTTL time.Duration

	// MaxSize is the maximum number of result sets to keep in cache
	// When exceeded, oldest entries will be removed first
	// Set to 0 for unlimited size
	MaxSize int

	// CleanupInterval defines how often the cleanup routine runs
	// to remove expired entries and enforce MaxSize
	// Example: 5 * time.Minute for cleanup every 5 minutes
	CleanupInterval time.Duration
}

// DefaultCacheConfig returns a CacheConfig with sensible defaults
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:         true,             // Cache is enabled by default
		DefaultTTL:      15 * time.Minute, // Cache entries expire after 15 minutes
		MaxSize:         1000,             // Store up to 1000 result sets
		CleanupInterval: 5 * time.Minute,  // Run cleanup every 5 minutes
	}
}

// NewBundleCache creates and initializes a new bundle cache
func NewBundleCache(config CacheConfig, log zerolog.Logger) *BundleCache {
	// Validate configuration

	// Initialize cache
	cache := &BundleCache{
		config:   config,
		log:      log.With().Str("component", "bundle_cache").Logger(),
		entries:  sync.Map{},
		stopChan: make(chan struct{}),
	}

	// Start cleanup routine if enabled
	if config.Enabled && config.CleanupInterval > 0 {
		go cache.startCleanupRoutine()
		cache.log.Info().
			Dur("interval", config.CleanupInterval).
			Int("max_size", config.MaxSize).
			Dur("ttl", config.DefaultTTL).
			Msg("Started cache cleanup routine")
	}

	return cache
}

func (c *BundleCache) startCleanupRoutine() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopChan:
			c.log.Info().Msg("Stopping cache cleanup routine")
			return
		}
	}
}

func (c *BundleCache) cleanup() {
	var (
		totalEntries   int
		expiredEntries int
		removedEntries int
		now            = time.Now()
		entries        = make([]*ResultSetCache, 0)
	)

	// First pass: collect stats and remove expired entries
	c.entries.Range(func(key, value interface{}) bool {
		totalEntries++
		resultSet := value.(*ResultSetCache)

		if now.After(resultSet.ExpiresAt) {
			c.entries.Delete(key)
			expiredEntries++
		} else {
			entries = append(entries, resultSet)
		}
		return true
	})

	// Second pass: enforce size limit if needed
	if c.config.MaxSize > 0 && len(entries) > c.config.MaxSize {
		// Sort by creation time, oldest first
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].CreatedAt.Before(entries[j].CreatedAt)
		})

		// Remove oldest entries until we're under the limit
		toRemove := len(entries) - c.config.MaxSize
		c.entries.Range(func(key, value interface{}) bool {
			if toRemove <= 0 {
				return false
			}

			resultSet := value.(*ResultSetCache)
			for _, oldEntry := range entries[:toRemove] {
				if resultSet.CreatedAt == oldEntry.CreatedAt {
					c.entries.Delete(key)
					removedEntries++
					toRemove--
					break
				}
			}
			return true
		})
	}

	c.log.Debug().
		Int("total_entries", totalEntries).
		Int("expired_removed", expiredEntries).
		Int("size_limit_removed", removedEntries).
		Int("remaining_entries", len(entries)-removedEntries).
		Msg("Completed cache cleanup")
}

func (c *BundleCache) generateCacheKey(resourceType, searchParams string) string {
	hasher := sha256.New()
	hasher.Write([]byte(resourceType + searchParams))
	return hex.EncodeToString(hasher.Sum(nil))
}

func (c *BundleCache) StoreResultSet(resourceType string, searchParams string, result SearchResult) {
	if !c.config.Enabled {
		return
	}

	cacheKey := c.generateCacheKey(resourceType, searchParams)
	resultSet := &ResultSetCache{
		Resources:    result.Resources,
		Issues:       result.Issues,
		Total:        result.Total,
		SearchParams: searchParams,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(c.config.DefaultTTL),
	}

	c.entries.Store(cacheKey, resultSet)
	c.log.Debug().
		Str("key", cacheKey).
		Int("total_resources", len(result.Resources)).
		Time("expires", resultSet.ExpiresAt).
		Msg("Stored complete result set in cache")
}

func (c *BundleCache) GetPageFromCache(resourceType, searchParams string, offset, count int) (*SearchResult, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	cacheKey := c.generateCacheKey(resourceType, searchParams)
	if entry, ok := c.entries.Load(cacheKey); ok {
		resultSet := entry.(*ResultSetCache)
		if time.Now().After(resultSet.ExpiresAt) {
			c.entries.Delete(cacheKey)
			return nil, false
		}

		// Calculate page boundaries
		start := offset
		if start < 0 {
			start = 0
		}
		end := start + count
		if end > len(resultSet.Resources) {
			end = len(resultSet.Resources)
		}

		// Return paginated result
		pagedResult := &SearchResult{
			Resources: resultSet.Resources[start:end],
			Issues:    resultSet.Issues,
			Total:     resultSet.Total,
		}

		c.log.Debug().
			Str("key", cacheKey).
			Int("offset", offset).
			Int("count", count).
			Int("returned_resources", len(pagedResult.Resources)).
			Msg("Retrieved page from cached result set")

		return pagedResult, true
	}

	return nil, false
}

// Stop gracefully shuts down the cache
func (c *BundleCache) Stop() {
	if c.config.Enabled && c.config.CleanupInterval > 0 {
		close(c.stopChan)
		c.log.Info().Msg("Cache cleanup routine stopped")
	}

	// Clear all entries
	c.entries.Range(func(key, _ interface{}) bool {
		c.entries.Delete(key)
		return true
	})

	c.log.Info().Msg("Cache cleared and stopped")
}
