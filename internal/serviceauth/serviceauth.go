package serviceauth

import (
	"net/http"
	"os"
	"strings"
)

const HeaderName = "X-One-Tok-Service-Token"

type TokenSet struct {
	primary string
	values  map[string]struct{}
}

func FromEnv(multiKey, singleKey string) TokenSet {
	return NewTokenSet(os.Getenv(multiKey), os.Getenv(singleKey))
}

func NewTokenSet(rawValues ...string) TokenSet {
	values := make(map[string]struct{})
	primary := ""

	for _, raw := range rawValues {
		for _, token := range strings.Split(raw, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if primary == "" {
				primary = token
			}
			values[token] = struct{}{}
		}
	}

	return TokenSet{primary: primary, values: values}
}

func (s TokenSet) Empty() bool {
	return len(s.values) == 0
}

func (s TokenSet) Primary() string {
	return s.primary
}

func (s TokenSet) MatchesRequest(r *http.Request) bool {
	if s.Empty() {
		return true
	}
	_, ok := s.values[strings.TrimSpace(r.Header.Get(HeaderName))]
	return ok
}

func MatchesRequest(r *http.Request, expected ...string) bool {
	tokens := NewTokenSet(expected...)
	if tokens.Empty() {
		return true
	}
	return tokens.MatchesRequest(r)
}
