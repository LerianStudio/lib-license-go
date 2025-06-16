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
