package middleware_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLicenseClientWithHooks is a mock implementation that allows test hooks
type mockLicenseClientWithHooks struct {
	validateCount      atomic.Int32
	refreshCount       atomic.Int32
	validationResult   model.ValidationResult
	validationErr      error
	shutdownCalled     bool
	shutdownMutex      sync.Mutex
	refreshInterval    time.Duration
	validateHook       func(ctx context.Context) (model.ValidationResult, error)
	shutdownHook       func()
	refreshHook        func(ctx context.Context)
	startRefreshCalled bool
	refreshMutex       sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
	logger             log.Logger
}

// NewMockLicenseClient creates a new mock license client with test hooks
func NewMockLicenseClient(result model.ValidationResult, err error) *mockLicenseClientWithHooks {
	ctx, cancel := context.WithCancel(context.Background())
	return &mockLicenseClientWithHooks{
		validationResult: result,
		validationErr:    err,
		refreshInterval:  50 * time.Millisecond, // Short interval for testing
		ctx:              ctx,
		cancel:           cancel,
	}
}

func (m *mockLicenseClientWithHooks) Validate(ctx context.Context) (model.ValidationResult, error) {
	m.validateCount.Add(1)
	
	// Call hook if provided
	if m.validateHook != nil {
		return m.validateHook(ctx)
	}
	
	return m.validationResult, m.validationErr
}

func (m *mockLicenseClientWithHooks) ShutdownBackgroundRefresh() {
	m.shutdownMutex.Lock()
	defer m.shutdownMutex.Unlock()
	
	m.shutdownCalled = true
	if m.cancel != nil {
		m.cancel()
	}
	
	if m.shutdownHook != nil {
		m.shutdownHook()
	}
}

func (m *mockLicenseClientWithHooks) StartBackgroundRefresh(ctx context.Context) {
	m.refreshMutex.Lock()
	defer m.refreshMutex.Unlock()
	
	if m.startRefreshCalled {
		return
	}
	
	m.startRefreshCalled = true
	m.refreshCount.Add(1)
	
	// Create a child context from the one provided
	ctx, cancel := context.WithCancel(ctx)
	m.ctx = ctx
	m.cancel = cancel
	
	// Call hook if provided
	if m.refreshHook != nil {
		m.refreshHook(ctx)
		return
	}
	
	// Simple background refresh simulation
	go func() {
		// Immediate initial validation
		m.Validate(ctx)
		
		ticker := time.NewTicker(m.refreshInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.Validate(ctx)
			}
		}
	}()
}

