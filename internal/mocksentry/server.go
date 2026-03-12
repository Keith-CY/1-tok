package mocksentry

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Path      string    `json:"path"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type Server struct {
	mu     sync.Mutex
	events []Event
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "mock-sentry"})
	case r.Method == http.MethodGet && r.URL.Path == "/events":
		s.mu.Lock()
		defer s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"count": len(s.events), "events": append([]Event(nil), s.events...)})
	case r.Method == http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.events = append(s.events, Event{
			Path:      r.URL.Path,
			Body:      string(body),
			CreatedAt: time.Now().UTC(),
		})
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
