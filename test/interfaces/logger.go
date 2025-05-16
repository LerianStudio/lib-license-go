// Package interfaces contains interfaces extracted from the codebase for mocking
package interfaces

// Logger defines the interface for the logger used in the project
type Logger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}
