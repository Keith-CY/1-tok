package identity

import (
	"errors"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"valid", "secret99", nil},
		{"valid long", "correct horse battery staple 1", nil},
		{"valid unicode", "пароль12", nil},
		{"too short", "abc1", ErrPasswordTooShort},
		{"empty", "", ErrPasswordTooShort},
		{"seven chars", "abcdef1", ErrPasswordTooShort},
		{"no digit", "abcdefgh", ErrPasswordNeedsDigit},
		{"no letter", "12345678", ErrPasswordNeedsLetter},
		{"exactly eight", "abcdefg1", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr error
	}{
		{"valid", "user@example.com", nil},
		{"valid subdomain", "user@mail.example.com", nil},
		{"empty", "", ErrEmailEmpty},
		{"spaces only", "   ", ErrEmailEmpty},
		{"no at", "userexample.com", ErrEmailInvalid},
		{"no local part", "@example.com", ErrEmailInvalid},
		{"no domain", "user@", ErrEmailInvalid},
		{"no dot in domain", "user@localhost", ErrEmailInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateEmail(%q) = %v, want %v", tt.email, err, tt.wantErr)
			}
		})
	}
}
