package mocks

import (
	commonslog "github.com/LerianStudio/lib-commons/commons/log"
)

// Logger is a minimal mock implementation of the commons logger
type Logger struct{}

func (m *Logger) Infof(format string, args ...interface{})  {}
func (m *Logger) Warnf(format string, args ...interface{})  {}
func (m *Logger) Errorf(format string, args ...interface{}) {}
func (m *Logger) Info(args ...interface{})                 {}
func (m *Logger) Warn(args ...interface{})                 {}
func (m *Logger) Error(args ...interface{})                {}
func (m *Logger) Debug(args ...interface{})                {}
func (m *Logger) Debugf(format string, args ...interface{}) {}
func (m *Logger) Debugln(args ...interface{})              {}
func (m *Logger) Errorln(args ...interface{})              {}
func (m *Logger) Fatal(args ...interface{})                {}
func (m *Logger) Fatalf(format string, args ...interface{}) {}
func (m *Logger) Fatalln(args ...interface{})              {}
func (m *Logger) Infoln(args ...interface{})               {}
func (m *Logger) Warnln(args ...interface{})               {}
func (m *Logger) WithDefaultMessageTemplate(tpl string) commonslog.Logger { return m }
func (m *Logger) WithFields(fields ...any) commonslog.Logger { return m }
func (m *Logger) Sync() error                             { return nil }

// NewLogger creates a new mock logger
func NewLogger() *Logger {
	return &Logger{}
}
