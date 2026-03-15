package validation

import (
	"testing"
)

func TestRequired(t *testing.T) {
	err := New().Required("name", "").Build()
	if err == nil {
		t.Error("expected error")
	}
	if err.Fields["name"] != "is required" {
		t.Errorf("field = %s", err.Fields["name"])
	}
}

func TestRequired_Valid(t *testing.T) {
	err := New().Required("name", "Alice").Build()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestMinLength(t *testing.T) {
	err := New().MinLength("password", "ab", 8).Build()
	if err == nil {
		t.Error("expected error")
	}
}

func TestMaxLength(t *testing.T) {
	err := New().MaxLength("bio", "x"+string(make([]byte, 500)), 100).Build()
	if err == nil {
		t.Error("expected error")
	}
}

func TestRange(t *testing.T) {
	err := New().Range("score", 6, 1, 5).Build()
	if err == nil {
		t.Error("expected error for 6 outside 1-5")
	}

	err = New().Range("score", 3, 1, 5).Build()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestPositive(t *testing.T) {
	err := New().Positive("amount", 0).Build()
	if err == nil {
		t.Error("expected error for 0")
	}
	err = New().Positive("amount", -1).Build()
	if err == nil {
		t.Error("expected error for -1")
	}
}

func TestSlug(t *testing.T) {
	err := New().Slug("id", "valid-slug-123").Build()
	if err != nil {
		t.Errorf("expected valid slug, got %v", err)
	}

	err = New().Slug("id", "Invalid Slug!").Build()
	if err == nil {
		t.Error("expected error for invalid slug")
	}
}

func TestEmail(t *testing.T) {
	err := New().Email("email", "user@example.com").Build()
	if err != nil {
		t.Errorf("expected valid email, got %v", err)
	}

	err = New().Email("email", "not-an-email").Build()
	if err == nil {
		t.Error("expected error for invalid email")
	}
}

func TestChainedValidation(t *testing.T) {
	err := New().
		Required("name", "").
		MinLength("password", "ab", 8).
		Range("score", 10, 1, 5).
		Build()

	if err == nil {
		t.Fatal("expected errors")
	}
	if len(err.Fields) != 3 {
		t.Errorf("expected 3 errors, got %d", len(err.Fields))
	}
}

func TestBuild_NoErrors(t *testing.T) {
	err := New().
		Required("name", "Alice").
		MinLength("password", "longpassword", 8).
		Build()

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestError_String(t *testing.T) {
	err := &Error{Fields: map[string]string{"name": "is required"}}
	if err.Error() == "" {
		t.Error("expected non-empty error string")
	}
}

func TestError_IsEmpty(t *testing.T) {
	err := &Error{Fields: map[string]string{}}
	if !err.IsEmpty() {
		t.Error("expected empty")
	}
}
