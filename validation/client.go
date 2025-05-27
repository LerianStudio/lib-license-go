package validation

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/zap"
	cn "github.com/LerianStudio/lib-license-go/constant"
	libErr "github.com/LerianStudio/lib-license-go/error"
	"github.com/LerianStudio/lib-license-go/internal/api"
	"github.com/LerianStudio/lib-license-go/internal/cache"
	"github.com/LerianStudio/lib-license-go/internal/config"
	"github.com/LerianStudio/lib-license-go/internal/refresh"
	"github.com/LerianStudio/lib-license-go/internal/shutdown"
	"github.com/LerianStudio/lib-license-go/model"
)

// Client handles license validation with caching and background refresh
type Client struct {
	config      *config.ClientConfig
	apiClient   *api.Client
	cacheManager *cache.Manager
	refreshManager *refresh.Manager
	shutdownManager *shutdown.Manager
	logger      log.Logger
}

// New creates a new license validation client
func New(appID, licenseKey, orgID, env string, logger *log.Logger) (*Client, error) {
	// Initialize logger
	var l log.Logger
	if logger != nil {
		l = *logger
	} else {
		l = zap.InitializeLogger()
	}

	// Create and validate config
	cfg := &config.ClientConfig{
		AppID:          appID,
		LicenseKey:     licenseKey,
		OrganizationID: orgID,
		Environment:    env,
		HTTPTimeout:    5 * time.Second,
		RefreshInterval: 7 * 24 * time.Hour,
	}
	
	if err := cfg.Validate(); err != nil {
		l.Errorf("Invalid configuration: %s", err.Error())
		return nil, err
	}
	
	// Generate fingerprint
	cfg.GenerateFingerprint()

	// Create cache manager
	cacheManager, err := cache.New(l)
	if err != nil {
		l.Errorf("Failed to initialize cache: %s", err.Error())
		return nil, err
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
	}
	
	// Create API client
	apiClient := api.New(cfg, httpClient, l)
	
	// Create shutdown manager
	shutdownManager := shutdown.New()
	
	// Create client
	client := &Client{
		config:      cfg,
		apiClient:   apiClient,
		cacheManager: cacheManager,
		shutdownManager: shutdownManager,
		logger:      l,
	}
	
	// Create and set up refresh manager
	refreshManager := refresh.New(client, cfg.RefreshInterval, l)
	client.refreshManager = refreshManager
	
	return client, nil
}

// SetHTTPClient allows overriding the HTTP client (useful for testing)
func (c *Client) SetHTTPClient(client *http.Client) {
	c.apiClient.SetHTTPClient(client)
}

// SetTerminationHandler allows customizing how the application terminates when license validation fails
func (c *Client) SetTerminationHandler(handler shutdown.Handler) {
	c.shutdownManager.SetHandler(handler)
}

// Validate checks if the license is valid with caching
func (c *Client) Validate(ctx context.Context) (model.ValidationResult, error) {
	// Check cache first
	if result, found := c.cacheManager.Get(c.config.Fingerprint); found {
		return result, nil
	}
	
	// Perform validation
	return c.validateAndHandle(ctx)
}

// validateAndHandle performs the validation and handles all error cases
func (c *Client) validateAndHandle(ctx context.Context) (model.ValidationResult, error) {
	// Call the license API
	res, err := c.apiClient.ValidateLicense(ctx)
	if err != nil {
		return c.handleAPIError(err)
	}
	
	// Check if license is valid
	if !res.Valid && !res.ActiveGracePeriod {
		c.logger.Errorf("Invalid license: neither active license nor grace period detected")
		c.refreshManager.Shutdown()
		// Terminate the application
		c.shutdownManager.Terminate("Invalid license state - no active license or grace period")
	}
	
	// License is valid or in grace period - process and log
	c.processValidResult(res)
	
	// Cache the result
	c.cacheManager.Store(c.config.Fingerprint, res)
	
	return res, nil
}

// handleAPIError handles all API error cases
func (c *Client) handleAPIError(err error) (model.ValidationResult, error) {
	// Handle ApiErrors specially
	if apiErr, ok := err.(*libErr.ApiError); ok {
		// Server errors (5xx) are treated as temporary and we fall back to cached value
		if apiErr.StatusCode >= 500 && apiErr.StatusCode < 600 {
			c.logger.Warnf("License server error (5xx) detected, treating as valid - error: %s", apiErr.Error())
			
			// Use cached result if available
			if cachedResult := c.cacheManager.GetLastResult(); cachedResult != nil {
				return *cachedResult, nil
			}
			
			// No cached result, return a temporary valid license
			return model.ValidationResult{
				Valid:             true,
				ExpiryDaysLeft:    cn.FallbackExpiryDaysLeft,
				ActiveGracePeriod: true,
			}, nil
		}
		
		// Client errors (4xx) are fatal - license is invalid
		if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			c.logger.Errorf("Exiting: license validation failed with client error: %s", apiErr.Error())
			c.refreshManager.Shutdown()
			
			// Terminate the application
			c.shutdownManager.Terminate("License validation failed with client error: " + apiErr.Error())
		}
	}
	
	// Handle connection errors by using the cached result if available
	if libErr.IsConnectionError(err) {
		if cachedResult := c.cacheManager.GetLastResult(); cachedResult != nil {
			c.logger.Warnf("Using cached license validation due to connection error - error: %s", err.Error())
			return *cachedResult, nil
		}
	}
	
	// For any other errors, just return the error
	return model.ValidationResult{}, fmt.Errorf("failed to validate license: %w", err)
}

// processValidResult handles a valid license response
func (c *Client) processValidResult(res model.ValidationResult) {
	// Log based on license state
	if res.Valid {
		if res.ExpiryDaysLeft <= 7 {
			// License valid and within 7 days of expiration - urgent warning
			c.logger.Warnf("WARNING: License expires in %d days. Contact your account manager to renew", res.ExpiryDaysLeft)
		} else if res.ExpiryDaysLeft <= 30 {
			// License valid but approaching expiration - normal warning
			c.logger.Warnf("License expires in %d days", res.ExpiryDaysLeft)
		}
		return
	}
	
	// License is in grace period
	if res.ActiveGracePeriod {
		if res.ExpiryDaysLeft <= 7 {
			// Grace period is about to expire
			c.logger.Warnf("CRITICAL: Grace period ends in %d days - application will terminate. Contact support immediately to renew license", res.ExpiryDaysLeft)
		} else {
			// License just expired, but is in grace period
			c.logger.Warnf("License expired! Running in grace period (%d days remaining)", res.ExpiryDaysLeft)
		}
	}
}

// StartBackgroundRefresh runs a ticker to refresh license periodically
func (c *Client) StartBackgroundRefresh(ctx context.Context) {
	c.refreshManager.Start(ctx)
}

// ShutdownBackgroundRefresh stops the background refresh process
func (c *Client) ShutdownBackgroundRefresh() {
	c.refreshManager.Shutdown()
}

// GetLogger returns the logger used by the client
func (c *Client) GetLogger() log.Logger {
	return c.logger
}

// ValidateWithRetry implements the refresh.Validator interface
func (c *Client) ValidateWithRetry(ctx context.Context) error {
	// Simple retry mechanism for background validation
	maxRetries := 3
	backoff := 5 * time.Second
	
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		_, err := c.validateAndHandle(ctx)
		if err == nil {
			return nil
		}
		
		lastErr = err
		c.logger.Warnf("Validation retry %d/%d failed: %v", i+1, maxRetries, err)
		
		// Wait before retrying, unless this is the last attempt
		if i < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}
	}
	
	return lastErr
}
