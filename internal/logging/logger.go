// Package logging provides structured logging for Ivaldi using Go's log/slog.
//
// This package wraps slog to provide:
// - Configurable verbosity levels (-v, -vv, -q flags)
// - Consistent logging format across the codebase
// - Separation of operational logs (stderr) from user output (stdout)
package logging

import (
	"log/slog"
	"os"
)

// Level represents logging verbosity
type Level int

const (
	LevelQuiet  Level = -1 // Only errors
	LevelNormal Level = 0  // Warnings and errors (default)
	LevelInfo   Level = 1  // Info, warnings, errors (-v)
	LevelDebug  Level = 2  // All messages (-vv)
)

var (
	logger *slog.Logger
	level  Level = LevelNormal
)

// Init initializes the global logger with the specified verbosity level.
// Should be called early in program startup, typically from CLI initialization.
func Init(verbosity Level) {
	level = verbosity

	var slogLevel slog.Level
	switch verbosity {
	case LevelQuiet:
		slogLevel = slog.LevelError
	case LevelNormal:
		slogLevel = slog.LevelWarn
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelDebug:
		slogLevel = slog.LevelDebug
	default:
		// Handle -vvv or higher as debug
		if verbosity > LevelDebug {
			slogLevel = slog.LevelDebug
		} else {
			slogLevel = slog.LevelWarn
		}
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	logger = slog.New(slog.NewTextHandler(os.Stderr, opts))
}

// Debug logs a debug message (visible with -vv).
// Use for detailed internal operations useful for debugging.
func Debug(msg string, args ...any) {
	if logger != nil {
		logger.Debug(msg, args...)
	}
}

// Info logs an info message (visible with -v).
// Use for progress messages and operational status.
func Info(msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}

// Warn logs a warning message (visible by default).
// Use for issues that don't stop operation but should be noted.
func Warn(msg string, args ...any) {
	if logger != nil {
		logger.Warn(msg, args...)
	}
}

// Error logs an error message (always visible).
// Use for failures that affect operation.
func Error(msg string, args ...any) {
	if logger != nil {
		logger.Error(msg, args...)
	}
}

// GetLevel returns the current verbosity level.
func GetLevel() Level {
	return level
}

// IsDebug returns true if debug logging is enabled.
func IsDebug() bool {
	return level >= LevelDebug
}

// IsInfo returns true if info logging is enabled.
func IsInfo() bool {
	return level >= LevelInfo
}

// IsQuiet returns true if quiet mode is enabled.
func IsQuiet() bool {
	return level == LevelQuiet
}
