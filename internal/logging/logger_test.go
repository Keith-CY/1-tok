package logging

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelInfo)
	log.Info("hello", map[string]any{"key": "value"})

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Level != LevelInfo {
		t.Errorf("level = %s", entry.Level)
	}
	if entry.Message != "hello" {
		t.Errorf("msg = %s", entry.Message)
	}
	if entry.Fields["key"] != "value" {
		t.Errorf("fields = %v", entry.Fields)
	}
}

func TestLogger_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelWarn)
	log.Debug("should not appear")
	log.Info("should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output, got %s", buf.String())
	}

	log.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("expected warn output")
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelDebug)
	log.Error("something broke", map[string]any{"code": 500})

	var entry Entry
	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Level != LevelError {
		t.Errorf("level = %s", entry.Level)
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelDebug)
	log.Debug("trace info")

	if buf.Len() == 0 {
		t.Error("expected debug output")
	}
}

func TestLogger_NoFields(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelInfo)
	log.Info("plain message")

	var entry Entry
	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Fields != nil {
		t.Errorf("expected nil fields, got %v", entry.Fields)
	}
}

func TestLogger_NilOutput(t *testing.T) {
	log := New(nil, LevelInfo)
	// Should not panic
	log.Info("test")
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelInfo)
	child := log.With(map[string]any{"service": "gateway"})
	child.Info("request")

	if buf.Len() == 0 {
		t.Error("expected output from child logger")
	}
}
