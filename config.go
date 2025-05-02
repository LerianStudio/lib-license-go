package sdk

import (
	"os"
)

// Config holds runtime configuration for the validator.
type Config struct {
	LicenseKey      string
	ApplicationName string
	OrgID           string
	APIGatewayURL   string

	fingerprint string
}

// LoadFromEnv builds the Config, including generating a fingerprint based on
// the application name and org ID.
func LoadFromEnv() Config {
	cfg := Config{
		LicenseKey:      os.Getenv("LICENSE_KEY"),
		ApplicationName: os.Getenv("APPLICATION_NAME"),
		APIGatewayURL:   os.Getenv("LERIAN_API_GATEWAY_URL"),
	}

	return cfg
}
