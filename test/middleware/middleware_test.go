package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLicenseClient implements a simplified LicenseClient for testing the middleware
type mockLicenseClient struct {
	validationResult model.ValidationResult
	validationErr    error
	logger           log.Logger
}

// Override the methods we need for testing
func (m *mockLicenseClient) Validate(ctx context.Context) (model.ValidationResult, error) {
	return m.validationResult, m.validationErr
}

func (m *mockLicenseClient) ShutdownBackgroundRefresh() {}

func (m *mockLicenseClient) StartBackgroundRefresh(ctx context.Context) {}

// Implement the Middleware method to match the actual LicenseClient
func (m *mockLicenseClient) Middleware() fiber.Handler {
	// This is a simplified version of the actual middleware implementation
	return func(c *fiber.Ctx) error {
		res, err := m.Validate(c.Context())
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"error": "License validation failed",
			})
		}

		if !res.Valid && !res.ActiveGracePeriod {
			return c.Status(http.StatusForbidden).JSON(fiber.Map{
				"error": "Invalid license",
			})
		}

		// Propagate expiration information to response headers
		c.Set("X-License-Expiry-Days", ""+strconv.Itoa(res.ExpiryDaysLeft))
		c.Set("X-License-Grace-Period", strconv.FormatBool(res.ActiveGracePeriod))

		// Continue to the next handler
		return c.Next()
	}
}

// Helper function to setup a Fiber app with our middleware
func setupFiberApp(client *mockLicenseClient) *fiber.App {
	app := fiber.New()
	
	// Add our middleware
	app.Use(client.Middleware())
	
	// Add a test route
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("success")
	})
	
	return app
}

func TestLicenseMiddleware_ValidLicense(t *testing.T) {
	// Setup
	client := &mockLicenseClient{
		validationResult: model.ValidationResult{
			Valid:          true,
			ExpiryDaysLeft: 30,
		},
	}
	
	// Create fiber app with our middleware
	app := setupFiberApp(client)
	
	// Make a request
	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	
	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	// Check headers
	assert.Equal(t, "30", resp.Header.Get("X-License-Expiry-Days"))
	assert.Equal(t, "false", resp.Header.Get("X-License-Grace-Period"))
}

func TestLicenseMiddleware_InvalidLicense(t *testing.T) {
	// Setup
	client := &mockLicenseClient{
		validationResult: model.ValidationResult{
			Valid:          false,
			ExpiryDaysLeft: 0,
		},
	}
	
	// Create fiber app with our middleware
	app := setupFiberApp(client)
	
	// Make a request
	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	
	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestLicenseMiddleware_GracePeriod(t *testing.T) {
	// Setup
	client := &mockLicenseClient{
		validationResult: model.ValidationResult{
			Valid:             true,
			ActiveGracePeriod: true,
			ExpiryDaysLeft:    0,
		},
	}
	
	// Create fiber app with our middleware
	app := setupFiberApp(client)
	
	// Make a request
	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	
	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	
	// Check headers
	assert.Equal(t, "0", resp.Header.Get("X-License-Expiry-Days"))
	assert.Equal(t, "true", resp.Header.Get("X-License-Grace-Period"))
}

func TestLicenseMiddleware_ValidationError(t *testing.T) {
	// Setup
	client := &mockLicenseClient{
		validationErr: assert.AnError,
	}
	
	// Create fiber app with our middleware
	app := setupFiberApp(client)
	
	// Make a request
	resp, err := app.Test(httptest.NewRequest("GET", "/test", nil))
	
	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
