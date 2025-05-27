package refresh

import (
	"context"
	"sync"
	"time"

	"github.com/LerianStudio/lib-commons/commons/log"
)

// Validator defines the interface for license validation
type Validator interface {
	ValidateWithRetry(ctx context.Context) error
}

// Manager handles background refresh of license validation
type Manager struct {
	refreshInterval    time.Duration
	started            bool
	mu                 sync.Mutex
	cancel             context.CancelFunc
	validator          Validator
	logger             log.Logger
	lastAttemptedRefresh time.Time
	lastSuccessfulRefresh time.Time
}

// New creates a new background refresh manager
func New(validator Validator, refreshInterval time.Duration, logger log.Logger) *Manager {
	return &Manager{
		validator:       validator,
		refreshInterval: refreshInterval,
		logger:          logger,
	}
}

// Start begins the background refresh process
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	
	refreshCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.started = true
	m.mu.Unlock()

	// Start a ticker to periodically refresh the license
	ticker := time.NewTicker(m.refreshInterval)
	
	go func() {
		m.logger.Info("Starting background license refresh")
		
		for {
			select {
			case <-refreshCtx.Done():
				ticker.Stop()
				m.logger.Info("Background license refresh stopped")
				return
				
			case <-ticker.C:
				m.logger.Info("Running scheduled license validation")
				m.attemptValidation(refreshCtx)
			}
		}
	}()
}

// Shutdown stops the background refresh process
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.started {
		return
	}
	
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	
	m.started = false
	m.logger.Info("Background license refresh shutdown complete")
}

// attemptValidation performs a validation with retry logic
func (m *Manager) attemptValidation(ctx context.Context) {
	m.mu.Lock()
	m.lastAttemptedRefresh = time.Now()
	m.mu.Unlock()
	
	m.logger.Info("Attempting license validation with retry")
	
	err := m.validator.ValidateWithRetry(ctx)
	if err == nil {
		m.mu.Lock()
		m.lastSuccessfulRefresh = time.Now()
		m.mu.Unlock()
		
		m.logger.Info("License validation successful")
	} else {
		m.logger.Errorf("License validation failed after retries: %v", err)
	}
}
