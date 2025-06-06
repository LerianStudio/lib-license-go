package model

type Config struct {
	ApplicationName string `json:"applicationName"`
	LicenseKey      string `json:"licenseKey"`      // License key for API validation
	OrganizationIDs string `json:"organizationIds"` // Comma-separated list of organization IDs
}
