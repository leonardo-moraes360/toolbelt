package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func TestInit_JSON(t *testing.T) {
	var buf bytes.Buffer

	// Reset so Init can run again.
	once = sync.Once{}
	defaultLogger = nil

	InitWithWriter(&buf, true)
	Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Errorf("JSON output should contain msg field, got: %s", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Errorf("JSON output should contain key field, got: %s", output)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Errorf("Output should be valid JSON: %v", err)
	}
}

func TestInit_Text(t *testing.T) {
	var buf bytes.Buffer

	// Reset so Init can run again.
	once = sync.Once{}
	defaultLogger = nil
	useColor = false

	InitWithWriter(&buf, false)
	Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Text output should contain message, got: %s", output)
	}
	if !strings.Contains(output, "INF") {
		t.Errorf("Text output should contain level, got: %s", output)
	}
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer

	once = sync.Once{}
	defaultLogger = nil

	InitWithWriter(&buf, true)

	tests := []struct {
		fn      func(string, ...any)
		level   string
		message string
	}{
		{Debug, "DEBUG", "debug message"},
		{Info, "INFO", "info message"},
		{Warn, "WARN", "warn message"},
		{Error, "ERROR", "error message"},
	}

	for _, tt := range tests {
		buf.Reset()
		tt.fn(tt.message)
		output := buf.String()
		if !strings.Contains(output, tt.level) {
			t.Errorf("%s should log at %s level, got: %s", tt.message, tt.level, output)
		}
	}
}

func TestShouldUseColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if shouldUseColor() {
		t.Error("shouldUseColor should return false when NO_COLOR is set")
	}

	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if shouldUseColor() {
		t.Error("shouldUseColor should return false when TERM is dumb")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer

	once = sync.Once{}
	defaultLogger = nil

	InitWithWriter(&buf, true)

	logger := With("component", "test")
	logger.Info("test with attrs")

	output := buf.String()
	if !strings.Contains(output, `"component":"test"`) {
		t.Errorf("With should add attributes, got: %s", output)
	}
}
