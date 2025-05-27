package middleware

import (
	"strings"

	cn "github.com/LerianStudio/lib-license-go/constant"
)

// For testing purposes only - these functions should never be used in production

// SetTestLicenseBaseURL sets the base URL for license validation API in tests only
// This function should ONLY be used in tests and NEVER in production code
func SetTestLicenseBaseURL(url string) {
	baseURL = strings.TrimSuffix(url, "/")
}

// ResetTestLicenseBaseURL resets the base URL to its default value
// This function should be called after tests to ensure the URL is reset
func ResetTestLicenseBaseURL() {
	baseURL = cn.DefaultLicenseGatewayBaseURL
}
