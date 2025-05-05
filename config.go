package sdk

import (
	"os"
)

// Config holds runtime configuration for the validator.
type Config struct {
	ApplicationName string
	LicenseKey      string
	OrganizationID  string
	APIGatewayURL   string

	fingerprint string
}

// LoadFromEnv builds the Config, including generating a fingerprint based on
// the license key and org ID.
func LoadFromEnv() Config {
	cfg := Config{
		ApplicationName: os.Getenv("APPLICATION_NAME"),
		LicenseKey:      os.Getenv("LICENSE_KEY"),
		OrganizationID:  os.Getenv("MIDAZ_ORGANIZATION_ID"),
		APIGatewayURL:   os.Getenv("LERIAN_API_GATEWAY_URL"),
	}

	return cfg
}
