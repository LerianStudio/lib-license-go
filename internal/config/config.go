package config

import (
	"errors"
	"time"

	"github.com/LerianStudio/lib-commons/commons"
)

// ClientConfig holds the configuration for the license client
type ClientConfig struct {
	AppName        string // Application name (e.g., "plugin-fees")
	LicenseKey     string
	OrganizationID string
	Fingerprint    string

	// HTTP configuration
	HTTPTimeout time.Duration

	// Background refresh configuration
	RefreshInterval time.Duration
}

// Validate checks if the configuration is valid
func (c *ClientConfig) Validate() error {
	if c.AppName == "" {
		return errors.New("application name is required")
	}
	if c.LicenseKey == "" {
		return errors.New("license key is required")
	}
	if c.OrganizationID == "" {
		return errors.New("organization ID is required")
	}
	return nil
}

// GenerateFingerprint creates a unique fingerprint for this license
func (c *ClientConfig) GenerateFingerprint() {
	fp := c.AppName + ":"

	if c.OrganizationID != "" {
		fp = fp + commons.HashSHA256(c.LicenseKey+":"+c.OrganizationID)
	} else {
		fp = fp + commons.HashSHA256(c.LicenseKey)
	}

	c.Fingerprint = fp
}
