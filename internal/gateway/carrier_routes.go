package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chenyu/1-tok/internal/httputil"
)


// --- Carrier routes ---

// /api/v1/orders/:id/milestones/:mid/bind-carrier
func isBindCarrierPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 7 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "orders" && parts[4] == "milestones" && parts[6] == "bind-carrier"
}

// /api/v1/orders/:id/milestones/:mid/jobs
func isCreateJobPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 7 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "orders" && parts[4] == "milestones" && parts[6] == "jobs"
}

// /api/v1/jobs/:id
func isJobPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "jobs"
}

// /api/v1/jobs/:id/:action
func isJobActionPath(path string, action string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "jobs" && parts[4] == action
}

func orderMilestoneFromBindPath(path string) (string, string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 6 {
		return "", "", errors.New("invalid bind path")
	}
	return parts[3], parts[5], nil
}

func jobIDFromPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 || parts[2] != "jobs" {
		return "", errors.New("invalid job path")
	}
	return parts[3], nil
}

func (s *Server) handleBindCarrier(w http.ResponseWriter, r *http.Request) {
	orderID, milestoneID, err := orderMilestoneFromBindPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		CarrierID    string   `json:"carrierId"`
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	binding, err := s.carrier.Bind(orderID, milestoneID, payload.CarrierID, payload.Capabilities)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"binding": binding})
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	_, milestoneID, err := orderMilestoneFromBindPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		BindingID string `json:"bindingId"`
		Input     string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	job, err := s.carrier.CreateJob(payload.BindingID, milestoneID, payload.Input)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]any{"job": job})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := jobIDFromPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	job, err := s.carrier.GetJob(jobID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) handleStartJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := jobIDFromPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	job, err := s.carrier.StartJob(jobID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) handleCompleteJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := jobIDFromPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		Output string `json:"output"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	job, err := s.carrier.CompleteJob(jobID, payload.Output)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) handleFailJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := jobIDFromPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	job, err := s.carrier.FailJob(jobID, payload.Error)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) handleJobProgress(w http.ResponseWriter, r *http.Request) {
	jobID, err := jobIDFromPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var payload struct {
		Step    int    `json:"step"`
		Total   int    `json:"total"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	job, err := s.carrier.UpdateProgress(jobID, payload.Step, payload.Total, payload.Message)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) handleJobHeartbeat(w http.ResponseWriter, r *http.Request) {
	jobID, err := jobIDFromPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Get job to find binding
	job, err := s.carrier.GetJob(jobID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	if err := s.carrier.Heartbeat(job.BindingID); err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	orderID, milestoneID, err := orderMilestoneFromBindPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	binding, err := s.carrier.GetBinding(orderID, milestoneID)
	if err != nil {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{"jobs": []any{}})
		return
	}

	jobs, err := s.carrier.ListJobs(binding.ID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (s *Server) handleGetBinding(w http.ResponseWriter, r *http.Request) {
	orderID, milestoneID, err := orderMilestoneFromBindPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	binding, err := s.carrier.GetBinding(orderID, milestoneID)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "no carrier bound"})
		return
	}

	stale, _ := s.carrier.IsStale(binding.ID)
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"binding": binding, "stale": stale})
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := jobIDFromPath(r.URL.Path)
	if err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	job, err := s.carrier.CancelJob(jobID)
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"job": job})
}
