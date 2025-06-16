package constant

// License-related constants
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

// FallbackExpiryDaysLeft defines the number of days to use for fallback expiry
// when the license server returns a 5xx error and no cached result is available
const FallbackExpiryDaysLeft = 7
