package config

import (
	"errors"
	"time"
)

// ClientConfig contains the configuration for the license client
type ClientConfig struct {
	AppName         string
	LicenseKey      string   // License key for API validation
	OrganizationIDs []string // List of valid organization IDs
	HTTPTimeout     time.Duration
	RefreshInterval time.Duration
}

// Validate checks if the configuration is valid
func (c *ClientConfig) Validate() error {
	if c.AppName == "" {
		return errors.New("application name is required")
	}
	if len(c.OrganizationIDs) == 0 {
		return errors.New("at least one organization ID is required")
	}

	return nil
}
