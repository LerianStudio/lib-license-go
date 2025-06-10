package constant

// License-related constants
const (
	// FallbackExpiryDaysLeft defines the number of days to use for fallback expiry
	// when the license server returns a 5xx error and no cached result is available
	FallbackExpiryDaysLeft = 7
)
