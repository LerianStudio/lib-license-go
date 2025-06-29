package validation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	libLicense "github.com/LerianStudio/lib-commons/commons/license"
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/zap"
	cn "github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/internal/api"
	"github.com/LerianStudio/lib-license-go/internal/cache"
	"github.com/LerianStudio/lib-license-go/internal/config"
	"github.com/LerianStudio/lib-license-go/internal/refresh"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/pkg"
	pkgHTTP "github.com/LerianStudio/lib-license-go/pkg/net/http"
)

// Client handles license validation with caching and background refresh
type Client struct {
	config          *config.ClientConfig
	apiClient       *api.Client
	cacheManager    *cache.Manager
	refreshManager  *refresh.Manager
	shutdownManager *libLicense.ManagerShutdown
	logger          log.Logger
	// IsGlobal indicates if this client is running in global-plugin mode
	IsGlobal bool
}

// This method has been moved to the end of the file

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
	parsedOrgIDs := pkg.ParseOrganizationIDs(orgIDs)

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
	shutdownManager := libLicense.New()

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
		l.Debugf("Validation client initialized in global plugin mode")
	}

	// Create and set up refresh manager
	refreshManager := refresh.New(client, cfg.RefreshInterval, l)
	client.refreshManager = refreshManager

	return client, nil
}

// TestValidate is a test function that checks if the license is valid with caching for all configured organizations
func (c *Client) TestValidate(ctx context.Context) (model.ValidationResult, error) {
	// Perform initial validation for all organization IDs
	// This is used during application startup to ensure at least one organization has a valid license
	return c.ValidateAllOrganizations(ctx)
}

// ValidateOrganizationWithCache validates a license for a specific organization ID with caching
func (c *Client) ValidateOrganizationWithCache(ctx context.Context, orgID string) (model.ValidationResult, error) {
	// Check if the organization ID is already in the cache
	if result, found := c.cacheManager.Get(orgID); found {
		return result, nil
	}

	// Not in cache, perform validation
	return c.validateSingleOrganization(ctx, orgID)
}

// ValidateAllOrganizations performs validation for all organization IDs
// At least one organization must have a valid license for the application to continue.
// This function collects all validation errors but will not terminate unless all validations fail.
func (c *Client) ValidateAllOrganizations(ctx context.Context) (model.ValidationResult, error) {
	// If no organization IDs are configured, return an error
	if len(c.config.OrganizationIDs) == 0 {
		return model.ValidationResult{}, cn.ErrNoOrganizationIDs
	}

	// Special handling for global plugin mode
	if c.IsGlobal {
		return c.validateSingleOrganization(ctx, cn.GlobalPluginValue)
	}

	// Process all organization IDs at once
	return c.validateMultipleOrganizations(ctx, c.config.OrganizationIDs)
}

// validateMultipleOrganizations performs validation for multiple organization IDs
func (c *Client) validateMultipleOrganizations(ctx context.Context, orgIDs []string) (model.ValidationResult, error) {
	var validFound bool

	var lastValidResult model.ValidationResult

	var allOrgErrors []error

	// Process each organization ID to validate licenses
	// We still need to iterate through orgIDs for proper error handling and caching
	// but we're no longer creating temporary API clients for each org
	for _, orgID := range orgIDs {
		// Use the API client directly with this organization ID
		result, err := c.apiClient.ValidateOrganization(ctx, orgID)

		// Check if this is a server error (5xx) which should be treated as valid with grace period
		if apiErr, ok := err.(*pkg.HTTPError); ok && apiErr.StatusCode >= 500 && apiErr.StatusCode < 600 {
			c.logger.Debugf("License server error (5xx) detected for organization %s, treating as valid - error: %s",
				orgID, apiErr.Error())

			// Create a temporary valid license result
			tempResult := model.ValidationResult{
				Valid:             true,
				ExpiryDaysLeft:    cn.FallbackExpiryDaysLeft,
				ActiveGracePeriod: true,
			}

			// Mark as valid
			validFound = true
			lastValidResult = tempResult

			continue
		}

		if err == nil && (result.Valid || result.ActiveGracePeriod) {
			// If this organization has a valid license, we're good
			validFound = true
			lastValidResult = result

			c.logValidResult(orgID, result)
			c.cacheManager.Store(orgID, result)
		} else {
			if err != nil {
				allOrgErrors = append(allOrgErrors, fmt.Errorf("org %s: %w", orgID, err))

				c.logger.Warnf("Validation failed for org %s", orgID)
				c.logger.Debugf("error: %v", err)

				// For APIErrors, we want to log appropriately but not terminate
				if apiErr, ok := err.(*pkg.HTTPError); ok {
					c.logger.Debugf("Organization %s license validation failed with status code %d: %v",
						orgID, apiErr.StatusCode, apiErr.Error())
				} else {
					c.logger.Debugf("Organization %s has invalid license: %v", orgID, err)
				}
			}
		}
	}

	// If no valid organizations, terminate the application
	if !validFound {
		var orgIDsErrorMsgs string

		baseErrMsg := "All license validations failed"

		if len(allOrgErrors) > 0 {
			errMsgs := make([]string, len(allOrgErrors))
			for i, err := range allOrgErrors {
				errMsgs[i] = err.Error()
			}

			orgIDsErrorMsgs = fmt.Sprintf("[%s]", strings.Join(errMsgs, "; "))
		} else {
			orgIDsErrorMsgs = fmt.Sprintf("%s: No valid licenses found for any configured organization", baseErrMsg)
		}

		c.logger.Errorf("Exiting: %s: %s", cn.ErrNoValidLicenses.Error(), baseErrMsg)
		c.logger.Debugf("Org IDs error: %s", orgIDsErrorMsgs)
		panic(fmt.Sprintf("%s: %s", cn.ErrNoValidLicenses.Error(), orgIDsErrorMsgs))
	}

	return lastValidResult, nil
}

