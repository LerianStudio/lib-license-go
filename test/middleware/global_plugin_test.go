package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cn "github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/internal/api"
	"github.com/LerianStudio/lib-license-go/middleware"
	"github.com/LerianStudio/lib-license-go/test/helper"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGlobalPluginMode tests the license validation middleware in global plugin mode
func TestGlobalPluginMode(t *testing.T) {
	t.Parallel()

	// Create a test HTTP server that returns a valid license response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the request body to verify the organization ID
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify the global organization ID was sent in the request
		orgID, exists := reqBody["organizationId"]
		assert.True(t, exists, "Request should contain organizationId")
		assert.Equal(t, cn.GlobalPluginValue, orgID, "Should use global as organization ID")

		// Return valid license response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":           true,
			"expiry_date":     time.Now().AddDate(0, 0, 30).Format("2006-01-02"),
			"expiry_days_left": 30,
			"is_trial":        false,
			"grace_period":    false,
		})
	}))
	defer ts.Close()

	// Override the license API URL for testing
	api.SetTestLicenseBaseURL(ts.URL)

	// Setup mock logger
	mockLogger := helper.NewMockLogger()
	mockLoggerImpl := helper.AsMock(mockLogger)
	mockLoggerImpl.On("Info", mock.Anything).Maybe()
	mockLoggerImpl.On("Infof", mock.Anything, mock.Anything).Maybe()
	mockLoggerImpl.On("Infof", mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockLoggerImpl.On("Debug", mock.Anything).Maybe()
	mockLoggerImpl.On("Debugf", mock.Anything, mock.Anything).Maybe()
	mockLoggerImpl.On("Debugf", mock.Anything, mock.Anything, mock.Anything).Maybe()
	
	t.Run("Valid Global License - Successful Initialization", func(t *testing.T) {
		// Create a license client with global plugin mode
		assert.NotPanics(t, func() {
			client := middleware.NewLicenseClient(
				"test-app", 
				cn.GlobalPluginValue, 
				"test-license-key", 
				mockLogger,
			)

			// Create a custom HTTP client that points to our test server
			httpClient := newTestClient(ts)
			client.SetHTTPClient(httpClient)
			
			// Create middleware - this triggers initial validation
			_ = client.Middleware()
		})
	})

	t.Run("Global Plugin - Request Handling", func(t *testing.T) {
		// Create a license client with global plugin mode
		client := middleware.NewLicenseClient(
			"test-app", 
			cn.GlobalPluginValue, 
			"test-license-key", 
			mockLogger,
		)
		
		// Create a custom HTTP client that points to our test server
		httpClient := newTestClient(ts)
		client.SetHTTPClient(httpClient)
		
		// Create the middleware handler
		middlewareHandler := client.Middleware()
		
		// Create Fiber app with middleware
		app := fiber.New()
		app.Use(middlewareHandler)
		app.Get("/test", func(c *fiber.Ctx) error {
			return c.SendString("success")
		})
		
		// Test a request without organization header (should pass in global mode)
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		
		// Should succeed even without organization header
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