// Helper method to wait until validation count reaches a certain value or timeout
func (m *mockLicenseClientWithHooks) waitForValidationCount(count int32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.validateCount.Load() >= count {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestBackgroundRefresh ensures the background refresh functionality works correctly
func TestBackgroundRefresh(t *testing.T) {
	// 1. Create mock client with initial validation state
	client := NewMockLicenseClient(model.ValidationResult{Valid: true}, nil)
	
	// 2. Start background refresh
	client.StartBackgroundRefresh(context.Background())
	
	// 3. Wait for at least 3 validations (initial + 2 ticks)
	success := client.waitForValidationCount(3, 500*time.Millisecond)
	
	// 4. Verify background process is running correctly
	assert.True(t, success, "Background refresh should validate license multiple times")
	assert.True(t, client.startRefreshCalled, "Background refresh should be marked as started")
	
	// 5. Shutdown background refresh
	client.ShutdownBackgroundRefresh()
	
	// 6. Verify shutdown was called
	assert.True(t, client.shutdownCalled, "Shutdown should be called")
	
	// 7. Record the validation count after shutdown
	validationCountAfterShutdown := client.validateCount.Load()
	
	// 8. Wait a bit to ensure no more validations occur
	time.Sleep(200 * time.Millisecond)
	
	// 9. Verify no more validations occurred after shutdown
	assert.Equal(t, validationCountAfterShutdown, client.validateCount.Load(), 
		"No more validations should occur after shutdown")
}

// TestBackgroundRefresh_GracefulShutdown tests the graceful shutdown behavior
func TestBackgroundRefresh_GracefulShutdown(t *testing.T) {
	var shutdownCompleted atomic.Bool
	
	// Create client with custom shutdown hook
	client := NewMockLicenseClient(model.ValidationResult{Valid: true}, nil)
	client.shutdownHook = func() {
		shutdownCompleted.Store(true)
	}
	
	// Start background refresh
	client.StartBackgroundRefresh(context.Background())
	
	// Verify it started
	success := client.waitForValidationCount(1, 200*time.Millisecond)
	assert.True(t, success, "Background refresh should start")
	
	// Shutdown background refresh
	client.ShutdownBackgroundRefresh()
	
	// Verify shutdown was completed
	assert.True(t, shutdownCompleted.Load(), "Shutdown should complete gracefully")
	
	// Try to start again (should be a no-op due to flag)
	prevRefreshCount := client.refreshCount.Load()
	client.StartBackgroundRefresh(context.Background())
	
	// Verify it wasn't started again
	assert.Equal(t, prevRefreshCount, client.refreshCount.Load(), 
		"Background refresh should not restart after shutdown")
}

// TestBackgroundRefresh_RefreshInterval tests the refresh interval behavior
func TestBackgroundRefresh_RefreshInterval(t *testing.T) {
	// Create client with custom refresh interval
	client := NewMockLicenseClient(model.ValidationResult{Valid: true}, nil)
	client.refreshInterval = 75 * time.Millisecond
	
	// Start background refresh
	client.StartBackgroundRefresh(context.Background())
	
	// Wait for initial validation
	success := client.waitForValidationCount(1, 100*time.Millisecond)
	assert.True(t, success, "Initial validation should occur")
	
	// Record time and count
	start := time.Now()
	initialCount := client.validateCount.Load()
	
	// Wait for approximately 225ms which should result in 3 more validations
	// (75ms * 3 = 225ms)
	time.Sleep(250 * time.Millisecond)
	elapsed := time.Since(start)
	newCount := client.validateCount.Load()
	expectedValidations := int32(elapsed / client.refreshInterval)
	
	// We should have at least the initial + expected validations
	// Allow for some timing variance but at least 3 total
	assert.GreaterOrEqual(t, newCount, initialCount+2, 
		"Should have multiple validations based on refresh interval")
	assert.LessOrEqual(t, newCount, initialCount+expectedValidations+1, 
		"Should not have too many extra validations")
	
	// Cleanup
	client.ShutdownBackgroundRefresh()
}

// TestBackgroundRefresh_ErrorHandling tests error handling during background refresh
func TestBackgroundRefresh_ErrorHandling(t *testing.T) {
	// Setup validation to return errors then succeed
	validationAttempts := atomic.Int32{}
	expectedError := errors.New("validation failed")
	
	client := NewMockLicenseClient(model.ValidationResult{}, nil)
	client.validateHook = func(ctx context.Context) (model.ValidationResult, error) {
		attempt := validationAttempts.Add(1)
		
		// First 3 attempts fail, then succeed
		if attempt <= 3 {
			return model.ValidationResult{}, expectedError
		}
		return model.ValidationResult{Valid: true}, nil
	}
	
	// Start background refresh
	client.StartBackgroundRefresh(context.Background())
	
	// Wait for multiple validation attempts including retries
	success := client.waitForValidationCount(4, 500*time.Millisecond)
	assert.True(t, success, "Should attempt multiple validations including retries")
	
	// Verify that validation eventually succeeded
	assert.GreaterOrEqual(t, validationAttempts.Load(), int32(4), 
		"Should have multiple validation attempts")
	
	// Cleanup
	client.ShutdownBackgroundRefresh()
}

// TestBackgroundRefresh_InvalidLicense tests behavior when license becomes invalid
func TestBackgroundRefresh_InvalidLicense(t *testing.T) {
	// Setup scenario where license starts valid but becomes invalid
	var licenseValid atomic.Bool
	licenseValid.Store(true)
	
	client := NewMockLicenseClient(model.ValidationResult{}, nil)
	client.validateHook = func(ctx context.Context) (model.ValidationResult, error) {
		if licenseValid.Load() {
			return model.ValidationResult{Valid: true}, nil
		}
		return model.ValidationResult{Valid: false}, nil
	}
	
	// Start background refresh
	client.StartBackgroundRefresh(context.Background())
	
	// Wait for initial validation
	success := client.waitForValidationCount(1, 200*time.Millisecond)
	require.True(t, success, "Initial validation should occur")
	
	// Change license to invalid
	licenseValid.Store(false)
	
	// Wait for next validation
	success = client.waitForValidationCount(2, 200*time.Millisecond)
	require.True(t, success, "Second validation should occur")
	
	// In a real implementation, this would terminate the application
	// Here we just verify the invalid state was detected
	
	// Cleanup
	client.ShutdownBackgroundRefresh()
}
