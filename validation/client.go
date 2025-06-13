package validation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/shutdown"
	"github.com/LerianStudio/lib-commons/commons/zap"
	cn "github.com/LerianStudio/lib-license-go/constant"
	libErr "github.com/LerianStudio/lib-license-go/error"
	"github.com/LerianStudio/lib-license-go/internal/api"
	"github.com/LerianStudio/lib-license-go/internal/cache"
	"github.com/LerianStudio/lib-license-go/internal/config"
	"github.com/LerianStudio/lib-license-go/internal/refresh"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/util"
)

// Client handles license validation with caching and background refresh
type Client struct {
	config          *config.ClientConfig
	apiClient       *api.Client
	cacheManager    *cache.Manager
	refreshManager  *refresh.Manager
	shutdownManager *shutdown.LicenseManagerShutdown
	logger          log.Logger
	// IsGlobal indicates if this client is running in global-plugin mode
	IsGlobal bool
}

// New creates a new license validation client
func New(appID, licenseKey, orgIDs string, logger *log.Logger) (*Client, error) {
	// Initialize logger
	var l log.Logger
	if logger != nil {
		l = *logger
	} else {
		l = zap.InitializeLogger()
	}

	// Parse organization IDs
	parsedOrgIDs := util.ParseOrganizationIDs(orgIDs)

	// Create and validate config
	cfg := &config.ClientConfig{
		AppName:         appID,
		LicenseKey:      licenseKey,
		OrganizationIDs: parsedOrgIDs,
		HTTPTimeout:     cn.DefaultHTTPTimeoutSeconds * time.Second,
		RefreshInterval: cn.DefaultRefreshIntervalDays * 24 * time.Hour,
	}

	if err := cfg.Validate(); err != nil {
		l.Errorf("Invalid configuration: %s", err.Error())
		return nil, err
	}

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

	// detect global plugin mode
	client.IsGlobal = len(parsedOrgIDs) == 1 && strings.EqualFold(parsedOrgIDs[0], cn.GlobalPluginValue)
	if client.IsGlobal {
		l.Infof("Validation client initialized in global plugin mode")
	}

	// Create and set up refresh manager
	refreshManager := refresh.New(client, cfg.RefreshInterval, l)
	client.refreshManager = refreshManager

	return client, nil
}

// Validate checks if the license is valid with caching for all configured organizations
func (c *Client) Validate(ctx context.Context) (model.ValidationResult, error) {
	// Perform initial validation for all organization IDs
	// This is used during application startup to ensure at least one organization has a valid license
	return c.validateAllOrganizations(ctx)
}

// ValidateWithOrgID validates a license for a specific organization ID with caching
func (c *Client) ValidateWithOrgID(ctx context.Context, orgID string) (model.ValidationResult, error) {
	// In global plugin mode, always use the global plugin ID
	validationOrgID := orgID
	if c.IsGlobal {
		c.logger.Debugf("Global plugin mode: using global ID for validation instead of %s", orgID)
		validationOrgID = cn.GlobalPluginValue
	}

	// Check if the organization ID is already in the cache
	if result, found := c.cacheManager.Get(validationOrgID); found {
		return result, nil
	}

	// Not in cache, perform validation
	return c.validateSingleOrganization(ctx, validationOrgID)
}

