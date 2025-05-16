package middleware

import (
	"context"
	"fmt"
	"net/http"
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
		v.logger.Info("Background license validation stopped")
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
func (v *LicenseClient) Middleware() fiber.Handler {
	v.startBackgroundRefreshOnce()

	return func(c *fiber.Ctx) error {
		// Create a child context with timeout for validation
		ctx, cancel := context.WithTimeout(c.Context(), v.cli.Timeout)
		defer cancel()

		res, err := v.Validate(ctx)
		if err != nil {
			v.logger.Warnf("License validation failed: %s", err.Error())
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "License validation failed",
			})
		}

		if !res.Valid && !res.ActiveGracePeriod {
			v.logger.Warnf("Invalid license detected (expires in %d days, grace period: %t)", res.ExpiryDaysLeft, res.ActiveGracePeriod)
			return c.Status(http.StatusForbidden).JSON(fiber.Map{
				"error": "Invalid license",
			})
		}

		// Propagate expiration information to response headers
		c.Set("X-License-Expiry-Days", fmt.Sprintf("%d", res.ExpiryDaysLeft))
		c.Set("X-License-Grace-Period", fmt.Sprintf("%t", res.ActiveGracePeriod))

		// Continue to the next handler
		return c.Next()
	}
}
