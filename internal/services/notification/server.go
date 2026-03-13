package notification

import (
	"fmt"
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/services/proxy"
)

type Server struct {
	inner http.Handler
}

func NewServer() *Server {
	s, err := NewServerE()
	if err != nil {
		panic(fmt.Sprintf("notification: %v", err))
	}
	return s
}

// NewServerE creates a Server with explicit error handling instead of panic.
func NewServerE() (*Server, error) {
	inner, err := proxy.NewSingleHostE(upstream(), func(req *http.Request) {
		req.URL.Path = "/api/v1/messages"
	})
	if err != nil {
		return nil, fmt.Errorf("notification upstream: %w", err)
	}
	return &Server{inner: inner}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"notification"}`))
		return
	}

	if r.URL.Path == "/v1/messages" {
		s.inner.ServeHTTP(w, r)
		return
	}

	http.NotFound(w, r)
}

func upstream() string {
	if value := os.Getenv("API_GATEWAY_UPSTREAM"); value != "" {
		return value
	}
	return "http://127.0.0.1:8080"
}
