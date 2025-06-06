// Package helper provides test utilities for the middleware package
package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServer is a wrapper around httptest.Server that provides additional test helpers
type TestServer struct {
	*httptest.Server
	URL string
}

// NewTestServer creates a new test server with the given handler
func NewTestServer(t *testing.T, handler http.Handler) *TestServer {
	t.Helper()

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	return &TestServer{
		Server: ts,
		URL:    ts.URL,
	}
}

// AssertRequestMethod asserts that the request has the expected method
func (ts *TestServer) AssertRequestMethod(t *testing.T, req *http.Request, expectedMethod string) {
	t.Helper()
	require.Equal(t, expectedMethod, req.Method, "unexpected HTTP method")
}

// AssertHeader asserts that the request has the expected header
func (ts *TestServer) AssertHeader(t *testing.T, req *http.Request, key, expectedValue string) {
	t.Helper()
	require.Equal(t, expectedValue, req.Header.Get(key), "unexpected header value")
}

// AssertQueryParam asserts that the request has the expected query parameter
func (ts *TestServer) AssertQueryParam(t *testing.T, req *http.Request, key, expectedValue string) {
	t.Helper()
	require.Equal(t, expectedValue, req.URL.Query().Get(key), "unexpected query parameter value")
}
