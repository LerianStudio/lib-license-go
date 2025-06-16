package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/shutdown"
	"github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/util"
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

	result, err := c.validator.ValidateWithOrgID(ctx, constant.GlobalPluginValue)
	if err != nil {
		l.Errorf("License validation failed: %v (code %s)", err, constant.ErrGlobalLicenseValidationFailed.Error())
		panic(fmt.Sprintf("%s: %s", constant.ErrGlobalLicenseValidationFailed.Error(), "License validation failed"))
	}

	if !result.Valid && !result.ActiveGracePeriod {
		l.Errorf("License is invalid (code %s)", constant.ErrGlobalLicenseInvalid.Error())
		panic(fmt.Sprintf("%s: %s", constant.ErrGlobalLicenseInvalid.Error(), "License is invalid"))
	}

	c.logLicenseStatus(result, constant.GlobalPluginValue)
}

// validateMultiOrgLicensesOnStartup validates each configured organization licence at startup.
// Panics if no valid licences are found so that the app never starts unlicensed.
func (c *LicenseClient) validateMultiOrgLicensesOnStartup(ctx context.Context) {
	l := c.validator.GetLogger()

	orgIDs := c.validator.GetOrganizationIDs()

	if len(orgIDs) == 0 {
		l.Errorf("No organization IDs configured (code %s)", constant.ErrNoOrganizationIDs.Error())
		panic(fmt.Sprintf("%s: %s", constant.ErrNoOrganizationIDs.Error(), "No organization IDs configured"))
	}

	validFound := false

	for _, orgID := range orgIDs {
		res, err := c.validator.ValidateWithOrgID(ctx, orgID)
		if err != nil {
			l.Warnf("Validation failed for org %s", orgID)
			l.Debugf("error: %v", err)

			continue
		}

		if res.Valid || res.ActiveGracePeriod {
			validFound = true

			c.logLicenseStatus(res, orgID)
		} else {
			l.Warnf("Invalid license for org %s", orgID)
		}
	}

	if !validFound {
		l.Errorf("No valid licenses found (code %s)", constant.ErrNoValidLicenses.Error())
		panic(fmt.Sprintf("%s: %s", constant.ErrNoValidLicenses.Error(), "No valid licenses found for any organization"))
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

	orgID := ctx.Get(constant.OrganizationIDHeader)
	if orgID == "" {
		l.Errorf("Missing org header (code %s)", constant.ErrMissingOrgIDHeader.Error())

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrMissingOrgIDHeader.Error(),
			"title":   "Missing Organization ID",
			"message": "X-Organization-ID header is required",
		})
	}

	if !util.ContainsOrganizationID(c.validator.GetOrganizationIDs(), orgID) {
		l.Errorf("Unknown org ID %s", orgID)

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrMissingOrgIDHeader.Error(),
			"title":   "Unknown Organization ID",
			"message": "Organization ID is not configured for this license",
		})
	}

	res, err := c.ValidateOrganization(ctx.Context(), orgID)
	if err != nil {
		l.Errorf("Validation failed for org %s: %v (code %s)", orgID, err, constant.ErrOrgLicenseValidationFail.Error())

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrOrgLicenseValidationFail.Error(),
			"title":   "License Validation Failed",
			"message": fmt.Sprintf("License validation failed for organization %s", orgID),
		})
	}

	if !res.Valid && !res.ActiveGracePeriod {
		l.Errorf("Org %s license invalid (code %s)", orgID, constant.ErrOrgLicenseInvalid.Error())

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrOrgLicenseInvalid.Error(),
			"title":   "Invalid License",
			"message": fmt.Sprintf("License is invalid or expired for organization %s", orgID),
		})
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

// ValidateOrganization checks if the license is valid for a specific organization ID
// This is used to validate a specific organization ID from the request header
func (c *LicenseClient) ValidateOrganization(ctx context.Context, orgID string) (model.ValidationResult, error) {
	if c == nil || c.validator == nil {
		return model.ValidationResult{}, fiber.ErrInternalServerError
	}

	// Ensure we're sending the exact organization ID to the backend validation API
	return c.validator.ValidateWithOrgID(ctx, orgID)
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
