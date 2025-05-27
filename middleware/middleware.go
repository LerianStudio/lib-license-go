package middleware

import (
	"context"
	"net/http"

	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/zap"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/validation"
	"github.com/gofiber/fiber/v2"
)

// LicenseClient is the public client API that exposes middleware functionality
// It's a wrapper around the internal validation client
type LicenseClient struct {
	validator *validation.Client
	logger    log.Logger
}

// NewLicenseClient creates a new license client with middleware capabilities
func NewLicenseClient(appID, licenseKey, orgID, env string, logger *log.Logger) *LicenseClient {
	validator, err := validation.New(appID, licenseKey, orgID, env, logger)
	if err != nil {
		// If we can't create the validator, we'll return a nil client
		// This will be caught by the middleware and will bypass validation
		return nil
	}

	// Use the logger from the validator if it exists
	var l log.Logger
	if logger != nil {
		l = *logger
	} else if validator != nil {
		l = zap.InitializeLogger()
	}

	return &LicenseClient{
		validator: validator,
		logger:    l,
	}
}

// Middleware creates a Fiber middleware that validates the license
func (c *LicenseClient) Middleware() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		// Start background refresh if client is valid
		if c != nil && c.validator != nil {
			// Start background validation in a separate goroutine
			// This will continuously validate the license in the background
			go c.validator.StartBackgroundRefresh(ctx.Context())
		}

		// Always proceed to the next handler
		// The background validation will terminate the application if needed
		return ctx.Next()
	}
}

// Validate checks if the license is valid
func (c *LicenseClient) Validate(ctx context.Context) (model.ValidationResult, error) {
	if c == nil || c.validator == nil {
		return model.ValidationResult{}, fiber.ErrInternalServerError
	}

	return c.validator.Validate(ctx)
}

// SetHTTPClient allows overriding the HTTP client (useful for testing)
func (c *LicenseClient) SetHTTPClient(client *http.Client) {
	if c != nil && c.validator != nil {
		c.validator.SetHTTPClient(client)
	}
}

// SetTerminationHandler allows customizing how the application terminates when license validation fails
func (c *LicenseClient) SetTerminationHandler(handler func(reason string)) {
	if c != nil && c.validator != nil {
		c.validator.SetTerminationHandler(handler)
	}
}

// ShutdownBackgroundRefresh stops the background refresh process
func (c *LicenseClient) ShutdownBackgroundRefresh() {
	if c != nil && c.validator != nil {
		c.validator.ShutdownBackgroundRefresh()
	}
}

// GetLogger returns the logger used by the client
func (c *LicenseClient) GetLogger() log.Logger {
	return c.logger
}
