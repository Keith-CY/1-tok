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
	if entry.Level != LevelInfo { t.Errorf("level = %s", entry.Level) }
	if entry.Message != "hello" { t.Errorf("msg = %s", entry.Message) }
	if entry.Fields["key"] != "value" { t.Errorf("fields = %v", entry.Fields) }
}

func TestLogger_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelWarn)
	log.Debug("no"); log.Info("no")
	if buf.Len() != 0 { t.Errorf("expected no output") }
	log.Warn("yes")
	if buf.Len() == 0 { t.Error("expected warn output") }
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelDebug)
	log.Error("broke", map[string]any{"code": 500})
	var entry Entry
	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Level != LevelError { t.Errorf("level = %s", entry.Level) }
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelDebug)
	log.Debug("trace")
	if buf.Len() == 0 { t.Error("expected debug output") }
}

func TestLogger_NoFields(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelInfo)
	log.Info("plain")
	var entry Entry
	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Fields != nil { t.Errorf("expected nil fields") }
}

func TestLogger_NilOutput(t *testing.T) {
	log := New(nil, LevelInfo)
	log.Info("test") // should not panic
}

func TestLogger_With_InheritsFields(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelInfo)
	child := log.With(map[string]any{"service": "gateway"})
	child.Info("request", map[string]any{"path": "/api"})

	var entry Entry
	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Fields["service"] != "gateway" {
		t.Errorf("expected inherited field 'service', got %v", entry.Fields)
	}
	if entry.Fields["path"] != "/api" {
		t.Errorf("expected call field 'path', got %v", entry.Fields)
	}
}

func TestLogger_With_ChainedFields(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, LevelInfo)
	child := log.With(map[string]any{"a": 1}).With(map[string]any{"b": 2})
	child.Info("test")

	var entry Entry
	json.Unmarshal(buf.Bytes(), &entry)
	if entry.Fields["a"] != float64(1) { t.Errorf("missing field a") }
	if entry.Fields["b"] != float64(2) { t.Errorf("missing field b") }
}
