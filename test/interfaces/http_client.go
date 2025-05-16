// Package interfaces contains interfaces extracted from the codebase for mocking
package interfaces

import (
	"net/http"
)

// HTTPClient defines the interface for the HTTP client used in the project
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
