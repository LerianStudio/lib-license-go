package config

import (
	"errors"
	"time"

	"github.com/LerianStudio/lib-commons/commons"
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-license-go/model"
)

// ClientConfig holds the configuration for the license client
type ClientConfig struct {
	AppName        string // Application name (e.g., "plugin-fees")
	LicenseKey     string
	OrganizationID string
	Environment    string
	Fingerprint    string

	// HTTP configuration
	HTTPTimeout time.Duration

	// Background refresh configuration
	RefreshInterval time.Duration
}

// NewDefaultConfig creates a new config with sensible defaults
func NewDefaultConfig() ClientConfig {
	return ClientConfig{
		HTTPTimeout:     5 * time.Second,
		RefreshInterval: 7 * 24 * time.Hour,
	}
}

// Validate checks if the configuration is valid
func (c *ClientConfig) Validate() error {
	if c.AppName == "" {
		return errors.New("application name is required")
	}
	if c.LicenseKey == "" {
		return errors.New("license key is required")
	}
	if c.Environment == "" {
		return errors.New("environment is required")
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

// FromModel converts a model.Config to a ClientConfig
func FromModel(cfg model.Config, logger log.Logger) (*ClientConfig, error) {
	// Convert model.Config to ClientConfig
	config := &ClientConfig{
		AppName:         cfg.ApplicationName,
		LicenseKey:      cfg.LicenseKey,
		OrganizationID:  cfg.OrganizationID,
		Environment:     cfg.PluginEnvironment,
		HTTPTimeout:     5 * time.Second,
		RefreshInterval: 7 * 24 * time.Hour,
	}

	// Validate the converted config
	if err := config.Validate(); err != nil {
		return nil, err
	}

	config.GenerateFingerprint()
	return config, nil
}
