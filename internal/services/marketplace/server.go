package marketplace

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/chenyu/1-tok/internal/services/proxy"
)

type Server struct {
	inner http.Handler
}

func NewServer() *Server {
	s, err := NewServerE()
	if err != nil {
		panic(fmt.Sprintf("marketplace: %v", err))
	}
	return s
}

// NewServerE creates a Server with explicit error handling instead of panic.
func NewServerE() (*Server, error) {
	inner, err := proxy.NewSingleHostE(upstream(), func(req *http.Request) {
		req.URL.Path = "/api/v1" + req.URL.Path[3:]
	})
	if err != nil {
		return nil, fmt.Errorf("marketplace upstream: %w", err)
	}
	return &Server{inner: inner}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"marketplace"}`))
		return
	}

	switch {
	case r.URL.Path == "/v1/providers", r.URL.Path == "/v1/listings":
		s.inner.ServeHTTP(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/rfqs"), strings.HasPrefix(r.URL.Path, "/v1/orders"):
		s.inner.ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func upstream() string {
	if value := os.Getenv("API_GATEWAY_UPSTREAM"); value != "" {
		return value
	}
	return "http://127.0.0.1:8080"
}
