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

	// Log mode information
	if validator.IsGlobal {
		validator.GetLogger().Infof("Initializing license client for global plugin")
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
			c.validateOrgSpecificLicensesOnStartup(bgCtx)
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
			return c.handleGlobalPluginRequest(ctx)
		}

		return c.handleOrgSpecificPluginRequest(ctx)
	}
}

// validateGlobalLicenseOnStartup validates the global license during application start.
// Panics if the license is not valid so that the app never starts with an invalid license.
func (c *LicenseClient) validateGlobalLicenseOnStartup(ctx context.Context) {
	l := c.validator.GetLogger()

	l.Info("Validating global plugin license")

	result, err := c.validator.ValidateWithOrgID(ctx, constant.GlobalPluginValue)
	if err != nil {
		l.Errorf("Global license validation failed: %v (code %s)", err, constant.ErrGlobalLicenseValidationFailed.Error())
		panic(fmt.Sprintf("%s: %v", constant.ErrGlobalLicenseValidationFailed.Error(), err))
	}

	if !result.Valid && !result.ActiveGracePeriod && !result.IsTrial {
		l.Errorf("Global license is invalid (code %s)", constant.ErrGlobalLicenseInvalid.Error())
		panic(constant.ErrGlobalLicenseInvalid)
	}

	c.logLicenseStatus(result, constant.GlobalPluginValue)
}

// validateOrgSpecificLicensesOnStartup validates each configured organization licence at startup.
// Panics if no valid licences are found so that the app never starts unlicensed.
func (c *LicenseClient) validateOrgSpecificLicensesOnStartup(ctx context.Context) {
	l := c.validator.GetLogger()

	l.Info("Validating organization-specific licenses")

	orgIDs := c.validator.GetOrganizationIDs()

	if len(orgIDs) == 0 {
		l.Errorf("No organization IDs configured (code %s)", constant.ErrNoOrganizationIDs.Error())
		panic(constant.ErrNoOrganizationIDs)
	}

	validFound := false

	for _, orgID := range orgIDs {
		res, err := c.validator.ValidateWithOrgID(ctx, orgID)
		if err != nil {
			l.Warnf("Validation for org %s failed: %v", orgID, err)
			continue
		}

		if res.Valid || res.ActiveGracePeriod || res.IsTrial {
			validFound = true

			c.logLicenseStatus(res, orgID)
		} else {
			l.Warnf("Invalid license for org %s", orgID)
		}
	}

	if !validFound {
		l.Errorf("No valid licenses found (code %s)", constant.ErrNoValidLicenses.Error())
		panic(constant.ErrNoValidLicenses)
	}
}

// handleGlobalPluginRequest validates global licence every request.
func (c *LicenseClient) handleGlobalPluginRequest(ctx *fiber.Ctx) error {
	l := c.validator.GetLogger()

	res, err := c.ValidateOrganization(ctx.Context(), constant.GlobalPluginValue)
	if err != nil {
		l.Warnf("Global validation failed: %v (code %s)", err, constant.ErrRequestGlobalLicenseValidationFail.Error())

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrRequestGlobalLicenseValidationFail.Error(),
			"title":   "License Validation Failed",
			"message": "Global plugin license validation failed",
		})
	}

	if !res.Valid && !res.ActiveGracePeriod && !res.IsTrial {
		l.Warnf("Global license invalid (code %s)", constant.ErrRequestGlobalLicenseInvalid.Error())

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrRequestGlobalLicenseInvalid.Error(),
			"title":   "Invalid License",
			"message": "Global license is invalid or expired",
		})
	}

	return ctx.Next()
}

// handleOrgSpecificPluginRequest validates license for org ID provided in header.
func (c *LicenseClient) handleOrgSpecificPluginRequest(ctx *fiber.Ctx) error {
	l := c.validator.GetLogger()

	orgID := ctx.Get(constant.OrganizationIDHeader)
	if orgID == "" {
		l.Warnf("Missing org header (code %s)", constant.ErrMissingOrgIDHeader.Error())

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrMissingOrgIDHeader.Error(),
			"title":   "Missing Organization ID",
			"message": "X-Organization-ID header is required",
		})
	}

	if !util.ContainsOrganizationID(c.validator.GetOrganizationIDs(), orgID) {
		l.Warnf("Unknown org ID %s", orgID)

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrMissingOrgIDHeader.Error(),
			"title":   "Unknown Organization ID",
			"message": "Organization ID is not configured for this license",
		})
	}

	res, err := c.ValidateOrganization(ctx.Context(), orgID)
	if err != nil {
		l.Warnf("Validation failed for org %s: %v (code %s)", orgID, err, constant.ErrOrgLicenseValidationFail.Error())

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrOrgLicenseValidationFail.Error(),
			"title":   "License Validation Failed",
			"message": fmt.Sprintf("License validation failed for organization %s", orgID),
		})
	}

	if !res.Valid && !res.ActiveGracePeriod && !res.IsTrial {
		l.Warnf("Org %s license invalid (code %s)", orgID, constant.ErrOrgLicenseInvalid.Error())

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrOrgLicenseInvalid.Error(),
			"title":   "Invalid License",
			"message": fmt.Sprintf("License is invalid or expired for organization %s", orgID),
		})
	}

	return ctx.Next()
}

// Validate checks if the license is valid across all organization IDs
func (c *LicenseClient) Validate(ctx context.Context) (model.ValidationResult, error) {
	if c == nil || c.validator == nil {
		return model.ValidationResult{}, fiber.ErrInternalServerError
	}

	return c.validator.Validate(ctx)
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
func (c *LicenseClient) logLicenseStatus(res model.ValidationResult, orgID string) {
	// The validation client already implements comprehensive license logging
	// No need to duplicate logging logic here

	// These validation client methods already handle all the different license states:
	// - logTrialLicense for trial licenses
	// - logValidLicense for valid non-trial licenses
	// - logGracePeriod for licenses in grace period

	// Only log errors for invalid licenses not in grace period
	// All other logging is handled by the validation client
	if !res.Valid && !res.ActiveGracePeriod && !res.IsTrial {
		l := c.validator.GetLogger()
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
