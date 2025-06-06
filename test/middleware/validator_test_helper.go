package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/lib-commons/commons/log"
	cn "github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/middleware"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/test/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockLogger is a mock implementation of the log.Logger interface
type MockLogger struct {
	mock.Mock
}

// Fields is a type alias for log.Fields
type Fields = map[string]any

// Debug implements log.Logger interface
func (m *MockLogger) Debug(args ...any) {
	m.Called(args...)
}

// Info implements log.Logger interface
func (m *MockLogger) Info(args ...any) {
	m.Called(args...)
}

// Warn implements log.Logger interface
func (m *MockLogger) Warn(args ...any) {
	m.Called(args...)
}

// Error implements log.Logger interface
func (m *MockLogger) Error(args ...any) {
	m.Called(args...)
}

// Fatal implements log.Logger interface
func (m *MockLogger) Fatal(args ...any) {
	m.Called(args...)
}

// Debugf implements log.Logger interface
func (m *MockLogger) Debugf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Infof implements log.Logger interface
func (m *MockLogger) Infof(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Warnf implements log.Logger interface
func (m *MockLogger) Warnf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Errorf implements log.Logger interface
func (m *MockLogger) Errorf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Fatalf implements log.Logger interface
func (m *MockLogger) Fatalf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// WithField implements log.Logger interface
func (m *MockLogger) WithField(key string, value any) *log.Logger {
	args := m.Called(key, value)
	return args.Get(0).(*log.Logger)
}

// WithFields implements log.Logger interface
func (m *MockLogger) WithFields(fields Fields) *log.Logger {
	args := m.Called(fields)
	return args.Get(0).(*log.Logger)
}

// WithError implements log.Logger interface
func (m *MockLogger) WithError(err error) *log.Logger {
	args := m.Called(err)
	return args.Get(0).(*log.Logger)
}

// WithContext implements log.Logger interface
func (m *MockLogger) WithContext(ctx context.Context) *log.Logger {
	args := m.Called(ctx)
	return args.Get(0).(*log.Logger)
}

// TestCase defines the structure for a license validation test case
type TestCase struct {
	Name              string
	SetupServer       func(*testing.T) *httptest.Server
	ExpectedValid     bool
	ExpectedGrace     bool
	ExpectedPanic     bool
	ExpectedDays      int
	ExpectedWarnLogs  int
	ExpectedErrorLogs int
}

// RunTestCases executes a list of test cases for license validation
func RunTestCases(t *testing.T, testCases []TestCase) {
	const (
		testAppID      = "test-app"
		testLicenseKey = "test-key"
		testOrgID      = "test-org"
	)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Setup test server
			ts := tc.SetupServer(t)
			defer ts.Close()

			// Set the required environment variables
			t.Setenv("MIDAZ_LICENSE_URL", ts.URL)
			t.Setenv(cn.EnvOrganizationIDs, testOrgID)

			// Create a mock logger
			logger := helper.NewMockLogger()
			mockLogger := helper.AsMock(logger)

			// Setup mock expectations based on test case
			if tc.ExpectedErrorLogs > 0 {
				mockLogger.On("Error", mock.Anything, mock.Anything, mock.Anything).Return()
			}
			if tc.ExpectedWarnLogs > 0 {
				mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Return()
			}
			if tc.ExpectedValid {
				mockLogger.On("Info", "License validation successful").Return()
			}

			// Support updated log formats for cache operations
			mockLogger.On("Infof", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return()
			mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.AnythingOfType("string"),
				mock.AnythingOfType("bool"), mock.AnythingOfType("int"), mock.AnythingOfType("bool"),
				mock.AnythingOfType("bool")).Return()

			// Create a new client with the mock logger and required parameters
			// Add test license key
			testLicenseKey := "test-license-key"
			client := middleware.NewLicenseClient(testAppID, testOrgID, testLicenseKey, logger)

			if tc.ExpectedPanic {
				assert.Panics(t, func() {
					_, _ = client.Validate(context.Background())
				})
			} else {
				result, err := client.Validate(context.Background())
				require.NoError(t, err)

				// Assert the validation result
				assert.Equal(t, tc.ExpectedValid, result.Valid)
				assert.Equal(t, tc.ExpectedDays, result.ExpiryDaysLeft)
			}

			// Assert that all expected mock calls were made
			mockLogger.AssertExpectations(t)
		})
	}
}

// jsonResponse creates an HTTP handler that responds with the given status code and JSON body
func jsonResponse(t *testing.T, statusCode int, body any) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	}
}

// validationResult creates a validation result for testing
func validationResult(valid bool, daysLeft int) *model.ValidationResult {
	return &model.ValidationResult{
		Valid:          valid,
		ExpiryDaysLeft: daysLeft,
	}
}
