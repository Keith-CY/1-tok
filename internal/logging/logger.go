// Package logging provides structured JSON logging with level filtering.
package logging

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Entry struct {
	Level     Level          `json:"level"`
	Message   string         `json:"msg"`
	Timestamp string         `json:"ts"`
	Fields    map[string]any `json:"fields,omitempty"`
}

type Logger struct {
	output     io.Writer
	level      Level
	baseFields map[string]any
}

var levelOrder = map[Level]int{
	LevelDebug: 0, LevelInfo: 1, LevelWarn: 2, LevelError: 3,
}

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
	merged := make(map[string]any)
	for k, v := range l.baseFields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	var finalFields map[string]any
	if len(merged) > 0 {
		finalFields = merged
	}
	entry := Entry{
		Level:     level,
		Message:   msg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Fields:    finalFields,
	}
	data, _ := json.Marshal(entry)
	data = append(data, '\n')
	l.output.Write(data)
}

func (l *Logger) Debug(msg string, fields ...map[string]any) { l.log(LevelDebug, msg, mergeFields(fields)) }
func (l *Logger) Info(msg string, fields ...map[string]any)  { l.log(LevelInfo, msg, mergeFields(fields)) }
func (l *Logger) Warn(msg string, fields ...map[string]any)  { l.log(LevelWarn, msg, mergeFields(fields)) }
func (l *Logger) Error(msg string, fields ...map[string]any) { l.log(LevelError, msg, mergeFields(fields)) }

// With returns a child logger with additional base fields.
func (l *Logger) With(fields map[string]any) *Logger {
	merged := make(map[string]any)
	for k, v := range l.baseFields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &Logger{output: l.output, level: l.level, baseFields: merged}
}

func mergeFields(fields []map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	return fields[0]
}
