package logging_test

import (
	"testing"

	"github.com/rustybrownlee/ot-simulator/plant/internal/logging"
)

func TestNewLogger_ReturnsNonNil(t *testing.T) {
	logger := logging.NewLogger()
	if logger == nil {
		t.Fatal("NewLogger() returned nil")
	}
}

func TestNewTestLogger_ReturnsNonNil(t *testing.T) {
	logger := logging.NewTestLogger()
	if logger == nil {
		t.Fatal("NewTestLogger() returned nil")
	}
}

func TestLogger_Methods_DoNotPanic(t *testing.T) {
	logger := logging.NewTestLogger()

	tests := []struct {
		name string
		fn   func()
	}{
		{"Debug", func() { logger.Debug("debug message", "key", "value") }},
		{"Info", func() { logger.Info("info message", "key", "value") }},
		{"Warn", func() { logger.Warn("warn message", "key", "value") }},
		{"Error", func() { logger.Error("error message", "key", "value") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked: %v", tt.name, r)
				}
			}()
			tt.fn()
		})
	}
}

func TestLogger_WithFields_PreservesBaseFields(t *testing.T) {
	logger := logging.NewTestLogger()
	enriched := logger.WithFields("component", "plc", "unit", "A")
	if enriched == nil {
		t.Fatal("WithFields() returned nil")
	}
	// Verify chaining does not panic.
	enriched.Info("test message", "extra", "field")
}

func TestLogger_WithFields_Chaining(t *testing.T) {
	logger := logging.NewTestLogger()
	step1 := logger.WithFields("layer", "modbus")
	step2 := step1.WithFields("addr", "10.10.10.1")
	if step2 == nil {
		t.Fatal("chained WithFields() returned nil")
	}
	step2.Debug("chained fields test")
}

func TestLogger_WithLevel_Debug(t *testing.T) {
	logger := logging.NewTestLogger()
	leveled := logger.WithLevel(logging.LevelDebug)
	if leveled == nil {
		t.Fatal("WithLevel() returned nil")
	}
	leveled.Debug("should not panic")
}

func TestLogger_WithLevel_AllLevels(t *testing.T) {
	levels := []logging.Level{
		logging.LevelDebug,
		logging.LevelInfo,
		logging.LevelWarn,
		logging.LevelError,
	}
	logger := logging.NewTestLogger()
	for _, level := range levels {
		l := logger.WithLevel(level)
		if l == nil {
			t.Errorf("WithLevel(%v) returned nil", level)
		}
	}
}

func TestLogger_WithFields_EmptyFields(t *testing.T) {
	logger := logging.NewTestLogger()
	enriched := logger.WithFields()
	if enriched == nil {
		t.Fatal("WithFields() with no args returned nil")
	}
	enriched.Info("empty fields")
}
