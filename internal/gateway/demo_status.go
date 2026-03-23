package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/demoenv"
	"github.com/chenyu/1-tok/internal/httputil"
	"github.com/chenyu/1-tok/internal/platform"
)

const demoStatusHTTPTimeout = 5 * time.Second

type settlementFundingRecord struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	BuyerOrgID string `json:"buyerOrgId"`
	Amount     string `json:"amount"`
	State      string `json:"state"`
}

func (s *Server) handleDemoStatus(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	status := s.buildDemoStatus(r.Context(), r.Header.Get("Authorization"))
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"status": status})
}

func (s *Server) handleDemoWarmup(w http.ResponseWriter, r *http.Request) {
	if _, err := s.resolveOpsUser(r); err != nil {
		httputil.WriteAuthError(w, err)
		return
	}

	var payload struct {
		ProviderOrgID         string `json:"providerOrgId"`
		MinimumAvailableCents int64  `json:"minimumAvailableCents"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
			httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, "invalid json")
			return
		}
	}
	if strings.TrimSpace(payload.ProviderOrgID) == "" {
		payload.ProviderOrgID = s.demoConfig.Provider.OrganizationID
	}
	if payload.MinimumAvailableCents <= 0 {
		payload.MinimumAvailableCents = s.demoConfig.MinProviderLiquidityCents
	}
	if strings.TrimSpace(payload.ProviderOrgID) == "" {
		httputil.WriteError(w, http.StatusBadRequest, httputil.ErrCodeValidation, "providerOrgId is required")
		return
	}

	pool, err := s.app.WarmProviderSettlementPool(payload.ProviderOrgID, payload.MinimumAvailableCents)
	if err != nil {
		statusCode := http.StatusBadRequest
		switch {
		case errors.Is(err, platform.ErrProviderSettlementPoolUnavailable),
			errors.Is(err, platform.ErrProviderSettlementProvisionerMissing):
			statusCode = http.StatusConflict
		}
		httputil.WriteJSON(w, statusCode, map[string]string{"error": err.Error()})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"pool": pool})
}

func (s *Server) buildDemoStatus(ctx context.Context, authHeader string) demoenv.Status {
	status := demoenv.Status{
		ResourcePrefix: s.demoConfig.ResourcePrefix,
		Services: []demoenv.ServiceStatus{
			{
				ID:      "api-gateway",
				Label:   "API Gateway",
				BaseURL: s.demoConfig.APIBaseURL,
				Healthy: true,
				Detail:  "current request reached api-gateway",
			},
		},
		Actors: []demoenv.ActorStatus{
			{
				Role:   "buyer",
				Email:  s.demoConfig.Buyer.Email,
				OrgID:  s.demoConfig.Buyer.OrganizationID,
				Ready:  strings.TrimSpace(s.demoConfig.Buyer.OrganizationID) != "",
				Detail: "configured from demo environment",
			},
			{
				Role:   "provider",
				Email:  s.demoConfig.Provider.Email,
				OrgID:  s.demoConfig.Provider.OrganizationID,
				Ready:  strings.TrimSpace(s.demoConfig.Provider.OrganizationID) != "",
				Detail: "configured from demo environment",
			},
			{
				Role:   "ops",
				Email:  s.demoConfig.Ops.Email,
				OrgID:  s.demoConfig.Ops.OrganizationID,
				Ready:  strings.TrimSpace(s.demoConfig.Ops.OrganizationID) != "",
				Detail: "configured from demo environment",
			},
		},
		BuyerBalance: demoenv.BuyerBalanceStatus{
			BuyerOrgID:           s.demoConfig.Buyer.OrganizationID,
			MinimumRequiredCents: s.demoConfig.MinBuyerBalanceCents,
		},
		ProviderSettlement: demoenv.ProviderSettlementStatus{
			ProviderOrgID:        s.demoConfig.Provider.OrganizationID,
			MinimumRequiredCents: s.demoConfig.MinProviderLiquidityCents,
		},
	}

	status.Services = append(status.Services, s.checkHealth(ctx, "settlement", "Settlement", s.demoConfig.SettlementBaseURL))
	status.Services = append(status.Services, s.checkHealth(ctx, "execution", "Execution", s.demoConfig.ExecutionBaseURL))
	status.Services = append(status.Services, s.checkHealth(ctx, "carrier", "Carrier", s.demoConfig.CarrierBaseURL))
	status.Services = append(status.Services, s.checkHealth(ctx, "fiber-adapter", "Fiber Adapter", s.demoConfig.FiberAdapterBaseURL))
	if strings.TrimSpace(s.demoConfig.IAMBaseURL) != "" {
		status.Services = append(status.Services, s.checkHealth(ctx, "iam", "IAM", s.demoConfig.IAMBaseURL))
	}

	if strings.TrimSpace(s.demoConfig.Buyer.OrganizationID) == "" {
		status.BlockerReasons = append(status.BlockerReasons, "DEMO_BUYER_ORG_ID is not configured")
	} else {
		balance, blockers := s.loadBuyerBalanceStatus(ctx, authHeader)
		status.BuyerBalance = balance
		status.BlockerReasons = append(status.BlockerReasons, blockers...)
	}

	if strings.TrimSpace(s.demoConfig.Provider.OrganizationID) == "" {
		status.BlockerReasons = append(status.BlockerReasons, "DEMO_PROVIDER_ORG_ID is not configured")
	} else {
		providerStatus, blockers := s.loadProviderSettlementStatus()
		status.ProviderSettlement = providerStatus
		status.BlockerReasons = append(status.BlockerReasons, blockers...)
	}

	return demoenv.FinalizeStatus(status)
}

func (s *Server) checkHealth(ctx context.Context, id, label, baseURL string) demoenv.ServiceStatus {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return demoenv.ServiceStatus{
			ID:      id,
			Label:   label,
			BaseURL: baseURL,
			Healthy: false,
			Detail:  "base url is not configured",
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/healthz", nil)
	if err != nil {
		return demoenv.ServiceStatus{ID: id, Label: label, BaseURL: baseURL, Healthy: false, Detail: err.Error()}
	}
	client := &http.Client{Timeout: demoStatusHTTPTimeout}
	res, err := client.Do(req)
	if err != nil {
		return demoenv.ServiceStatus{ID: id, Label: label, BaseURL: baseURL, Healthy: false, Detail: err.Error()}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return demoenv.ServiceStatus{
			ID:      id,
			Label:   label,
			BaseURL: baseURL,
			Healthy: false,
			Detail:  fmt.Sprintf("unexpected status %d", res.StatusCode),
		}
	}
	return demoenv.ServiceStatus{ID: id, Label: label, BaseURL: baseURL, Healthy: true}
}

func (s *Server) loadBuyerBalanceStatus(ctx context.Context, authHeader string) (demoenv.BuyerBalanceStatus, []string) {
	status := demoenv.BuyerBalanceStatus{
		BuyerOrgID:           s.demoConfig.Buyer.OrganizationID,
		MinimumRequiredCents: s.demoConfig.MinBuyerBalanceCents,
	}
	if strings.TrimSpace(s.demoConfig.SettlementBaseURL) == "" {
		return status, []string{"DEMO_SETTLEMENT_BASE_URL is not configured"}
	}
	records, err := fetchSettlementFundingRecords(ctx, s.demoConfig.SettlementBaseURL, authHeader, map[string]string{
		"buyerOrgId": s.demoConfig.Buyer.OrganizationID,
		"kind":       "buyer_topup",
	})
	if err != nil {
		return status, []string{fmt.Sprintf("load buyer topups: %v", err)}
	}
	for _, record := range records {
		if record.Kind != "buyer_topup" {
			continue
		}
		if strings.EqualFold(record.State, "SETTLED") {
			status.SettledTopUpCount++
			status.SettledTopUpCents += parseAmountToCents(record.Amount)
			continue
		}
		status.PendingTopUpCount++
	}
	status.MeetsMinimumThreshold = status.SettledTopUpCents >= status.MinimumRequiredCents
	return status, nil
}

func (s *Server) loadProviderSettlementStatus() (demoenv.ProviderSettlementStatus, []string) {
	status := demoenv.ProviderSettlementStatus{
		ProviderOrgID:        s.demoConfig.Provider.OrganizationID,
		MinimumRequiredCents: s.demoConfig.MinProviderLiquidityCents,
	}
	blockers := make([]string, 0, 3)

	carrierBinding, err := s.app.GetProviderCarrierBinding(s.demoConfig.Provider.OrganizationID)
	if err != nil {
		blockers = append(blockers, "provider carrier binding is missing")
	} else {
		status.CarrierBindingStatus = carrierBinding.Status
	}

	settlementBinding, err := s.app.GetProviderSettlementBinding(s.demoConfig.Provider.OrganizationID)
	if err != nil {
		blockers = append(blockers, "provider settlement binding is missing")
	} else {
		status.SettlementBindingStatus = settlementBinding.Status
	}

	pool, err := s.app.GetProviderSettlementPool(s.demoConfig.Provider.OrganizationID)
	if err != nil {
		blockers = append(blockers, "provider liquidity pool is missing")
		return status, blockers
	}
	status.PoolStatus = string(pool.Status)
	status.ReadyChannelCount = pool.ReadyChannelCount
	status.AvailableToAllocateCents = pool.AvailableToAllocateCents
	status.ReservedOutstandingCents = pool.ReservedOutstandingCents
	status.MeetsMinimumThreshold =
		status.CarrierBindingStatus == "active" &&
			status.SettlementBindingStatus == "active" &&
			pool.Status == platform.ProviderLiquidityPoolStatusHealthy &&
			pool.AvailableToAllocateCents >= status.MinimumRequiredCents
	return status, blockers
}

func fetchSettlementFundingRecords(ctx context.Context, baseURL, authHeader string, filters map[string]string) ([]settlementFundingRecord, error) {
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/v1/funding-records")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	for key, value := range filters {
		if strings.TrimSpace(value) == "" {
			continue
		}
		query.Set(key, value)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(authHeader) != "" {
		req.Header.Set("Authorization", authHeader)
	}

	client := &http.Client{Timeout: demoStatusHTTPTimeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}

	var payload struct {
		Records []settlementFundingRecord `json:"records"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Records, nil
}

func parseAmountToCents(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return int64(parsed * 100)
}