// validateAllOrganizations performs validation for all organization IDs
// At least one organization must have a valid license for the application to continue.
// This function collects all validation errors but will not terminate unless all validations fail.
func (c *Client) validateAllOrganizations(ctx context.Context) (model.ValidationResult, error) {
	// If no organization IDs are configured, return an error
	if len(c.config.OrganizationIDs) == 0 {
		c.logger.Error("No organization IDs configured")
		return model.ValidationResult{}, cn.ErrNoOrganizationIDs
	}

	// Special handling for global plugin mode
	if c.IsGlobal {
		c.logger.Debug("Validating in global plugin mode")
		result, err := c.validateSingleOrganization(ctx, cn.GlobalPluginValue)
		return result, err
	}

	// Track if at least one organization has a valid license
	var anyValid bool

	var lastValidResult model.ValidationResult
	// We'll collect all errors, but don't need lastErr since we're using allOrgErrors
	var allOrgErrors []error

	// Validate each organization ID
	for _, orgID := range c.config.OrganizationIDs {
		// Use performAPIValidation to validate just this organization ID directly
		// We bypass error handling to collect all errors before making a decision
		result, err := c.performAPIValidation(ctx, orgID)

		// Check if this is a server error (5xx) which should be treated as valid with grace period
		if apiErr, ok := err.(*libErr.APIError); ok && apiErr.StatusCode >= 500 && apiErr.StatusCode < 600 {
			c.logger.Warnf("License server error (5xx) detected for organization %s, treating as valid - error: %s",
				orgID, apiErr.Error())

			// Create a temporary valid license result
			tempResult := model.ValidationResult{
				Valid:             true,
				ExpiryDaysLeft:    cn.FallbackExpiryDaysLeft,
				ActiveGracePeriod: true,
			}

			// Store this result in cache
			c.cacheManager.Store(orgID, tempResult)

			// Mark as valid
			anyValid = true
			lastValidResult = tempResult

			continue
		}

		if err == nil && (result.Valid || result.ActiveGracePeriod || result.IsTrial) {
			// If this organization has a valid license, we're good
			anyValid = true
			lastValidResult = result
			c.logValidResult(result)
			c.cacheManager.Store(orgID, result)
		} else {
			if err != nil {
				allOrgErrors = append(allOrgErrors, fmt.Errorf("org %s: %w", orgID, err))

				// For APIErrors, we want to log appropriately but not terminate
				if apiErr, ok := err.(*libErr.APIError); ok {
					c.logger.Warnf("Organization %s license validation failed with status code %d: %v",
						orgID, apiErr.StatusCode, apiErr.Error())
				} else {
					c.logger.Warnf("Organization %s has invalid license: %v", orgID, err)
				}
			} else {
				c.logger.Warnf("Organization %s has invalid license status", orgID)
			}
		}
	}

	// If no valid organizations, terminate the application
	if !anyValid {
		c.refreshManager.Shutdown()
		c.logger.Error("No valid licenses found for any configured organization")

		// Construct a comprehensive error message with all validation errors
		var errMsg string

		if len(allOrgErrors) > 0 {
			errMsgs := make([]string, len(allOrgErrors))
			for i, err := range allOrgErrors {
				errMsgs[i] = err.Error()
			}

			errMsg = fmt.Sprintf("All license validations failed: [%s]", strings.Join(errMsgs, "; "))
		} else {
			errMsg = "No valid licenses found for any configured organization"
		}

		c.shutdownManager.Terminate(errMsg)

		return model.ValidationResult{}, errors.New(errMsg)
	}

	return lastValidResult, nil
}

// validateSingleOrganization performs validation for a specific organization ID
// and handles the result (caching, logging, error handling)
func (c *Client) validateSingleOrganization(ctx context.Context, orgID string) (model.ValidationResult, error) {
	// Validate for this single org
	result, err := c.performAPIValidation(ctx, orgID)
	if err != nil {
		// Handle errors according to type
		return c.handleAPIError(err)
	}

	// Successful validation
	c.logValidResult(result)
	c.cacheManager.Store(orgID, result)
	return result, nil
}

// performAPIValidation makes the actual API request to validate a license for a single organization ID
// This is a low-level function that only performs the API call without handling the result
func (c *Client) performAPIValidation(ctx context.Context, orgID string) (model.ValidationResult, error) {
	// Handle global plugin mode
	validationOrgID := orgID
	if c.IsGlobal {
		// In global plugin mode, always use the global plugin organization ID
		validationOrgID = cn.GlobalPluginValue
		c.logger.Debugf("Using global plugin ID for validation instead of %s", orgID)
	}

	// Create a temporary config with just this organization ID
	tempConfig := &config.ClientConfig{
		AppName:         c.config.AppName,
		LicenseKey:      c.config.LicenseKey,
		OrganizationIDs: []string{validationOrgID},
		HTTPTimeout:     c.config.HTTPTimeout,
		RefreshInterval: c.config.RefreshInterval,
	}

	// Create a temporary API client with the single org config
	tempClient := api.New(tempConfig, c.apiClient.GetHTTPClient(), c.logger)

	// Perform validation using the temporary client
	return tempClient.ValidateLicense(ctx)
}

// ValidateWithRetry implements refresh.Validator interface
// It attempts to validate the license with retries
func (c *Client) ValidateWithRetry(ctx context.Context) error {
	// Simple retry mechanism for background validation
	maxRetries := 3
	backoff := 5 * time.Second

	var lastErr error

	// Use cached flag instead of recomputing each retry
	isGlobalPlugin := c.IsGlobal

	for i := 0; i < maxRetries; i++ {
		// Create a timeout context for this validation attempt
		timeoutCtx, cancel := context.WithTimeout(ctx, c.config.HTTPTimeout)

		var err error

		// Perform the validation with the timeout context
		if isGlobalPlugin {
			// For global plugin, only validate with the global organization ID
			c.logger.Debug("Refreshing global plugin license validation")
			_, err = c.ValidateWithOrgID(timeoutCtx, cn.GlobalPluginValue)
		} else {
			// For regular multi-org mode, validate all organization IDs
			_, err = c.validateAllOrganizations(timeoutCtx)
		}

		// Always cancel the timeout context when done with this attempt
		cancel()

		if err == nil {
			return nil
		}

		// Check if the error was due to context timeout or cancellation
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			c.logger.Warnf("Validation attempt %d/%d timed out or was canceled", i+1, maxRetries)
		} else {
			c.logger.Warnf("Validation retry %d/%d failed: %v", i+1, maxRetries, err)
		}

		lastErr = err

		// Check if parent context is done before sleeping
		if ctx.Err() != nil {
			c.logger.Debug("Parent context canceled, stopping retry attempts")
			break
		}

		// Wait before retrying, unless this is the last attempt
		if i < maxRetries-1 {
			// Use a timer with context to allow for cancellation during backoff
			select {
			case <-time.After(backoff):
				// Continue with next retry
			case <-ctx.Done():
				c.logger.Debug("Context canceled during backoff, stopping retry attempts")
				return ctx.Err()
			}

			backoff *= 2 // Exponential backoff
		}
	}

	return lastErr
}

