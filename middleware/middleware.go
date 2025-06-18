package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/shutdown"
	cn "github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/pkg"
	pkgHTTP "github.com/LerianStudio/lib-license-go/pkg/net/http"
	"github.com/LerianStudio/lib-license-go/validation"
	"github.com/gofiber/fiber/v2"
)

// LicenseClient is the public client API that exposes middleware functionality
// It's a wrapper around the internal validation client
type LicenseClient struct {
	validator *validation.Client
}

// NewLicenseClient creates a new license client with middleware capabilities
func NewLicenseClient(appID, licenseKey, orgIDs string, logger *log.Logger) *LicenseClient {
	// Create validation client (handles logger internally)
	validator, err := validation.New(appID, licenseKey, orgIDs, logger)
	if err != nil {
		return nil
	}

	return &LicenseClient{validator: validator}
}

// Middleware creates a Fiber middleware that validates the license and manages background refresh
func (c *LicenseClient) Middleware() fiber.Handler {
	// Perform startup validation
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

	// Return request handler
	return func(ctx *fiber.Ctx) error {
		if c == nil || c.validator == nil {
			return ctx.Next()
		}

		if c.validator.IsGlobal {
			return c.processGlobalPluginRequest(ctx)
		}

		return c.processMultiOrgPluginRequest(ctx)
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

// processGlobalPluginRequest handles requests in global plugin mode.
// Since validation happens at startup and through background refresh,
// we don't need to validate on each request for global mode.
func (c *LicenseClient) processGlobalPluginRequest(ctx *fiber.Ctx) error {
	// In global mode, we're already validated at startup and through background refresh
	// No need to validate on each request - just continue processing
	return ctx.Next()
}

// processMultiOrgPluginRequest validates license for org ID provided in header.
func (c *LicenseClient) processMultiOrgPluginRequest(ctx *fiber.Ctx) error {
	l := c.validator.GetLogger()

	orgID := ctx.Get(cn.OrganizationIDHeader)
	if orgID == "" {
		l.Errorf("Missing org header (code %s)", cn.ErrMissingOrgIDHeader.Error())

		return pkgHTTP.WithError(ctx, pkg.ValidateBusinessError(cn.ErrMissingOrgIDHeader, "", cn.OrganizationIDHeader))
	}

	if !pkg.ContainsOrganizationID(c.validator.GetOrganizationIDs(), orgID) {
		l.Errorf("Unknown org ID %s", orgID)

		return pkgHTTP.WithError(ctx, pkg.ValidateBusinessError(cn.ErrUnknownOrgIDHeader, "", orgID))
	}

	res, err := c.validator.ValidateOrganizationWithCache(ctx.Context(), orgID)
	if err != nil {
		l.Errorf("Validation failed for org %s: %v", orgID, err)

		return pkgHTTP.WithError(ctx, pkg.ValidateBusinessError(err, "", orgID))
	}

	if !res.Valid && !res.ActiveGracePeriod {
		l.Errorf("Org %s license invalid", orgID)

		return pkgHTTP.WithError(ctx, pkg.ValidateBusinessError(cn.ErrOrgLicenseInvalid, "", orgID))
	}

	return ctx.Next()
}

// TestValidate is a test function that validates the license
func (c *LicenseClient) TestValidate(ctx context.Context) (model.ValidationResult, error) {
	if c == nil || c.validator == nil {
		return model.ValidationResult{}, fiber.ErrInternalServerError
	}

	return c.validator.TestValidate(ctx)
}

// logLicenseStatus delegates license status logging to the validation client
// which has specialized logging functions for different license conditions
// The validation client already implements comprehensive license logging with specialized functions:
// - logTrialLicense for trial licenses
// - logValidLicense for valid non-trial licenses
// - logGracePeriod for licenses in grace period
// No need to duplicate logging logic here
// Only log errors for invalid licenses not in grace period when not in grace period and not trial
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
