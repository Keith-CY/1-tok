package demoenv

import "time"

type Verdict string

const (
	VerdictReady   Verdict = "ready"
	VerdictBlocked Verdict = "blocked"
)

type ServiceStatus struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	BaseURL string `json:"baseUrl,omitempty"`
	Healthy bool   `json:"healthy"`
	Detail  string `json:"detail,omitempty"`
}

type ActorStatus struct {
	Role   string `json:"role"`
	Email  string `json:"email,omitempty"`
	OrgID  string `json:"orgId,omitempty"`
	Ready  bool   `json:"ready"`
	Detail string `json:"detail,omitempty"`
}

type BuyerBalanceStatus struct {
	BuyerOrgID            string `json:"buyerOrgId,omitempty"`
	SettledTopUpCents     int64  `json:"settledTopUpCents"`
	SettledTopUpCount     int    `json:"settledTopUpCount"`
	PendingTopUpCount     int    `json:"pendingTopUpCount"`
	MinimumRequiredCents  int64  `json:"minimumRequiredCents"`
	MeetsMinimumThreshold bool   `json:"meetsMinimumThreshold"`
}

type ProviderSettlementStatus struct {
	ProviderOrgID            string `json:"providerOrgId,omitempty"`
	CarrierBindingStatus     string `json:"carrierBindingStatus,omitempty"`
	SettlementBindingStatus  string `json:"settlementBindingStatus,omitempty"`
	PoolStatus               string `json:"poolStatus,omitempty"`
	ReadyChannelCount        int    `json:"readyChannelCount"`
	AvailableToAllocateCents int64  `json:"availableToAllocateCents"`
	ReservedOutstandingCents int64  `json:"reservedOutstandingCents"`
	MinimumRequiredCents     int64  `json:"minimumRequiredCents"`
	MeetsMinimumThreshold    bool   `json:"meetsMinimumThreshold"`
}

type Status struct {
	CheckedAt          time.Time                `json:"checkedAt"`
	ResourcePrefix     string                   `json:"resourcePrefix,omitempty"`
	Verdict            Verdict                  `json:"verdict"`
	BlockerReasons     []string                 `json:"blockerReasons"`
	Services           []ServiceStatus          `json:"services"`
	Actors             []ActorStatus            `json:"actors"`
	BuyerBalance       BuyerBalanceStatus       `json:"buyerBalance"`
	ProviderSettlement ProviderSettlementStatus `json:"providerSettlement"`
}

func FinalizeStatus(status Status) Status {
	blockers := make([]string, 0, len(status.BlockerReasons)+4)
	blockers = append(blockers, status.BlockerReasons...)

	for _, service := range status.Services {
		if !service.Healthy {
			blockers = append(blockers, service.Label+" is not healthy")
		}
	}
	for _, actor := range status.Actors {
		if !actor.Ready {
			blockers = append(blockers, actor.Role+" actor is not ready")
		}
	}
	if !status.BuyerBalance.MeetsMinimumThreshold {
		blockers = append(blockers, "buyer prefund balance is below the demo threshold")
	}
	if !status.ProviderSettlement.MeetsMinimumThreshold {
		blockers = append(blockers, "provider liquidity pool is below the demo threshold")
	}

	status.BlockerReasons = dedupeNonEmpty(blockers)
	if len(status.BlockerReasons) == 0 {
		status.Verdict = VerdictReady
	} else {
		status.Verdict = VerdictBlocked
	}
	if status.CheckedAt.IsZero() {
		status.CheckedAt = time.Now().UTC()
	}
	return status
}

func dedupeNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	deduped := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}
