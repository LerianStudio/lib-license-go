package middleware

import (
	cn "github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/pkg"
	pkgHTTP "github.com/LerianStudio/lib-license-go/pkg/net/http"
	"github.com/gofiber/fiber/v2"
)

// Middleware creates a Fiber middleware that validates the license and manages background refresh
func (c *LicenseClient) Middleware() fiber.Handler {
	// Perform startup validation
	c.startupValidation()

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

	// Extract organization ID from header
	orgID := ctx.Get(cn.OrganizationIDHeader)

	// Use the shared validation function
	res, err := c.validateOrganizationID(ctx.Context(), orgID)
	if err != nil {
		if err == cn.ErrMissingOrgIDHeader {
			l.Errorf("Missing org header (code %s)", cn.ErrMissingOrgIDHeader.Error())
			return pkgHTTP.WithError(ctx, pkg.ValidateBusinessError(err, "", cn.OrganizationIDHeader))
		}

		if err == cn.ErrUnknownOrgIDHeader {
			l.Errorf("Unknown org ID %s", orgID)
			return pkgHTTP.WithError(ctx, pkg.ValidateBusinessError(err, "", orgID))
		}

		l.Errorf("Validation failed for org %s: %v", orgID, err)
		return pkgHTTP.WithError(ctx, pkg.ValidateBusinessError(err, "", orgID))
	}

	// Check if license is valid
	if !res.Valid && !res.ActiveGracePeriod {
		l.Errorf("Org %s license invalid", orgID)
		return pkgHTTP.WithError(ctx, pkg.ValidateBusinessError(cn.ErrOrgLicenseInvalid, "", orgID))
	}

	return ctx.Next()
}
