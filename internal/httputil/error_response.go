package httputil

import "net/http"

// ErrorResponse is a standardized error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// WriteError writes a standardized error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message, Code: code})
}

// WriteErrorWithDetails writes a standardized error response with details.
func WriteErrorWithDetails(w http.ResponseWriter, status int, code, message string, details any) {
	WriteJSON(w, status, ErrorResponse{Error: message, Code: code, Details: details})
}

// Standard error codes.
const (
	ErrCodeBadRequest    = "BAD_REQUEST"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeForbidden     = "FORBIDDEN"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeConflict      = "CONFLICT"
	ErrCodeInternal      = "INTERNAL"
	ErrCodeTimeout       = "TIMEOUT"
	ErrCodeValidation    = "VALIDATION_ERROR"
	ErrCodeRateLimited   = "RATE_LIMITED"
)
