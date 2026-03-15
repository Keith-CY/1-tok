package httputil

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// APIKeyAuth returns middleware that validates an API key from the X-API-Key header.
// If no keys are configured, all requests pass through.
func APIKeyAuth(validKeys ...string) func(http.Handler) http.Handler {
	keySet := make(map[string]struct{}, len(validKeys))
	for _, k := range validKeys {
		if k != "" {
			keySet[k] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(keySet) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = extractBearerToken(r.Header.Get("Authorization"))
			}

			if !validateKey(key, keySet) {
				WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid api key"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractBearerToken(header string) string {
	if strings.HasPrefix(header, "Bearer ") {
		return header[7:]
	}
	return ""
}

func validateKey(key string, validKeys map[string]struct{}) bool {
	for valid := range validKeys {
		if subtle.ConstantTimeCompare([]byte(key), []byte(valid)) == 1 {
			return true
		}
	}
	return false
}
