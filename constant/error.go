package constant

// Structured error codes for license middleware responses
const (
	ErrGlobalLicenseValidationFailed      = "LCS-0001"
	ErrGlobalLicenseInvalid               = "LCS-0002"
	ErrNoOrganizationIDs                  = "LCS-0003"
	ErrNoValidLicenses                    = "LCS-0004"
	ErrRequestGlobalLicenseValidationFail = "LCS-0005"
	ErrRequestGlobalLicenseInvalid        = "LCS-0006"
	ErrMissingOrgIDHeader                 = "LCS-0007"
	ErrOrgLicenseValidationFail           = "LCS-0008"
	ErrOrgLicenseInvalid                  = "LCS-0009"
)
