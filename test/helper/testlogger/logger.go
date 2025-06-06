// Package testlogger provides a test logger implementation for testing
package testlogger

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/LerianStudio/lib-commons/commons/log"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Level   string
	Message string
	Fields  map[string]any
}

// TestLogger implements log.Logger for testing purposes
type TestLogger struct {
	mu      sync.Mutex
	entries []LogEntry
}

// New creates a new TestLogger
func New() *TestLogger {
	return &TestLogger{
		entries: make([]LogEntry, 0),
	}
}

// log adds a log entry
func (l *TestLogger) log(level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	l.entries = append(l.entries, LogEntry{
		Level:   level,
		Message: msg,
	})
}

// Debug implements log.Logger
func (l *TestLogger) Debug(args ...any) {
	l.log("DEBUG", fmt.Sprint(args...))
}

// Debugf implements log.Logger
func (l *TestLogger) Debugf(format string, args ...any) {
	l.log("DEBUG", format, args...)
}

// Debugln implements log.Logger
func (l *TestLogger) Debugln(args ...any) {
	l.log("DEBUG", fmt.Sprintln(args...))
}

// Info implements log.Logger
func (l *TestLogger) Info(args ...any) {
	l.log("INFO", fmt.Sprint(args...))
}

// Infof implements log.Logger
func (l *TestLogger) Infof(format string, args ...any) {
	l.log("INFO", format, args...)
}

// Infoln implements log.Logger
func (l *TestLogger) Infoln(args ...any) {
	l.log("INFO", fmt.Sprintln(args...))
}

// Warn implements log.Logger
func (l *TestLogger) Warn(args ...any) {
	l.log("WARN", fmt.Sprint(args...))
}

// Warnf implements log.Logger
func (l *TestLogger) Warnf(format string, args ...any) {
	l.log("WARN", format, args...)
}

// Warnln implements log.Logger
func (l *TestLogger) Warnln(args ...any) {
	l.log("WARN", fmt.Sprintln(args...))
}

// Error implements log.Logger
func (l *TestLogger) Error(args ...any) {
	l.log("ERROR", fmt.Sprint(args...))
}

// Errorf implements log.Logger
func (l *TestLogger) Errorf(format string, args ...any) {
	l.log("ERROR", format, args...)
}

// Errorln implements log.Logger
func (l *TestLogger) Errorln(args ...any) {
	l.log("ERROR", fmt.Sprintln(args...))
}

// Fatal implements log.Logger
func (l *TestLogger) Fatal(args ...any) {
	l.log("FATAL", fmt.Sprint(args...))
}

// Fatalf implements log.Logger
func (l *TestLogger) Fatalf(format string, args ...any) {
	l.log("FATAL", format, args...)
}

// Fatalln implements log.Logger
func (l *TestLogger) Fatalln(args ...any) {
	l.log("FATAL", fmt.Sprintln(args...))
}

// WithField implements log.Logger
func (l *TestLogger) WithField(key string, value any) log.Logger {
	// For simplicity, we don't track fields in this implementation
	return l
}

// WithFields implements log.Logger
func (l *TestLogger) WithFields(fields ...any) log.Logger {
	// For simplicity, we don't track fields in this implementation
	return l
}

// Sync implements log.Logger
func (l *TestLogger) Sync() error {
	// Nothing to sync in test logger
	return nil
}

// WithDefaultMessageTemplate implements log.Logger
func (l *TestLogger) WithDefaultMessageTemplate(template string) log.Logger {
	// For simplicity, we don't use the template in test logger
	return l
}

// GetEntries returns all log entries
func (l *TestLogger) GetEntries() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries := make([]LogEntry, len(l.entries))
	copy(entries, l.entries)

	return entries
}

// Clear clears all log entries
func (l *TestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]LogEntry, 0)
}

// Count returns the number of log entries for the given level
func (l *TestLogger) Count(level string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	count := 0

	for _, entry := range l.entries {
		if entry.Level == level {
			count++
		}
	}

	return count
}

// Contains returns true if the log contains a message that contains all the given strings
func (l *TestLogger) Contains(level string, substrings ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, entry := range l.entries {
		if entry.Level == level {
			allFound := true

			for _, s := range substrings {
				if !bytes.Contains([]byte(entry.Message), []byte(s)) {
					allFound = false
					break
				}
			}

			if allFound {
				return true
			}
		}
	}

	return false
}
