package logging

import (
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name      string
		verbosity Level
		wantLevel Level
	}{
		{"quiet mode", LevelQuiet, LevelQuiet},
		{"normal mode", LevelNormal, LevelNormal},
		{"info mode", LevelInfo, LevelInfo},
		{"debug mode", LevelDebug, LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.verbosity)
			if got := GetLevel(); got != tt.wantLevel {
				t.Errorf("GetLevel() = %v, want %v", got, tt.wantLevel)
			}
		})
	}
}

func TestIsDebug(t *testing.T) {
	tests := []struct {
		name      string
		verbosity Level
		want      bool
	}{
		{"quiet mode", LevelQuiet, false},
		{"normal mode", LevelNormal, false},
		{"info mode", LevelInfo, false},
		{"debug mode", LevelDebug, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.verbosity)
			if got := IsDebug(); got != tt.want {
				t.Errorf("IsDebug() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInfo(t *testing.T) {
	tests := []struct {
		name      string
		verbosity Level
		want      bool
	}{
		{"quiet mode", LevelQuiet, false},
		{"normal mode", LevelNormal, false},
		{"info mode", LevelInfo, true},
		{"debug mode", LevelDebug, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.verbosity)
			if got := IsInfo(); got != tt.want {
				t.Errorf("IsInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsQuiet(t *testing.T) {
	tests := []struct {
		name      string
		verbosity Level
		want      bool
	}{
		{"quiet mode", LevelQuiet, true},
		{"normal mode", LevelNormal, false},
		{"info mode", LevelInfo, false},
		{"debug mode", LevelDebug, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.verbosity)
			if got := IsQuiet(); got != tt.want {
				t.Errorf("IsQuiet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogFunctionsDoNotPanicWhenNotInitialized(t *testing.T) {
	// Reset logger to nil state
	logger = nil
	level = LevelNormal

	// These should not panic even when logger is nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log function panicked: %v", r)
		}
	}()

	Debug("test debug")
	Info("test info")
	Warn("test warn")
	Error("test error")
}

func TestLogFunctionsWithArgs(t *testing.T) {
	Init(LevelDebug)

	// These should not panic with structured args
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Log function panicked with args: %v", r)
		}
	}()

	Debug("test debug", "key", "value")
	Info("test info", "count", 42)
	Warn("test warn", "error", "some error")
	Error("test error", "path", "/some/path", "error", "failed")
}

func TestLevelConstants(t *testing.T) {
	// Verify level ordering
	if LevelQuiet >= LevelNormal {
		t.Errorf("LevelQuiet should be less than LevelNormal")
	}
	if LevelNormal >= LevelInfo {
		t.Errorf("LevelNormal should be less than LevelInfo")
	}
	if LevelInfo >= LevelDebug {
		t.Errorf("LevelInfo should be less than LevelDebug")
	}
}

func TestHighVerbosityLevels(t *testing.T) {
	// Test that -vvv or higher maps to debug
	Init(Level(3)) // -vvv
	if !IsDebug() {
		t.Error("Level(3) should enable debug")
	}

	Init(Level(10)) // Very verbose
	if !IsDebug() {
		t.Error("Level(10) should enable debug")
	}
}
