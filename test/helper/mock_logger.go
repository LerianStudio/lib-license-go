package helper

import (
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/stretchr/testify/mock"
)

// MockLogger is a mock implementation of the log.Logger interface
type MockLogger struct {
	mock.Mock
}

// Debug implements log.Logger interface
func (m *MockLogger) Debug(args ...any) {
	m.Called(args...)
}

// Info implements log.Logger interface
func (m *MockLogger) Info(args ...any) {
	m.Called(args...)
}

// Warn implements log.Logger interface
func (m *MockLogger) Warn(args ...any) {
	m.Called(args...)
}

// Error implements log.Logger interface
func (m *MockLogger) Error(args ...any) {
	m.Called(args...)
}

// Fatal implements log.Logger interface
func (m *MockLogger) Fatal(args ...any) {
	m.Called(args...)
}

// Debugf implements log.Logger interface
func (m *MockLogger) Debugf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Infof implements log.Logger interface
func (m *MockLogger) Infof(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Warnf implements log.Logger interface
func (m *MockLogger) Warnf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Errorf implements log.Logger interface
func (m *MockLogger) Errorf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Fatalf implements log.Logger interface
func (m *MockLogger) Fatalf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Debugln implements log.Logger interface
func (m *MockLogger) Debugln(args ...any) {
	m.Called(args...)
}

// Infoln implements log.Logger interface
func (m *MockLogger) Infoln(args ...any) {
	m.Called(args...)
}

// Warnln implements log.Logger interface
func (m *MockLogger) Warnln(args ...any) {
	m.Called(args...)
}

// Errorln implements log.Logger interface
func (m *MockLogger) Errorln(args ...any) {
	m.Called(args...)
}

// Fatalln implements log.Logger interface
func (m *MockLogger) Fatalln(args ...any) {
	m.Called(args...)
}

// Println implements log.Logger interface
func (m *MockLogger) Println(args ...any) {
	m.Called(args...)
}

// Printf implements log.Logger interface
func (m *MockLogger) Printf(format string, args ...any) {
	m.Called(append([]any{format}, args...)...)
}

// Debugw implements log.Logger interface
func (m *MockLogger) Debugw(msg string, keysAndValues ...any) {
	m.Called(append([]any{msg}, keysAndValues...)...)
}

// Infow implements log.Logger interface
func (m *MockLogger) Infow(msg string, keysAndValues ...any) {
	m.Called(append([]any{msg}, keysAndValues...)...)
}

// Warnw implements log.Logger interface
func (m *MockLogger) Warnw(msg string, keysAndValues ...any) {
	m.Called(append([]any{msg}, keysAndValues...)...)
}

// Errorw implements log.Logger interface
func (m *MockLogger) Errorw(msg string, keysAndValues ...any) {
	m.Called(append([]any{msg}, keysAndValues...)...)
}

// Fatalw implements log.Logger interface
func (m *MockLogger) Fatalw(msg string, keysAndValues ...any) {
	m.Called(append([]any{msg}, keysAndValues...)...)
}

// WithField implements log.Logger interface
func (m *MockLogger) WithField(key string, value any) log.Logger {
	args := m.Called(key, value)
	return args.Get(0).(log.Logger)
}

// WithFields implements log.Logger interface
func (m *MockLogger) WithFields(fields ...any) log.Logger {
	args := m.Called(fields...)
	return args.Get(0).(log.Logger)
}

// Sync implements log.Logger interface
func (m *MockLogger) Sync() error {
	args := m.Called()
	return args.Error(0)
}

// WithDefaultMessageTemplate implements log.Logger interface
func (m *MockLogger) WithDefaultMessageTemplate(template string) log.Logger {
	args := m.Called(template)
	return args.Get(0).(log.Logger)
}

// NewMockLogger creates a new mock logger
func NewMockLogger() *log.Logger {
	mockLogger := &MockLogger{}
	// Return as log.Logger interface
	var logger log.Logger = mockLogger
	return &logger
}

// AsMock converts a log.Logger to a MockLogger for testing
func AsMock(l *log.Logger) *MockLogger {
	if mockLogger, ok := (*l).(*MockLogger); ok {
		return mockLogger
	}
	return nil
}
