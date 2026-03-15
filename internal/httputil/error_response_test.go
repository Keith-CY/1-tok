package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}

	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "invalid input" {
		t.Errorf("error = %s", resp.Error)
	}
	if resp.Code != ErrCodeBadRequest {
		t.Errorf("code = %s", resp.Code)
	}
}

func TestWriteErrorWithDetails(t *testing.T) {
	w := httptest.NewRecorder()
	WriteErrorWithDetails(w, http.StatusUnprocessableEntity, ErrCodeValidation, "validation failed", map[string]string{
		"field": "score must be 1-5",
	})

	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != ErrCodeValidation {
		t.Errorf("code = %s", resp.Code)
	}
	if resp.Details == nil {
		t.Error("expected details")
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []string{
		ErrCodeBadRequest, ErrCodeUnauthorized, ErrCodeForbidden,
		ErrCodeNotFound, ErrCodeConflict, ErrCodeInternal,
		ErrCodeTimeout, ErrCodeValidation, ErrCodeRateLimited,
	}
	for _, code := range codes {
		if code == "" {
			t.Error("empty error code")
		}
	}
}
