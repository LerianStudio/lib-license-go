package cache

import (
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/dgraph-io/ristretto/v2"
)

// Manager handles caching of license validation results
type Manager struct {
	cache  *ristretto.Cache[string, model.ValidationResult]
	logger log.Logger
}

// New creates a new cache manager
func New(logger log.Logger) (*Manager, error) {
	cache, err := ristretto.NewCache[string, model.ValidationResult](&ristretto.Config[string, model.ValidationResult]{
		NumCounters:            constant.CacheNumCounters,
		MaxCost:                constant.CacheMaxCost,
		BufferItems:            constant.CacheBufferItems,
		TtlTickerDurationInSec: constant.CacheTTLTickerDurationInSec,
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
		m.logger.Debugf("License cached for org %s [expires: %d days | grace: %t]",
			orgID, val.ExpiryDaysLeft, val.ActiveGracePeriod)
		return val, true
	}

	return model.ValidationResult{}, false
}

// Store caches a validation result with a fixed TTL
func (m *Manager) Store(orgID string, result model.ValidationResult) {
	// Store with a fixed TTL for security (ensure regular re-validation)
	m.cache.SetWithTTL(orgID, result, 1, constant.CacheTTL)

	// Wait for any pending writes to complete
	m.cache.Wait()

	// Log the cached result with a simpler format for test compatibility
	m.logger.Debugf("Stored license validation for org %s", orgID)
}
