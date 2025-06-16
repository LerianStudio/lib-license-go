package constant

// HeaderConstants defines HTTP header names used in requests
const (
	// OrganizationIDHeader defines the header name for organization ID in requests
	OrganizationIDHeader = "X-Organization-ID"
)

// TimeConstants defines timeout and interval values
const (
	// DefaultHTTPTimeoutSeconds is the default HTTP client timeout in seconds
	DefaultHTTPTimeoutSeconds = 5
	// DefaultRefreshIntervalHours is the default license refresh interval in hours
	DefaultRefreshIntervalHours = 2
)

// ExpiryThresholds defines license expiration warning thresholds
const (
	// DefaultMinExpiryHoursToNormalWarn is the threshold for normal expiry warnings
	DefaultMinExpiryHoursToNormalWarn = 3
	// DefaultMinExpiryHoursToUrgentWarn is the threshold for urgent expiry warnings
	DefaultMinExpiryHoursToUrgentWarn = 2
	// DefaultGraceExpiryHoursToCriticalWarn is the threshold for critical grace period warnings
	DefaultGraceExpiryHoursToCriticalWarn = 1
	// DefaultTrialExpiryHoursToWarn is the threshold for trial license expiry warnings
	DefaultTrialExpiryHoursToWarn = 2
	// DefaultLicenseExpiredHours represents a license that expires now
	DefaultLicenseExpiredHours = 0
)
