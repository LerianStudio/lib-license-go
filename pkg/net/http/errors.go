package http

import (
	"errors"
	"net"
	"strings"

	"github.com/LerianStudio/lib-commons/commons"
	commonsHttp "github.com/LerianStudio/lib-commons/commons/net/http"
	"github.com/LerianStudio/lib-license-go/pkg"
	"github.com/gofiber/fiber/v2"
)

// WithError returns an error with the given status code and message.
func WithError(c *fiber.Ctx, err error) error {
	switch e := err.(type) {
	case pkg.EntityNotFoundError:
		return commonsHttp.NotFound(c, e.Code, e.Title, e.Message)
	case pkg.EntityConflictError:
		return commonsHttp.Conflict(c, e.Code, e.Title, e.Message)
	case pkg.ValidationError:
		return commonsHttp.BadRequest(c, pkg.ValidationKnownFieldsError{
			Code:    e.Code,
			Title:   e.Title,
			Message: e.Message,
			Fields:  nil,
		})
	case pkg.UnprocessableOperationError:
		return commonsHttp.UnprocessableEntity(c, e.Code, e.Title, e.Message)
	case pkg.UnauthorizedError:
		return commonsHttp.Unauthorized(c, e.Code, e.Title, e.Message)
	case pkg.ForbiddenError:
		return commonsHttp.Forbidden(c, e.Code, e.Title, e.Message)
	case pkg.ValidationKnownFieldsError, pkg.ValidationUnknownFieldsError:
		return commonsHttp.BadRequest(c, e)
	case pkg.ResponseError:
		var rErr commons.Response
		_ = errors.As(err, &rErr)

		return commonsHttp.JSONResponseError(c, rErr)
	default:
		var iErr pkg.InternalServerError
		_ = errors.As(pkg.ValidateInternalError(err, ""), &iErr)

		return commonsHttp.InternalServerError(c, iErr.Code, iErr.Title, iErr.Message)
	}
}

// IsConnectionError checks if an error is likely related to network connectivity
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for known connection error messages
	connectionErrors := []string{
		"connection refused",
		"no such host",
		"host unreachable",
		"i/o timeout",
		"no route to host",
		"network is unreachable",
		"operation timed out",
		"EOF",
		"connection reset by peer",
		"dial tcp",
		"TLS handshake",
		"context deadline exceeded",
		"operation canceled",
	}

	for _, msg := range connectionErrors {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(msg)) {
			return true
		}
	}

	// Check for specific error types
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Try to unwrap and check nested error
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil && unwrapped != err {
		return IsConnectionError(unwrapped)
	}

	return false
}

// IsServerError checks if an error is related to a server error (5xx)
func IsServerError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check if the error message indicates a server error (5xx)
	if strings.HasPrefix(errStr, "server error: ") {
		return true
	}

	// Try to unwrap and check nested error
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil && unwrapped != err {
		return IsServerError(unwrapped)
	}

	return false
}
