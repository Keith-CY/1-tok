package httputil

import "net/http"

// APIVersion is the current API version.
const APIVersion = "v1.0.0"

// VersionHeader adds X-API-Version header to all responses.
func VersionHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-API-Version", APIVersion)
		next.ServeHTTP(w, r)
	})
}
