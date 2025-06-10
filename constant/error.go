package constant

import "errors"

// ErrorCodes defines structured error codes for license middleware responses
// Format: LCS-XXXX where XXXX is a unique 4-digit number
var (
	// Global license validation errors (0001-0004)
	ErrGlobalLicenseValidationFailed = errors.New("LCS-0001") // Failed to validate global license
	ErrGlobalLicenseInvalid          = errors.New("LCS-0002") // Global license is invalid
	ErrNoOrganizationIDs             = errors.New("LCS-0003") // No organization IDs configured
	ErrNoValidLicenses               = errors.New("LCS-0004") // No valid licenses found for any organization

	// Request-specific license validation errors (0005-0009)
	ErrRequestGlobalLicenseValidationFail = errors.New("LCS-0005") // Failed to validate global license during request
	ErrRequestGlobalLicenseInvalid        = errors.New("LCS-0006") // Global license is invalid during request
	ErrMissingOrgIDHeader                 = errors.New("LCS-0007") // Organization ID header is missing
	ErrOrgLicenseValidationFail           = errors.New("LCS-0008") // Failed to validate organization license
	ErrOrgLicenseInvalid                  = errors.New("LCS-0009") // Organization license is invalid
)
