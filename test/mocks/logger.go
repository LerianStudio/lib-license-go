package mocks

import (
	commonslog "github.com/LerianStudio/lib-commons/commons/log"
)

// Logger is a minimal mock implementation of the commons logger
type Logger struct{}

func (m *Logger) Infof(format string, args ...any)                        {}
func (m *Logger) Warnf(format string, args ...any)                        {}
func (m *Logger) Errorf(format string, args ...any)                       {}
func (m *Logger) Info(args ...any)                                        {}
func (m *Logger) Warn(args ...any)                                        {}
func (m *Logger) Error(args ...any)                                       {}
func (m *Logger) Debug(args ...any)                                       {}
func (m *Logger) Debugf(format string, args ...any)                       {}
func (m *Logger) Debugln(args ...any)                                     {}
func (m *Logger) Errorln(args ...any)                                     {}
func (m *Logger) Fatal(args ...any)                                       {}
func (m *Logger) Fatalf(format string, args ...any)                       {}
func (m *Logger) Fatalln(args ...any)                                     {}
func (m *Logger) Infoln(args ...any)                                      {}
func (m *Logger) Warnln(args ...any)                                      {}
func (m *Logger) WithDefaultMessageTemplate(tpl string) commonslog.Logger { return m }
func (m *Logger) WithFields(fields ...any) commonslog.Logger              { return m }
func (m *Logger) Sync() error                                             { return nil }

// NewLogger creates a new mock logger
func NewLogger() *Logger {
	return &Logger{}
}
