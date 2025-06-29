package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	libLicense "github.com/LerianStudio/lib-commons/commons/license"
	"github.com/LerianStudio/lib-commons/commons/log"
	cn "github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/pkg"
	"github.com/LerianStudio/lib-license-go/validation"
)

// LicenseClient is the public client API that exposes middleware functionality
// It's a wrapper around the internal validation client
type LicenseClient struct {
	validator *validation.Client
	// initOnce ensures startup validation and background refresh happen only once
	// even when both HTTP middleware and gRPC interceptors are used
	initOnce sync.Once
}

// ValidateInitialization checks if the client is correctly initialized.
// It panics with a descriptive error message if the client or its validator is nil.
// This is used internally by methods that require a properly initialized client.
func (c *LicenseClient) ValidateInitialization(operation string) {
	if c == nil {
		panic(fmt.Sprintf("LicenseClient is nil, cannot %s. Review your configuration.", operation))
	}

	if c.validator == nil {
		panic(fmt.Sprintf("LicenseClient.validator is nil, cannot %s. Review your configuration.", operation))
	}
}

// validateClientInitialization checks if the client is correctly initialized.
// It returns an error for validator-related issues but panics if the client itself is nil.
// This provides a balance between fail-fast for critical configuration errors
// and graceful error handling for runtime issues.
func (c *LicenseClient) validateClientInitialization(operation string) error {
	// Critical error: nil client should always panic
	if c == nil {
		panic(fmt.Sprintf("LicenseClient is nil, cannot %s. Review your configuration.", operation))
	}

	// Non-critical error: nil validator returns an error
	if c.validator == nil {
		return fmt.Errorf("LicenseClient.validator is nil, cannot %s", operation)
	}

	return nil
}

// NewLicenseClient creates a new license client with middleware capabilities
func NewLicenseClient(appID, licenseKey, orgIDs string, logger *log.Logger) *LicenseClient {
	// Create validation client (handles logger internally)
	validator, err := validation.New(appID, licenseKey, orgIDs, logger)
	if err != nil {
		return nil
	}

	return &LicenseClient{
		validator: validator,
	}
}

// validateGlobalLicenseOnStartup validates the global license during application start.
// Panics if the license is not valid so that the app never starts with an invalid license.
func (c *LicenseClient) validateGlobalLicenseOnStartup(ctx context.Context) {
	l := c.validator.GetLogger()

	result, err := c.validator.ValidateOrganizationWithCache(ctx, cn.GlobalPluginValue)
	if err != nil {
		l.Errorf("License validation failed: %v", err)
		panic(fmt.Sprintf("License validation failed: %s", err.Error()))
	}

	if !result.Valid && !result.ActiveGracePeriod {
		l.Error("License is invalid")
		panic("No valid license found")
	}

	c.logLicenseStatus(result, cn.GlobalPluginValue)
}

// validateMultiOrgLicensesOnStartup validates each configured organization licence at startup.
// Panics if no valid licences are found so that the app never starts unlicensed.
func (c *LicenseClient) validateMultiOrgLicensesOnStartup(ctx context.Context) {
	l := c.validator.GetLogger()

	_, err := c.validator.ValidateAllOrganizations(ctx)
	if err != nil {
		l.Debugf("error in validateMultiOrgLicensesOnStartup: %v", err)
		panic("No valid licenses found for any organization")
	}
}

// logLicenseStatus delegates license status logging to the validation client
func (c *LicenseClient) logLicenseStatus(res model.ValidationResult, orgID string) {
	if !res.Valid && !res.ActiveGracePeriod {
		l := c.validator.GetLogger()

		if res.IsTrial {
			l.Errorf("LICENSE TRIAL: Organization %s has a expired trial license - application access will be denied", orgID)

			return
		}

		l.Errorf("LICENSE INVALID: Organization %s has no valid license - application access will be denied", orgID)
	}
}

// TestValidate is a test function that validates the license
func (c *LicenseClient) TestValidate(ctx context.Context) (model.ValidationResult, error) {
	if c == nil || c.validator == nil {
		return model.ValidationResult{}, fmt.Errorf("license client or validator is nil")
	}

	return c.validator.TestValidate(ctx)
}

// SetHTTPClient allows overriding the HTTP client (useful for testing)
func (c *LicenseClient) SetHTTPClient(client *http.Client) {
	if c != nil && c.validator != nil {
		c.validator.SetHTTPClient(client)
	}
}

// SetTerminationHandler allows customizing how the application terminates when license validation fails
func (c *LicenseClient) SetTerminationHandler(handler func(reason string)) {
	if c != nil && c.validator != nil {
		c.validator.SetTerminationHandler(handler)
	}
}

// ShutdownBackgroundRefresh stops the background refresh process
func (c *LicenseClient) ShutdownBackgroundRefresh() {
	if c != nil && c.validator != nil {
		c.validator.ShutdownBackgroundRefresh()
	}
}

// GetLogger returns the logger used by the client
func (c *LicenseClient) GetLogger() log.Logger {
	return c.validator.GetLogger()
}

// GetLicenseManagerShutdown returns the shutdown manager from the validation client
func (c *LicenseClient) GetLicenseManagerShutdown() *libLicense.ManagerShutdown {
	if c != nil && c.validator != nil {
		return c.validator.GetShutdownManager()
	}

	return nil
}

// startupValidation performs license validation at application startup and initializes background refresh.
// It is safe to call multiple times as it uses sync.Once internally to ensure validation happens only once.
// Panics if the client is nil or misconfigured to prevent running without license validation.
func (c *LicenseClient) startupValidation() {
	// Validate client initialization before entering the once block
	// This prevents silently skipping validation on misconfigured clients
	c.ValidateInitialization("perform startup validation")

	c.initOnce.Do(func() {
		bgCtx := context.Background()
		if c.validator.IsGlobal {
			c.validateGlobalLicenseOnStartup(bgCtx)
		} else {
			c.validateMultiOrgLicensesOnStartup(bgCtx)
		}
		// Kick-off background refresh regardless of mode
		go c.validator.StartBackgroundRefresh(bgCtx)
	})
}

// validateOrganizationID validates if the provided organization ID is valid
func (c *LicenseClient) validateOrganizationID(ctx context.Context, orgID string) (model.ValidationResult, error) {
	// Check for proper client initialization
	if err := c.validateClientInitialization("validate organization ID"); err != nil {
		return model.ValidationResult{}, err
	}

	if orgID == "" {
		return model.ValidationResult{}, cn.ErrMissingOrgIDHeader
	}

	if !pkg.ContainsOrganizationID(c.validator.GetOrganizationIDs(), orgID) {
		return model.ValidationResult{}, cn.ErrUnknownOrgIDHeader
	}

	return c.validator.ValidateOrganizationWithCache(ctx, orgID)
}