// handleAPIError handles all API error cases
// This is called for single organization validation (not from validateAndHandleAllOrgs)
func (c *Client) handleAPIError(err error) (model.ValidationResult, error) {
	// Handle APIErrors specially
	if apiErr, ok := err.(*libErr.APIError); ok {
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

		// Client errors (4xx) are fatal for single org validation - license is invalid
		// In multi-org validation, these errors are handled in validateAndHandleAllOrgs
		if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			// Check if we're in a multi-org validation process
			if len(c.config.OrganizationIDs) > 1 {
				// For multi-org validation, just return the error so the loop can continue
				return model.ValidationResult{}, apiErr
			}

			// For single org validation, terminate as before
			c.logger.Errorf("Exiting: license validation failed with client error: %s", apiErr.Error())
			c.refreshManager.Shutdown()
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
	// Get organization ID from the client config
	orgID := c.getOrgIDForLogging()

	// Handle different license states
	switch {
	case res.Valid && res.IsTrial:
		c.logTrialLicense(orgID, res.ExpiryDaysLeft)
	case res.Valid:
		c.logValidLicense(orgID, res.ExpiryDaysLeft)
	}

	// Handle grace period separately (can occur with valid licenses)
	if res.ActiveGracePeriod {
		c.logGracePeriod(orgID, res.ExpiryDaysLeft)
	}
}

// getOrgIDForLogging safely extracts the organization ID for logging purposes
func (c *Client) getOrgIDForLogging() string {
	if len(c.config.OrganizationIDs) > 0 {
		return c.config.OrganizationIDs[0]
	}

	return "unknown"
}

// logTrialLicense handles logging for trial licenses
func (c *Client) logTrialLicense(orgID string, expiryDays int) {
	messagePrefix := "TRIAL LICENSE"
	messageSuffix := "Please upgrade to a full license to continue using the application"

	switch {
	case expiryDays == cn.DefaultLicenseExpiredDays:
		// Trial license expires today
		c.logger.Warnf("%s: Organization %s trial expires today. %s", messagePrefix, orgID, messageSuffix)
	case expiryDays <= cn.DefaultTrialExpiryDaysToWarn:
		// Trial license is about to expire soon
		c.logger.Warnf("%s: Organization %s trial expires in %d days. %s", messagePrefix, orgID, expiryDays, messageSuffix)
	default:
		// General trial notice
		c.logger.Infof("%s: Organization %s is using a trial license that expires in %d days", messagePrefix, orgID, expiryDays)
	}
}

// logValidLicense handles logging for valid non-trial licenses
func (c *Client) logValidLicense(orgID string, expiryDays int) {
	switch {
	case expiryDays <= cn.DefaultMinExpiryDaysToUrgentWarn:
		// License valid and within urgent warning threshold
		c.logger.Warnf("WARNING: Organization %s license expires in %d days. Contact your account manager to renew", orgID, expiryDays)
	case expiryDays <= cn.DefaultMinExpiryDaysToNormalWarn:
		// License valid but approaching expiration
		c.logger.Warnf("Organization %s license expires in %d days", orgID, expiryDays)
	default:
		// General valid license message
		c.logger.Infof("Organization %s has a valid license", orgID)
	}
}

// logGracePeriod handles logging for licenses in grace period
func (c *Client) logGracePeriod(orgID string, expiryDays int) {
	if expiryDays <= cn.DefaultGraceExpiryDaysToCriticalWarn {
		// Grace period is about to expire
		c.logger.Warnf("CRITICAL: Organization %s grace period ends in %d days - application will terminate. Contact support immediately to renew license", orgID, expiryDays)
	} else {
		// General grace period warning
		c.logger.Warnf("WARNING: Organization %s license has expired but grace period is active for %d more days", orgID, expiryDays)
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

// GetOrganizationIDs returns the organization IDs configured for this client
func (c *Client) GetOrganizationIDs() []string {
	if c == nil || c.config == nil {
		return []string{}
	}

	return c.config.OrganizationIDs
}

// SetHTTPClient allows overriding the HTTP client (useful for testing)
func (c *Client) SetHTTPClient(client *http.Client) {
	c.apiClient.SetHTTPClient(client)
}

// GetShutdownManager returns the license shutdown manager
func (c *Client) GetShutdownManager() *shutdown.LicenseManagerShutdown {
	return c.shutdownManager
}

// SetTerminationHandler allows customizing how the application terminates when license validation fails
func (c *Client) SetTerminationHandler(handler shutdown.Handler) {
	c.shutdownManager.SetHandler(handler)
}
