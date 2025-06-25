package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/shutdown"
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
func (c *LicenseClient) GetLicenseManagerShutdown() *shutdown.LicenseManagerShutdown {
	if c != nil && c.validator != nil {
		return c.validator.GetShutdownManager()
	}

	return nil
}

// startupValidation performs common validation steps for both HTTP and gRPC
func (c *LicenseClient) startupValidation() {
	c.initOnce.Do(func() {
		if c != nil && c.validator != nil {
			bgCtx := context.Background()
			if c.validator.IsGlobal {
				c.validateGlobalLicenseOnStartup(bgCtx)
			} else {
				c.validateMultiOrgLicensesOnStartup(bgCtx)
			}
			// Kick-off background refresh regardless of mode
			go c.validator.StartBackgroundRefresh(bgCtx)
		}
	})
}

// validateOrganizationID validates if the provided organization ID is valid
func (c *LicenseClient) validateOrganizationID(ctx context.Context, orgID string) (model.ValidationResult, error) {
	if orgID == "" {
		return model.ValidationResult{}, cn.ErrMissingOrgIDHeader
	}

	if !pkg.ContainsOrganizationID(c.validator.GetOrganizationIDs(), orgID) {
		return model.ValidationResult{}, cn.ErrUnknownOrgIDHeader
	}

	return c.validator.ValidateOrganizationWithCache(ctx, orgID)
}
