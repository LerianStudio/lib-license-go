package cache

import (
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/dgraph-io/ristretto"
)

// Manager handles caching of license validation results
type Manager struct {
	cache  *ristretto.Cache
	logger log.Logger
}

// New creates a new cache manager
func New(logger log.Logger) (*Manager, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: constant.CacheNumCounters,
		MaxCost:     constant.CacheMaxCost,
		BufferItems: constant.CacheBufferItems,
	})
	if err != nil {
		return nil, err
	}

	return &Manager{
		cache:  cache,
		logger: logger,
	}, nil
}

// Get retrieves a cached validation result by organization ID
func (m *Manager) Get(orgID string) (model.ValidationResult, bool) {
	if val, found := m.cache.Get(orgID); found {
		if result, ok := val.(model.ValidationResult); ok {
			m.logger.Debugf("License cached for org %s [expires: %d days | grace: %t]",
				orgID, result.ExpiryDaysLeft, result.ActiveGracePeriod)
			return result, true
		}
	}

	return model.ValidationResult{}, false
}

// Store caches a validation result with a fixed TTL
func (m *Manager) Store(orgID string, result model.ValidationResult) {
	// Store with a fixed TTL for security (ensure regular re-validation)
	m.cache.SetWithTTL(orgID, result, 1, constant.CacheTTL)

	// Log the cached result with a simpler format for test compatibility
	m.logger.Debugf("Stored license validation for org %s", orgID)
}


