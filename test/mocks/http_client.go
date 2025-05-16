package mocks

import (
	"bytes"
	"io"
	"net/http"
)

// RoundTripFunc allows us to easily mock HTTP responses
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip implements the http.RoundTripper interface
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewHTTPClientMock creates a new HTTP client with a mock transport
func NewHTTPClientMock(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

// NewHTTPResponse creates a new HTTP response with specified status code and body
func NewHTTPResponse(statusCode int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

// HTTPClientConnectionErrorMock returns a mock HTTP client that simulates a connection error
func HTTPClientConnectionErrorMock() *http.Client {
	return NewHTTPClientMock(func(req *http.Request) *http.Response {
		return nil // Nil response simulates connection error
	})
}

// HTTPClientWithStatusMock returns a mock HTTP client that returns the given status code
func HTTPClientWithStatusMock(status int, body []byte) *http.Client {
	return NewHTTPClientMock(func(req *http.Request) *http.Response {
		return NewHTTPResponse(status, body)
	})
}
