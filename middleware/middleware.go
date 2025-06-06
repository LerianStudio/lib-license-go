package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/LerianStudio/lib-commons/commons/log"
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
func NewLicenseClient(appID, orgIDs, licenseKey string, logger *log.Logger) *LicenseClient {
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
		l.Errorf("Global license validation failed: %v (code %s)", err, constant.ErrGlobalLicenseValidationFailed)
		panic(fmt.Sprintf("%s: %v", constant.ErrGlobalLicenseValidationFailed, err))
	}

	if !result.Valid && !result.ActiveGracePeriod && !result.IsTrial {
		l.Errorf("Global license is invalid (code %s)", constant.ErrGlobalLicenseInvalid)
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
		l.Errorf("No organization IDs configured (code %s)", constant.ErrNoOrganizationIDs)
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
		l.Errorf("No valid licenses found (code %s)", constant.ErrNoValidLicenses)
		panic(constant.ErrNoValidLicenses)
	}
}

// handleGlobalPluginRequest validates global licence every request.
func (c *LicenseClient) handleGlobalPluginRequest(ctx *fiber.Ctx) error {
	l := c.validator.GetLogger()

	res, err := c.ValidateOrganization(ctx.Context(), constant.GlobalPluginValue)
	if err != nil {
		l.Warnf("Global validation failed: %v (code %s)", err, constant.ErrRequestGlobalLicenseValidationFail)

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrRequestGlobalLicenseValidationFail,
			"title":   "License Validation Failed",
			"message": "Global plugin license validation failed",
		})
	}

	if !res.Valid && !res.ActiveGracePeriod && !res.IsTrial {
		l.Warnf("Global license invalid (code %s)", constant.ErrRequestGlobalLicenseInvalid)

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrRequestGlobalLicenseInvalid,
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
		l.Warnf("Missing org header (code %s)", constant.ErrMissingOrgIDHeader)

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrMissingOrgIDHeader,
			"title":   "Missing Organization ID",
			"message": "X-Organization-ID header is required",
		})
	}

	if !util.ContainsOrganizationID(c.validator.GetOrganizationIDs(), orgID) {
		l.Warnf("Unknown org ID %s", orgID)

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrMissingOrgIDHeader,
			"title":   "Unknown Organization ID",
			"message": "Organization ID is not configured for this license",
		})
	}

	res, err := c.ValidateOrganization(ctx.Context(), orgID)
	if err != nil {
		l.Warnf("Validation failed for org %s: %v (code %s)", orgID, err, constant.ErrOrgLicenseValidationFail)

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrOrgLicenseValidationFail,
			"title":   "License Validation Failed",
			"message": fmt.Sprintf("License validation failed for organization %s", orgID),
		})
	}

	if !res.Valid && !res.ActiveGracePeriod && !res.IsTrial {
		l.Warnf("Org %s license invalid (code %s)", orgID, constant.ErrOrgLicenseInvalid)

		return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"code":    constant.ErrOrgLicenseInvalid,
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

// logLicenseStatus logs the license status with organization ID
func (c *LicenseClient) logLicenseStatus(res model.ValidationResult, orgID string) {
	l := c.validator.GetLogger()

	if res.Valid {
		// Handle trial license
		if res.IsTrial {
			messagePrefix := "TRIAL LICENSE"
			messageSuffix := "Please upgrade to a full license to continue using the application"

			// Handle active trial license
			if res.ExpiryDaysLeft == 0 {
				// Trial license expires today
				l.Warnf("%s: Organization %s trial expires today. %s", messagePrefix, orgID, messageSuffix)
			} else if res.ExpiryDaysLeft <= 7 { // Using a constant for clarity
				// Trial license is about to expire soon
				l.Warnf("%s: Organization %s trial expires in %d days. %s", messagePrefix, orgID, res.ExpiryDaysLeft, messageSuffix)
			} else {
				// General trial notice
				l.Infof("%s: Organization %s is using a trial license that expires in %d days", messagePrefix, orgID, res.ExpiryDaysLeft)
			}

			return
		}

		// Log based on license state for non-trial licenses
		if res.ExpiryDaysLeft <= 7 { // Urgent warning threshold
			// License valid and within 7 days of expiration - urgent warning
			l.Warnf("WARNING: Organization %s license expires in %d days. Contact your account manager to renew", orgID, res.ExpiryDaysLeft)
		} else if res.ExpiryDaysLeft <= 30 { // Normal warning threshold
			// License valid but approaching expiration - normal warning
			l.Warnf("Organization %s license expires in %d days", orgID, res.ExpiryDaysLeft)
		} else {
			// General valid license message
			l.Infof("Organization %s has a valid license", orgID)
		}
	}

	// License is in grace period
	if res.ActiveGracePeriod {
		if res.ExpiryDaysLeft <= 3 { // Critical grace period warning threshold
			// Grace period is about to expire
			l.Warnf("CRITICAL: Organization %s grace period ends in %d days - application will terminate. Contact support immediately to renew license", orgID, res.ExpiryDaysLeft)
		} else {
			// General grace period warning
			l.Warnf("WARNING: Organization %s license has expired but grace period is active for %d more days", orgID, res.ExpiryDaysLeft)
		}
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
