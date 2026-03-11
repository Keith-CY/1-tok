package notification

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
			req.URL.Path = "/api/v1/messages"
		}),
	}
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
