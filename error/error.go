package error

import (
	"errors"
	"net"
	"strings"
)

// ApiError is a custom error type to propagate HTTP status codes
// for strict error handling in Validate.
type ApiError struct {
	StatusCode int
	Msg        string
}

func (e *ApiError) Error() string {
	return e.Msg
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
