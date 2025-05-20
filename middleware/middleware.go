package middleware

import (
	"context"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// ShutdownBackgroundRefresh stops the background refresh routine.
func (v *LicenseClient) ShutdownBackgroundRefresh() {
	v.bgConfig.mu.Lock()
	defer v.bgConfig.mu.Unlock()

	if !v.bgConfig.started {
		return
	}

	// Cancel the context to signal goroutines to stop
	if v.bgConfig.cancel != nil {
		v.bgConfig.cancel()
		v.bgConfig.cancel = nil
		v.logger.Debug("Background license validation stopped")
	}
	v.bgConfig.started = false
}

// startBackgroundRefreshOnce ensures background refresh is only started once
// across all middleware instances.
var (
	bgRefreshStarted = false
	bgRefreshMutex   sync.Mutex
)

func (v *LicenseClient) startBackgroundRefreshOnce() {
	bgRefreshMutex.Lock()
	defer bgRefreshMutex.Unlock()

	if !bgRefreshStarted {
		bgRefreshStarted = true

		// Create a context that can be canceled if needed
		ctx := context.Background()

		// Start background refresh
		v.StartBackgroundRefresh(ctx)
	}
}

// Middleware returns a Fiber middleware that validates licenses.
// This middleware will automatically start background license validation
// exactly once across all instances.
// 
// The actual license validation happens in the background process which
// terminates the application if the license becomes invalid.
func (v *LicenseClient) Middleware() fiber.Handler {
	// Start background validation exactly once
	v.startBackgroundRefreshOnce()

	return func(c *fiber.Ctx) error {
		// No need to validate on every request - the background process already
		// validates and will terminate the application if license is invalid
		
		// Simply continue to the next handler
		return c.Next()
	}
}
