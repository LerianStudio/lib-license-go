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
	config          *config.ClientConfig
	apiClient       *api.Client
	cacheManager    *cache.Manager
	refreshManager  *refresh.Manager
	shutdownManager *shutdown.Manager
	logger          log.Logger
}

// New creates a new license validation client
func New(appID, licenseKey, orgID string, logger *log.Logger) (*Client, error) {
	// Initialize logger
	var l log.Logger
	if logger != nil {
		l = *logger
	} else {
		l = zap.InitializeLogger()
	}

	// Create and validate config
	cfg := &config.ClientConfig{
		AppName:         appID,
		LicenseKey:      licenseKey,
		OrganizationID:  orgID,
		HTTPTimeout:     cn.DefaultHTTPTimeoutSeconds * time.Second,
		RefreshInterval: cn.DefaultRefreshIntervalDays * 24 * time.Hour,
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
		config:          cfg,
		apiClient:       apiClient,
		cacheManager:    cacheManager,
		shutdownManager: shutdownManager,
		logger:          l,
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
	res, err := c.apiClient.ValidateLicense(ctx)
	if err != nil {
		return c.handleAPIError(err)
	}

	c.logValidResult(res)
	c.cacheManager.Store(c.config.Fingerprint, res)

	if !res.Valid && !res.ActiveGracePeriod {
		c.refreshManager.Shutdown()

		if res.IsTrial {
			c.shutdownManager.Terminate("Thank you for trying our application. Your trial period has now ended. Please purchase a subscription to continue enjoying our services.")
		} else {
			c.logger.Errorf("Invalid license: neither active license nor grace period detected")
			c.shutdownManager.Terminate("Invalid license state - no active license or grace period")
		}
	}

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

// logValidResult handles a valid license response
func (c *Client) logValidResult(res model.ValidationResult) {
	if res.Valid {
		// Handle trial license
		if res.IsTrial {
			messagePrefix := "TRIAL LICENSE"
			messageSuffix := "Please upgrade to a full license to continue using the application"

			// Handle active trial license
			if res.ExpiryDaysLeft == cn.DefaultLicenseExpiredDays {
				// Trial license expires today
				c.logger.Warnf("%s: Your trial expires today. %s", messagePrefix, messageSuffix)
			} else if res.ExpiryDaysLeft <= cn.DefaultTrialExpiryDaysToWarn {
				// Trial license is about to expire soon
				c.logger.Warnf("%s: Your trial expires in %d days. %s", messagePrefix, res.ExpiryDaysLeft, messageSuffix)
			} else {
				// General trial notice
				c.logger.Infof("%s: You are using a trial license that expires in %d days", messagePrefix, res.ExpiryDaysLeft)
			}
			return
		}

		// Log based on license state for non-trial licenses

		if res.ExpiryDaysLeft <= cn.DefaultMinExpiryDaysToUrgentWarn {
			// License valid and within 7 days of expiration - urgent warning
			c.logger.Warnf("WARNING: License expires in %d days. Contact your account manager to renew", res.ExpiryDaysLeft)
		} else if res.ExpiryDaysLeft <= cn.DefaultMinExpiryDaysToNormalWarn {
			// License valid but approaching expiration - normal warning
			c.logger.Warnf("License expires in %d days", res.ExpiryDaysLeft)
		}
	}

	// License is in grace period
	if res.ActiveGracePeriod {
		if res.ExpiryDaysLeft <= cn.DefaultGraceExpiryDaysToCriticalWarn {
			// Grace period is about to expire
			c.logger.Warnf("CRITICAL: Grace period ends in %d days - application will terminate. Contact support immediately to renew license", res.ExpiryDaysLeft)
		} else {
			// General grace period warning
			c.logger.Warnf("WARNING: License has expired but grace period is active for %d more days", res.ExpiryDaysLeft)
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
