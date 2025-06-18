package constant

import "errors"

// ErrorCodes defines structured error codes for license middleware responses
// Format: LCS-XXXX where XXXX is a unique 4-digit number
var (
	ErrInternalServer = errors.New("LCS-0001") // Internal server error

	// Global license validation errors (0002-0003)
	ErrNoOrganizationIDs = errors.New("LCS-0002") // No organization IDs configured
	ErrNoValidLicenses   = errors.New("LCS-0003") // No valid licenses found for any organization

	// Request-specific license validation errors (0010-0012)
	ErrMissingOrgIDHeader       = errors.New("LCS-0010") // Organization ID header is missing
	ErrUnknownOrgIDHeader       = errors.New("LCS-0011") // Organization ID header is unknown
	ErrOrgLicenseValidationFail = errors.New("LCS-0012") // Failed to validate organization license
	ErrOrgLicenseInvalid        = errors.New("LCS-0013") // Organization license is invalid
)
