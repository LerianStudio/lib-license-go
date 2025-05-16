package model

// ValidationResult contains the data returned by license validation.
type ValidationResult struct {
	Valid             bool `json:"valid"`
	ExpiryDaysLeft    int  `json:"expiryDaysLeft,omitempty"`
	ActiveGracePeriod bool `json:"activeGracePeriod,omitempty"`
}
