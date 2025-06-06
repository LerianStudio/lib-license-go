package constant

// License URL constants
const (
	ProdLicenseGatewayBaseURL = "https://license.lerian.io"
	DevLicenseGatewayBaseURL  = "https://license.dev.lerian.io"
)

// OrganizationIDHeader defines the header name for organization ID in requests
const OrganizationIDHeader = "X-Organization-ID"

// Default values and thresholds
const (
	DefaultHTTPTimeoutSeconds            = 5
	DefaultRefreshIntervalDays           = 7
	DefaultMinExpiryDaysToNormalWarn     = 30
	DefaultMinExpiryDaysToUrgentWarn     = 7
	DefaultGraceExpiryDaysToCriticalWarn = 7
	DefaultTrialExpiryDaysToWarn         = 2
	DefaultLicenseExpiredDays            = 0
)
