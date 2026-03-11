package iam

import (
	"encoding/json"
	"net/http"
)

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"iam"}`))
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/v1/roles" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"roles": map[string][]string{
				"buyer":    {"org_owner", "procurement", "operator", "finance_viewer"},
				"provider": {"org_owner", "sales", "delivery_operator", "finance_viewer"},
				"ops":      {"ops_reviewer", "risk_admin", "finance_admin", "super_admin"},
			},
		})
		return
	}

	http.NotFound(w, r)
}
