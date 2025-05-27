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
	// Application identification
	AppID          string
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
	if c.AppID == "" {
		return errors.New("application ID is required")
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
	fp := c.AppID + ":"

	if c.OrganizationID != "" {
		fp = fp + commons.HashSHA256(c.LicenseKey+":"+c.OrganizationID)
	} else {
		fp = fp + commons.HashSHA256(c.LicenseKey)
	}

	c.Fingerprint = fp
}

// FromModel converts a model.Config to a ClientConfig
func FromModel(cfg model.Config, logger log.Logger) (*ClientConfig, error) {
	if err := validateModelConfig(&cfg, logger); err != nil {
		return nil, err
	}

	config := &ClientConfig{
		AppID:           cfg.ApplicationName,
		LicenseKey:      cfg.LicenseKey,
		OrganizationID:  cfg.OrganizationID,
		Environment:     cfg.PluginEnvironment,
		HTTPTimeout:     5 * time.Second,
		RefreshInterval: 7 * 24 * time.Hour,
	}

	config.GenerateFingerprint()
	return config, nil
}

// validateModelConfig validates the model.Config
func validateModelConfig(cfg *model.Config, logger log.Logger) error {
	if cfg.ApplicationName == "" {
		return errors.New("application name is required")
	}
	if cfg.LicenseKey == "" {
		return errors.New("license key is required")
	}
	if cfg.PluginEnvironment == "" {
		return errors.New("environment is required")
	}
	return nil
}
