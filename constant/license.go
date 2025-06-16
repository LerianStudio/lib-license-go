package constant

// License-related constants
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

// FallbackExpiryDaysLeft defines the number of days to use for fallback expiry
// when the license server returns a 5xx error and no cached result is available
const FallbackExpiryDaysLeft = 7
