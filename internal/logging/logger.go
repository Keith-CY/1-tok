// Package logging provides structured JSON logging.
package logging

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// Level represents a log level.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Entry is a structured log entry.
type Entry struct {
	Level     Level          `json:"level"`
	Message   string         `json:"msg"`
	Timestamp string         `json:"ts"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// Logger writes structured JSON log entries.
type Logger struct {
	output io.Writer
	level  Level
}

var levelOrder = map[Level]int{
	LevelDebug: 0,
	LevelInfo:  1,
	LevelWarn:  2,
	LevelError: 3,
}

// New creates a new logger.
func New(output io.Writer, level Level) *Logger {
	if output == nil {
		output = os.Stderr
	}
	return &Logger{output: output, level: level}
}

func (l *Logger) shouldLog(level Level) bool {
	return levelOrder[level] >= levelOrder[l.level]
}

func (l *Logger) log(level Level, msg string, fields map[string]any) {
	if !l.shouldLog(level) {
		return
	}
	entry := Entry{
		Level:     level,
		Message:   msg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Fields:    fields,
	}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')
	l.output.Write(data)
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, fields ...map[string]any) {
	l.log(LevelDebug, msg, mergeFields(fields))
}

// Info logs at info level.
func (l *Logger) Info(msg string, fields ...map[string]any) {
	l.log(LevelInfo, msg, mergeFields(fields))
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, fields ...map[string]any) {
	l.log(LevelWarn, msg, mergeFields(fields))
}

// Error logs at error level.
func (l *Logger) Error(msg string, fields ...map[string]any) {
	l.log(LevelError, msg, mergeFields(fields))
}

// With returns a child logger with additional default fields.
func (l *Logger) With(fields map[string]any) *Logger {
	return &Logger{
		output: &prefixWriter{parent: l.output, fields: fields},
		level:  l.level,
	}
}

func mergeFields(fields []map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	return fields[0]
}

type prefixWriter struct {
	parent io.Writer
	fields map[string]any
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	return pw.parent.Write(p)
}
