package httputil

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	iamclient "github.com/chenyu/1-tok/internal/integrations/iam"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/serviceauth"
)

func TestWriteAuthError_Unauthorized(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAuthError(rec, iamclient.ErrUnauthorized)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestWriteAuthError_InvalidToken(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAuthError(rec, serviceauth.ErrInvalidServiceToken)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestWriteAuthError_OrgMismatch(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAuthError(rec, platform.ErrOrgMismatch)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestWriteAuthError_MembershipRequired(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAuthError(rec, platform.ErrMembershipRequired)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestWriteAuthError_Generic(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAuthError(rec, errors.New("unknown error"))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rec.Code)
	}
}
