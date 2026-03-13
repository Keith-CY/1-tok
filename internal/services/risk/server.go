package risk

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/chenyu/1-tok/internal/services/proxy"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
)

type Server struct {
	inner http.Handler
}

func NewServer() *Server {
	s, err := NewServerE()
	if err != nil {
		panic(fmt.Sprintf("risk: %v", err))
	}
	return s
}

// NewServerE creates a Server with explicit error handling instead of panic.
func NewServerE() (*Server, error) {
	inner, err := proxy.NewSingleHostE(runtimeconfig.APIGatewayUpstream(), func(req *http.Request) {
		req.URL.Path = "/api/v1" + req.URL.Path[3:]
	})
	if err != nil {
		return nil, fmt.Errorf("risk upstream: %w", err)
	}
	return &Server{inner: inner}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"risk"}`))
		return
	}

	if r.URL.Path == "/v1/credits/decision" || strings.HasPrefix(r.URL.Path, "/v1/disputes") {
		s.inner.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

