package constant

// URLConstants defines license service endpoint URLs
const (
	// ProdLicenseGatewayBaseURL is the production license service URL
	ProdLicenseGatewayBaseURL = "https://license.lerian.io"
	// DevLicenseGatewayBaseURL is the development license service URL
	DevLicenseGatewayBaseURL = "https://license.dev.lerian.io"
)

// HeaderConstants defines HTTP header names used in requests
const (
	// OrganizationIDHeader defines the header name for organization ID in requests
	OrganizationIDHeader = "X-Organization-ID"
)

// TimeConstants defines timeout and interval values
const (
	// DefaultHTTPTimeoutSeconds is the default HTTP client timeout in seconds
	DefaultHTTPTimeoutSeconds = 5
	// DefaultRefreshIntervalDays is the default license refresh interval in days
	DefaultRefreshIntervalDays = 7
)

// ExpiryThresholds defines license expiration warning thresholds
const (
	// DefaultMinExpiryDaysToNormalWarn is the threshold for normal expiry warnings
	DefaultMinExpiryDaysToNormalWarn = 30
	// DefaultMinExpiryDaysToUrgentWarn is the threshold for urgent expiry warnings
	DefaultMinExpiryDaysToUrgentWarn = 7
	// DefaultGraceExpiryDaysToCriticalWarn is the threshold for critical grace period warnings
	DefaultGraceExpiryDaysToCriticalWarn = 7
	// DefaultTrialExpiryDaysToWarn is the threshold for trial license expiry warnings
	DefaultTrialExpiryDaysToWarn = 2
	// DefaultLicenseExpiredDays represents a license that expires today
	DefaultLicenseExpiredDays = 0
)
