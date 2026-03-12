package serviceauth

import (
	"net/http"
	"strings"
)

const HeaderName = "X-One-Tok-Service-Token"

func MatchesRequest(r *http.Request, expected string) bool {
	if strings.TrimSpace(expected) == "" {
		return true
	}
	return strings.TrimSpace(r.Header.Get(HeaderName)) == strings.TrimSpace(expected)
}
