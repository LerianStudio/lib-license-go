package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/LerianStudio/lib-license-go/middleware"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/test/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestCase is defined in validator_test_helper.go

const (
	testAppID      = "test-app"
	testLicenseKey = "test-key"
	testOrgID      = "test-org"
	testEnv        = "test-env"
)

// mockHTTPClient returns a test server URL and a function to close it
func mockHTTPClient(t *testing.T, statusCode int, response any) (string, func()) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if response != nil {
			json.NewEncoder(w).Encode(response)
		}
	}))

	return server.URL, server.Close
}

func TestLicenseValidation(t *testing.T) {
	setupTestEnv(t)

	tests := []struct {
		name          string
		setupMocks    func(*helper.MockLogger)
		testCase      TestCase
		expectError   bool
		expectedValid bool
		expectedDays  int
	}{
		{
			name: "Valid license with 30 days left",
			setupMocks: func(l *helper.MockLogger) {
				l.On("Infof", mock.Anything, mock.Anything).Maybe()
				l.On("Warnf", "License expires in %d days", 30).Once()
			},
			expectError:   false,
			expectedValid: true,
			expectedDays:  30,
			testCase: TestCase{
				Name: "Valid license with 30 days left",
				SetupServer: func(t *testing.T) *httptest.Server {
					return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						assert.Equal(t, "/test-env/licenses/validate", r.URL.Path)
						assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

						// Parse request body
						var reqBody map[string]string
						err := json.NewDecoder(r.Body).Decode(&reqBody)
						require.NoError(t, err)
						assert.Equal(t, "test-key", reqBody["licenseKey"])

						// Return a valid license response
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						json.NewEncoder(w).Encode(model.ValidationResult{
							Valid:             true,
							ExpiryDaysLeft:    30,
							ActiveGracePeriod: false,
						})
					}))
				},
				ExpectedValid: true,
				ExpectedDays:  30,
			},
		},
		{
			name: "Expired license in grace period",
			setupMocks: func(l *helper.MockLogger) {
				l.On("Infof", mock.Anything, mock.Anything).Maybe()
				l.On("Warnf", "CRITICAL: Grace period ends in %d days - application will terminate. Contact support immediately to renew license", 5).Once()
			},
			expectError:   false,
			expectedValid: false,
			expectedDays:  5,
			testCase: TestCase{
				Name: "Expired license in grace period",
				SetupServer: func(t *testing.T) *httptest.Server {
					return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// Parse request body
						var reqBody map[string]string
						err := json.NewDecoder(r.Body).Decode(&reqBody)
						require.NoError(t, err)

						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						json.NewEncoder(w).Encode(model.ValidationResult{
							Valid:             false,
							ExpiryDaysLeft:    5,
							ActiveGracePeriod: true,
						})
					}))
				},
				ExpectedValid: false,
				ExpectedDays:  5,
			},
		},
		{
			name: "Invalid license",
			setupMocks: func(l *helper.MockLogger) {
				l.On("Infof", mock.Anything, mock.Anything).Maybe()
				l.On("Debugf", "Client error during license validation - status: %d, code: %s, message: %s", 403, "INVALID_LICENSE", "invalid license").Once()
				l.On("Errorf", "Exiting: license validation failed with client error: %s", "client error: 403").Once()
			},
			expectError: true,
			testCase: TestCase{
				Name: "Invalid license",
				SetupServer: func(t *testing.T) *httptest.Server {
					return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusForbidden)
						json.NewEncoder(w).Encode(map[string]any{
							"code":    "INVALID_LICENSE",
							"message": "invalid license",
						})
					}))
				},
				ExpectedValid: false,
			},
			expectedValid: false,
		},
		{
			name: "Server error",
			setupMocks: func(l *helper.MockLogger) {
				l.On("Infof", mock.Anything, mock.Anything).Maybe()
			},
			testCase: TestCase{
				Name: "Server error",
				SetupServer: func(t *testing.T) *httptest.Server {
					return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						json.NewEncoder(w).Encode(map[string]any{
							"code":    "INTERNAL_SERVER_ERROR",
							"message": "internal server error",
						})
					}))
				},
				ExpectedValid: false,
			},
			expectError:   true,
			expectedValid: false,
		},
		{
			name: "Server error",
			setupMocks: func(l *helper.MockLogger) {
				l.On("Infof", mock.Anything, mock.Anything).Maybe()
				l.On("Debugf", "Server error during license validation - status: %d, code: %s, message: %s", 500, "", "").Once()
				l.On("Warnf", "License server error (5xx) detected, treating as valid - error: %s", "server error: 500").Once()
			},
			expectError:   false,
			expectedValid: true, // When server error occurs, fallback to valid license with grace period
			expectedDays:  0,
			testCase: TestCase{
				Name: "Server error",
				SetupServer: func(t *testing.T) *httptest.Server {
					return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// Simulate a server error
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusInternalServerError)
						json.NewEncoder(w).Encode(map[string]string{
							"error": "server error",
						})
					}))
				},
				ExpectedValid: true,
				ExpectedDays:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock logger
			mockLogger := helper.NewMockLogger()
			mockLoggerImpl := helper.AsMock(mockLogger)
			if tt.setupMocks != nil {
				tt.setupMocks(mockLoggerImpl)
			}

			// Set up test server
			ts := tt.testCase.SetupServer(t)
			defer ts.Close()

			// Create a custom HTTP client that points to our test server
			httpClient := newTestClient(ts)

			// Set required environment variables
			t.Setenv("TEST_MIDAZ_LICENSE_URL", ts.URL)
			t.Setenv("MIDAZ_ORGANIZATION_ID", testOrgID)
			t.Setenv("PLUGIN_ENVIRONMENT", testEnv)

			// Create a new client with the mock logger and custom HTTP client
			client := middleware.NewLicenseClient(testAppID, testLicenseKey, testOrgID, testEnv, mockLogger)
			// Override the HTTP client to use our test client
			client.SetHTTPClient(httpClient)

			if tt.expectError {
				// For error cases, we expect a panic with a specific error message
				assert.Panics(t, func() {
					_, _ = client.Validate(context.Background())
				}, "Expected panic for license validation error")
			} else {
				// For success cases, verify the validation result
				result, err := client.Validate(context.Background())
				assert.NoError(t, err)

				if tt.expectedValid {
					assert.True(t, result.Valid)
				} else {
					assert.False(t, result.Valid)
				}

				if tt.expectedDays > 0 {
					assert.Equal(t, tt.expectedDays, result.ExpiryDaysLeft)
				}
			}

			// Verify all expected mock calls were made
			mockLoggerImpl.AssertExpectations(t)
		})
	}
}

// setupTestEnv sets up the required environment variables for testing
func setupTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MIDAZ_ORGANIZATION_ID", testOrgID)
	t.Setenv("PLUGIN_ENVIRONMENT", testEnv)
	// Don't set MIDAZ_LICENSE_URL here - let the test cases set it
}

// newTestClient returns a client with a custom transport for testing
func newTestClient(server *httptest.Server) *http.Client {
	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
}

func TestLicenseClient_Integration(t *testing.T) {
	// Skip this test in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setupTestEnv(t)

	// This is a basic integration test that can be expanded
	// It requires the license server to be running
	client := middleware.NewLicenseClient(testAppID, testLicenseKey, testOrgID, testEnv, nil)
	// We don't assert on the result since it depends on the environment
	// Just check that the function doesn't panic
	assert.NotPanics(t, func() {
		_, _ = client.Validate(context.Background())
	})
}
