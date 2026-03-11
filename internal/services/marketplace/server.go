package marketplace

import (
	"net/http"
	"os"

	"github.com/chenyu/1-tok/internal/services/proxy"
)

type Server struct {
	inner http.Handler
}

func NewServer() *Server {
	return &Server{
		inner: proxy.NewSingleHost(upstream(), func(req *http.Request) {
			req.URL.Path = "/api/v1" + req.URL.Path[3:]
		}),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"marketplace"}`))
		return
	}

	switch r.URL.Path {
	case "/v1/providers", "/v1/listings", "/v1/orders":
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
