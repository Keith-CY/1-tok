// Package validation provides reusable input validators.
package validation

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
)

// Error holds a list of field validation errors.
type Error struct {
	Fields map[string]string
}

func (e *Error) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for field, msg := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(parts, "; ")
}

// IsEmpty returns true if there are no errors.
func (e *Error) IsEmpty() bool { return len(e.Fields) == 0 }

// Builder collects validation errors.
type Builder struct {
	fields map[string]string
}

// New creates a new validation builder.
func New() *Builder {
	return &Builder{fields: make(map[string]string)}
}

// Required checks that a string is non-empty.
func (b *Builder) Required(field, value string) *Builder {
	if strings.TrimSpace(value) == "" {
		b.fields[field] = "is required"
	}
	return b
}

// MinLength checks minimum string length.
func (b *Builder) MinLength(field, value string, min int) *Builder {
	if len(value) < min {
		b.fields[field] = fmt.Sprintf("must be at least %d characters", min)
	}
	return b
}

// MaxLength checks maximum string length.
func (b *Builder) MaxLength(field, value string, max int) *Builder {
	if len(value) > max {
		b.fields[field] = fmt.Sprintf("must be at most %d characters", max)
	}
	return b
}

// Range checks that an integer is within bounds.
func (b *Builder) Range(field string, value, min, max int64) *Builder {
	if value < min || value > max {
		b.fields[field] = fmt.Sprintf("must be between %d and %d", min, max)
	}
	return b
}

// Positive checks that an integer is positive.
func (b *Builder) Positive(field string, value int64) *Builder {
	if value <= 0 {
		b.fields[field] = "must be positive"
	}
	return b
}

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// Slug checks that a string is a valid slug.
func (b *Builder) Slug(field, value string) *Builder {
	if !slugPattern.MatchString(value) {
		b.fields[field] = "must be a valid slug (lowercase alphanumeric with hyphens)"
	}
	return b
}

// Email checks that a string is a valid email.
func (b *Builder) Email(field, value string) *Builder {
	if _, err := mail.ParseAddress(value); err != nil {
		b.fields[field] = "must be a valid email"
	}
	return b
}

// Build returns the validation error if any fields failed.
func (b *Builder) Build() *Error {
	if len(b.fields) == 0 {
		return nil
	}
	return &Error{Fields: b.fields}
}
