package identity

import (
	"errors"
	"strings"
	"unicode"
)

// Password policy constants.
const MinPasswordLength = 8

// Validation errors.
var (
	ErrPasswordTooShort    = errors.New("password must be at least 8 characters")
	ErrPasswordNeedsLetter = errors.New("password must contain at least one letter")
	ErrPasswordNeedsDigit  = errors.New("password must contain at least one digit")
	ErrEmailEmpty          = errors.New("email is required")
	ErrEmailInvalid        = errors.New("invalid email address")
)

// ValidatePassword checks minimum strength requirements:
//   - at least 8 characters
//   - at least one letter (unicode)
//   - at least one digit
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	var hasLetter, hasDigit bool
	for _, c := range password {
		if unicode.IsLetter(c) {
			hasLetter = true
		}
		if unicode.IsDigit(c) {
			hasDigit = true
		}
	}
	if !hasLetter {
		return ErrPasswordNeedsLetter
	}
	if !hasDigit {
		return ErrPasswordNeedsDigit
	}
	return nil
}

// ValidateEmail performs basic structural email validation:
// non-empty, contains exactly one "@" with non-empty local and domain parts,
// and domain has at least one dot.
func ValidateEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return ErrEmailEmpty
	}
	at := strings.IndexByte(trimmed, '@')
	if at < 1 || at == len(trimmed)-1 {
		return ErrEmailInvalid
	}
	domain := trimmed[at+1:]
	if !strings.Contains(domain, ".") {
		return ErrEmailInvalid
	}
	return nil
}
