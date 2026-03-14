package httputil

import (
	"errors"
	"net/http"
	"strings"

	iamclient "github.com/chenyu/1-tok/internal/integrations/iam"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/serviceauth"
)

// WriteAuthError maps authentication/authorization errors to appropriate HTTP
// status codes. This replaces the duplicated writeAuthError in gateway and
// settlement servers.
func WriteAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, iamclient.ErrUnauthorized):
		WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	case errors.Is(err, serviceauth.ErrInvalidServiceToken):
		WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	case errors.Is(err, platform.ErrOrgMismatch) || errors.Is(err, platform.ErrMembershipRequired):
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case strings.Contains(err.Error(), "mismatch") || strings.Contains(err.Error(), "membership is required"):
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	default:
		WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
}
