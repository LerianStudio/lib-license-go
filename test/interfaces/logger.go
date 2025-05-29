// Package interfaces contains interfaces extracted from the codebase for mocking
package interfaces

// Logger defines the interface for the logger used in the project
type Logger interface {
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
	Debugf(format string, args ...any)
	Warnf(format string, args ...any)
	Fatalf(format string, args ...any)
}
