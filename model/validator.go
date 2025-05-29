package model

// ValidationResult contains the data returned by license validation.
type ValidationResult struct {
	Valid             bool `json:"valid"`
	ExpiryDaysLeft    int  `json:"expiryDaysLeft,omitempty"`
	ActiveGracePeriod bool `json:"activeGracePeriod,omitempty"`
	IsTrial           bool `json:"isTrial,omitempty"`
}

// ErrorResponse contains error information returned by the license API
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