// validateSingleOrganization performs validation for a specific organization ID
// and handles the result (caching, logging, error handling)
func (c *Client) validateSingleOrganization(ctx context.Context, orgID string) (model.ValidationResult, error) {
	// Validate for this single org using the variadic performAPIValidation
	result, err := c.apiClient.ValidateOrganization(ctx, orgID)
	if err != nil {
		// Handle errors according to type
		return c.handleAPIError(orgID, err)
	}

	errMsg := "No valid licenses found"

	// Check if the license is valid or in grace period
	if !result.Valid && !result.ActiveGracePeriod {
		// License is expired and not in grace period
		c.logger.Errorf("Exiting: %s", errMsg)
		panic(fmt.Sprintf("%s: %s", cn.ErrNoValidLicenses.Error(), errMsg))
	}

	// Successful validation
	c.logValidResult(orgID, result)
	c.cacheManager.Store(orgID, result)

	return result, nil
}

// ValidateWithRetry implements refresh.Validator interface
// It attempts to validate the license with retries
func (c *Client) ValidateWithRetry(ctx context.Context) error {
	// Simple retry mechanism for background validation
	maxRetries := 3
	backoff := 5 * time.Second

	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// Create a timeout context for this validation attempt
		timeoutCtx, cancel := context.WithTimeout(ctx, c.config.HTTPTimeout)

		var err error

		_, err = c.ValidateAllOrganizations(timeoutCtx)

		// Always cancel the timeout context when done with this attempt
		cancel()

		if err == nil {
			return nil
		}

		// Check if the error was due to context timeout or cancellation
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			c.logger.Debugf("Validation attempt %d/%d timed out or was canceled", i+1, maxRetries)
		} else {
			c.logger.Debugf("Validation retry %d/%d failed: %v", i+1, maxRetries, err)
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
func (c *Client) handleAPIError(orgID string, err error) (model.ValidationResult, error) {
	// Handle APIErrors specially
	if apiErr, ok := err.(*pkg.HTTPError); ok {
		// Server errors (5xx) are treated as temporary and we fall back to cached value
		if apiErr.StatusCode >= 500 && apiErr.StatusCode < 600 {
			c.logger.Debugf("License server error (5xx) detected, treating as valid - error: %s", apiErr.Error())

			// Try to get any cached result for this org ID
			if result, found := c.cacheManager.Get(orgID); found {
				c.logger.Debugf("Using cached license validation for org %s due to server error", orgID)
				return result, nil
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
			if orgID != cn.GlobalPluginValue {
				// For multi-org validation, just return the error so the loop can continue
				return model.ValidationResult{}, pkg.ForbiddenError{
					Code:    apiErr.Code,
					Title:   apiErr.Title,
					Message: apiErr.Message,
				}
			}

			// For single org validation, terminate as before
			c.logger.Errorf("Exiting: license validation failed with client error: %s", apiErr.Error())
			panic(fmt.Sprintf("License validation failed with client error: %s", apiErr.Error()))
		}
	}

	// Handle connection errors by using the cached result if available
	if pkgHTTP.IsConnectionError(err) {
		if result, found := c.cacheManager.Get(orgID); found {
			c.logger.Debugf("Using cached license validation for org %s due to connection error: %s", orgID, err.Error())
			return result, nil
		}
	}

	// For any other errors, just return the error
	return model.ValidationResult{}, cn.ErrOrgLicenseValidationFail
}

// logValidResult handles a valid license response
func (c *Client) logValidResult(orgID string, res model.ValidationResult) {
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

// logTrialLicense handles logging for trial licenses
func (c *Client) logTrialLicense(orgID string, expiryDays int) {
	messagePrefix := fmt.Sprintf("TRIAL LICENSE: Organization %s", orgID)
	messageSuffix := "Please upgrade to a full license to continue using the application"

	if c.IsGlobal {
		messagePrefix = "TRIAL LICENSE: Application"
	}

	switch {
	case expiryDays == cn.DefaultLicenseExpiredDays:
		// Trial license expires today
		c.logger.Warnf("%s trial expires today. %s", messagePrefix, messageSuffix)
	case expiryDays <= cn.DefaultTrialExpiryDaysToWarn:
		// Trial license is about to expire soon
		c.logger.Warnf("%s trial expires in %d days. %s", messagePrefix, expiryDays, messageSuffix)
	default:
		// General trial notice
		c.logger.Infof("%s is using a trial license that expires in %d days", messagePrefix, expiryDays)
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
func (c *Client) GetShutdownManager() *libLicense.ManagerShutdown {
	return c.shutdownManager
}

// SetTerminationHandler allows customizing how the application terminates when license validation fails
func (c *Client) SetTerminationHandler(handler libLicense.Handler) {
	c.shutdownManager.SetHandler(handler)
}
