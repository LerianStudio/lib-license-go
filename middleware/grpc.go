package middleware

import (
	"context"

	cn "github.com/LerianStudio/lib-license-go/constant"
	"github.com/LerianStudio/lib-license-go/pkg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor creates a gRPC unary server interceptor that validates the license
// It works similarly to the HTTP middleware but adapted for gRPC context
func (c *LicenseClient) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	// Validate client initialization
	c.ValidateInitialization("create unary interceptor")

	// Perform startup validation
	c.startupValidation()

	// Return the interceptor function
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// Validate client initialization for each request
		c.ValidateInitialization("process unary request")

		if c.validator.IsGlobal {
			// In global mode, validation happens at startup and through background refresh
			return handler(ctx, req)
		}

		return c.processGRPCMultiOrgRequest(ctx, req, info, handler)
	}
}

// StreamServerInterceptor creates a gRPC stream server interceptor that validates the license
func (c *LicenseClient) StreamServerInterceptor() grpc.StreamServerInterceptor {
	// Validate client initialization
	c.ValidateInitialization("create stream interceptor")

	// Perform startup validation
	c.startupValidation()

	// Return the interceptor function
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Validate client initialization for each request
		c.ValidateInitialization("process stream request")

		if c.validator.IsGlobal {
			// In global mode, validation happens at startup and through background refresh
			return handler(srv, ss)
		}

		// Validate organization ID from gRPC metadata
		if err := c.validateGRPCOrganizationID(ss.Context()); err != nil {
			return err
		}

		// Continue with the stream handling
		return handler(srv, ss)
	}
}

// validateGRPCOrganizationID extracts and validates the organization ID from gRPC metadata
// Returns an error if validation fails
// This is a helper function to avoid code duplication between unary and stream interceptors
func (c *LicenseClient) validateGRPCOrganizationID(ctx context.Context) error {
	l := c.validator.GetLogger()

	// Extract organization ID from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		l.Error("Failed to extract metadata from gRPC context")
		return status.Error(codes.Internal, "missing metadata")
	}

	// Get organization ID from metadata
	orgIDs := md.Get(cn.OrganizationIDHeader)
	if len(orgIDs) == 0 {
		l.Errorf("Missing org header (code %s)", cn.ErrMissingOrgIDHeader.Error())
		return status.Error(codes.InvalidArgument, cn.ErrMissingOrgIDHeader.Error())
	}

	orgID := orgIDs[0]

	// Validate the organization ID
	res, err := c.validateOrganizationID(ctx, orgID)
	if err != nil {
		if err == cn.ErrUnknownOrgIDHeader {
			l.Errorf("Unknown org ID %s", orgID)

			return status.Error(codes.InvalidArgument, cn.ErrUnknownOrgIDHeader.Error())
		}

		l.Errorf("Validation failed for org %s: %v", orgID, err)

		return status.Error(codes.PermissionDenied, pkg.ValidateBusinessError(err, "", orgID).Error())
	}

	// Check if the license is valid
	if !res.Valid && !res.ActiveGracePeriod {
		l.Errorf("Org %s license invalid", orgID)

		return status.Error(codes.PermissionDenied, cn.ErrOrgLicenseInvalid.Error())
	}

	return nil
}

// processGRPCMultiOrgRequest handles gRPC requests in multi-org mode
func (c *LicenseClient) processGRPCMultiOrgRequest(
	ctx context.Context,
	req any,
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	// Validate organization ID from gRPC metadata
	if err := c.validateGRPCOrganizationID(ctx); err != nil {
		return nil, err
	}

	// Continue with the request handling
	return handler(ctx, req)
}
