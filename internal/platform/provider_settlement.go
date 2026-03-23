package platform

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

type UDTTypeScript struct {
	CodeHash string `json:"codeHash"`
	HashType string `json:"hashType"`
	Args     string `json:"args"`
}

type ProviderSettlementBinding struct {
	ID                    string        `json:"id"`
	ProviderOrgID         string        `json:"providerOrgId"`
	Asset                 string        `json:"asset"`
	PeerID                string        `json:"peerId"`
	P2PAddress            string        `json:"p2pAddress"`
	NodeRPCURL            string        `json:"nodeRpcUrl,omitempty"`
	PaymentRequestBaseURL string        `json:"paymentRequestBaseUrl,omitempty"`
	UDTTypeScript         UDTTypeScript `json:"udtTypeScript"`
	OwnershipProof        string        `json:"ownershipProof,omitempty"`
	Status                string        `json:"status"`
	CreatedAt             time.Time     `json:"createdAt"`
	VerifiedAt            *time.Time    `json:"verifiedAt,omitempty"`
	LastIncidentAt        *time.Time    `json:"lastIncidentAt,omitempty"`
}

type ProviderLiquidityPoolStatus string

const (
	ProviderLiquidityPoolStatusHealthy      ProviderLiquidityPoolStatus = "healthy"
	ProviderLiquidityPoolStatusDegraded     ProviderLiquidityPoolStatus = "degraded"
	ProviderLiquidityPoolStatusDisconnected ProviderLiquidityPoolStatus = "disconnected"
	ProviderLiquidityPoolStatusRecovering   ProviderLiquidityPoolStatus = "recovering"
	ProviderLiquidityPoolStatusSuspended    ProviderLiquidityPoolStatus = "suspended"
)

type ProviderLiquidityPool struct {
	ProviderSettlementBindingID string                      `json:"providerSettlementBindingId"`
	ProviderOrgID               string                      `json:"providerOrgId"`
	Asset                       string                      `json:"asset"`
	Status                      ProviderLiquidityPoolStatus `json:"status"`
	ReadyChannelCount           int                         `json:"readyChannelCount"`
	TotalSpendableCents         int64                       `json:"totalSpendableCents"`
	ReservedOutstandingCents    int64                       `json:"reservedOutstandingCents"`
	AvailableToAllocateCents    int64                       `json:"availableToAllocateCents"`
	LastHealthyAt               *time.Time                  `json:"lastHealthyAt,omitempty"`
	LastFailureAt               *time.Time                  `json:"lastFailureAt,omitempty"`
	WarmUntil                   *time.Time                  `json:"warmUntil,omitempty"`
	DisconnectReason            string                      `json:"disconnectReason,omitempty"`
}

type ProviderLiquidityReuseSource string

const (
	ProviderLiquidityReuseReused     ProviderLiquidityReuseSource = "reused"
	ProviderLiquidityReuseNewChannel ProviderLiquidityReuseSource = "new_channel"
)

type ProviderLiquidityReservation struct {
	ID                          string                       `json:"id"`
	OrderID                     string                       `json:"orderId,omitempty"`
	ProviderSettlementBindingID string                       `json:"providerSettlementBindingId"`
	ProviderOrgID               string                       `json:"providerOrgId"`
	ReservedCents               int64                        `json:"reservedCents"`
	ChannelID                   string                       `json:"channelId,omitempty"`
	ReuseSource                 ProviderLiquidityReuseSource `json:"reuseSource"`
	Status                      string                       `json:"status"`
	CreatedAt                   time.Time                    `json:"createdAt"`
	ReleasedAt                  *time.Time                   `json:"releasedAt,omitempty"`
}

type EnsureProviderLiquidityInput struct {
	ProviderOrgID      string                    `json:"providerOrgId"`
	Binding            ProviderSettlementBinding `json:"binding"`
	NeededReserveCents int64                     `json:"neededReserveCents"`
	CurrentPool        ProviderLiquidityPool     `json:"currentPool"`
}

type EnsureProviderLiquidityResult struct {
	ChannelID           string                       `json:"channelId,omitempty"`
	ReuseSource         ProviderLiquidityReuseSource `json:"reuseSource"`
	ReadyChannelCount   int                          `json:"readyChannelCount"`
	TotalSpendableCents int64                        `json:"totalSpendableCents"`
	WarmUntil           *time.Time                   `json:"warmUntil,omitempty"`
}

type ProviderSettlementProvisioner interface {
	EnsureProviderLiquidity(input EnsureProviderLiquidityInput) (EnsureProviderLiquidityResult, error)
}

func (a *App) SetProviderSettlementProvisioner(provisioner ProviderSettlementProvisioner) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settlementProvisioner = provisioner
}

func (a *App) RegisterProviderSettlementBinding(input ProviderSettlementBinding) (ProviderSettlementBinding, error) {
	if strings.TrimSpace(input.ProviderOrgID) == "" ||
		strings.TrimSpace(input.Asset) == "" ||
		strings.TrimSpace(input.PeerID) == "" ||
		strings.TrimSpace(input.P2PAddress) == "" {
		return ProviderSettlementBinding{}, ErrMissingRequiredFields
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, binding := range a.settlementBindings {
		if binding.ProviderOrgID == input.ProviderOrgID && binding.Status == "active" {
			return ProviderSettlementBinding{}, fmt.Errorf("active settlement binding already exists for provider %s", input.ProviderOrgID)
		}
	}

	binding := ProviderSettlementBinding{
		ID:                    fmt.Sprintf("psb_%d", len(a.settlementBindings)+1),
		ProviderOrgID:         input.ProviderOrgID,
		Asset:                 input.Asset,
		PeerID:                input.PeerID,
		P2PAddress:            input.P2PAddress,
		NodeRPCURL:            input.NodeRPCURL,
		PaymentRequestBaseURL: input.PaymentRequestBaseURL,
		UDTTypeScript:         input.UDTTypeScript,
		OwnershipProof:        input.OwnershipProof,
		Status:                "pending_verification",
		CreatedAt:             a.now(),
	}

	a.settlementBindings = append(a.settlementBindings, binding)
	if a.settlementBindingsByOrg == nil {
		a.settlementBindingsByOrg = make(map[string]int)
	}
	a.settlementBindingsByOrg[binding.ProviderOrgID] = len(a.settlementBindings) - 1
	return binding, nil
}

func (a *App) GetProviderSettlementBinding(providerOrgID string) (ProviderSettlementBinding, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.settlementBindingsByOrg == nil {
		a.reindexSettlementBindings()
	}
	if idx, ok := a.settlementBindingsByOrg[providerOrgID]; ok && idx >= 0 && idx < len(a.settlementBindings) {
		return a.settlementBindings[idx], nil
	}
	return ProviderSettlementBinding{}, fmt.Errorf("%w: %s", ErrProviderSettlementBindingNotFound, providerOrgID)
}

func (a *App) VerifyProviderSettlementBinding(bindingID string) (ProviderSettlementBinding, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.settlementBindings {
		if a.settlementBindings[i].ID == bindingID {
			now := a.now()
			a.settlementBindings[i].Status = "active"
			a.settlementBindings[i].VerifiedAt = &now
			a.settlementPools[a.settlementBindings[i].ProviderOrgID] = ProviderLiquidityPool{
				ProviderSettlementBindingID: a.settlementBindings[i].ID,
				ProviderOrgID:               a.settlementBindings[i].ProviderOrgID,
				Asset:                       a.settlementBindings[i].Asset,
				Status:                      ProviderLiquidityPoolStatusHealthy,
				LastHealthyAt:               &now,
			}
			return a.settlementBindings[i], nil
		}
	}
	return ProviderSettlementBinding{}, fmt.Errorf("settlement binding not found: %s", bindingID)
}

func (a *App) SuspendProviderSettlementBinding(bindingID string) (ProviderSettlementBinding, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.settlementBindings {
		if a.settlementBindings[i].ID == bindingID {
			now := a.now()
			a.settlementBindings[i].Status = "suspended"
			a.settlementBindings[i].LastIncidentAt = &now
			pool := a.settlementPools[a.settlementBindings[i].ProviderOrgID]
			pool.ProviderSettlementBindingID = a.settlementBindings[i].ID
			pool.ProviderOrgID = a.settlementBindings[i].ProviderOrgID
			pool.Asset = a.settlementBindings[i].Asset
			pool.Status = ProviderLiquidityPoolStatusSuspended
			pool.LastFailureAt = &now
			a.settlementPools[a.settlementBindings[i].ProviderOrgID] = pool
			return a.settlementBindings[i], nil
		}
	}
	return ProviderSettlementBinding{}, fmt.Errorf("settlement binding not found: %s", bindingID)
}

func (a *App) GetProviderSettlementPool(providerOrgID string) (ProviderLiquidityPool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	pool, ok := a.settlementPools[providerOrgID]
	if !ok {
		return ProviderLiquidityPool{}, fmt.Errorf("no settlement pool for provider %s", providerOrgID)
	}
	return pool, nil
}

func (a *App) GetProviderSettlementReservation(orderID string) (ProviderLiquidityReservation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	reservationID, ok := a.settlementReservationByOrder[orderID]
	if !ok {
		return ProviderLiquidityReservation{}, fmt.Errorf("no settlement reservation for order %s", orderID)
	}
	reservation, ok := a.settlementReservations[reservationID]
	if !ok {
		return ProviderLiquidityReservation{}, fmt.Errorf("settlement reservation not found: %s", reservationID)
	}
	return reservation, nil
}

func (a *App) ReportProviderSettlementDisconnect(providerOrgID, reason string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	if idx, ok := a.settlementBindingsByOrg[providerOrgID]; ok && idx >= 0 && idx < len(a.settlementBindings) {
		a.settlementBindings[idx].LastIncidentAt = &now
	}

	pool, ok := a.settlementPools[providerOrgID]
	if !ok {
		return fmt.Errorf("no settlement pool for provider %s", providerOrgID)
	}
	pool.Status = ProviderLiquidityPoolStatusDisconnected
	pool.DisconnectReason = reason
	pool.LastFailureAt = &now
	pool.WarmUntil = nil
	a.settlementPools[providerOrgID] = pool

	for _, reservation := range a.settlementReservations {
		if reservation.ProviderOrgID != providerOrgID || reservation.OrderID == "" || reservation.Status != "active" {
			continue
		}
		order, err := a.orders.Get(reservation.OrderID)
		if err != nil || order == nil {
			continue
		}
		order.Status = core.OrderStatusAwaitingPaymentRail
		for i := range order.Milestones {
			if order.Milestones[i].State == core.MilestoneStateRunning {
				order.Milestones[i].State = core.MilestoneStatePaused
			}
			if !slices.Contains(order.Milestones[i].AnomalyFlags, "provider_settlement_disconnected") {
				order.Milestones[i].AnomalyFlags = append(order.Milestones[i].AnomalyFlags, "provider_settlement_disconnected")
			}
		}
		if err := a.orders.Save(order); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) RecoverProviderSettlement(providerOrgID string) (ProviderLiquidityPool, error) {
	a.mu.Lock()
	settlementBinding, hasSettlementBinding := a.activeSettlementBindingLocked(providerOrgID)
	if !hasSettlementBinding || settlementBinding.Status != "active" {
		a.mu.Unlock()
		return ProviderLiquidityPool{}, ErrProviderSettlementPoolUnavailable
	}
	currentPool, ok := a.settlementPools[providerOrgID]
	if !ok {
		a.mu.Unlock()
		return ProviderLiquidityPool{}, fmt.Errorf("no settlement pool for provider %s", providerOrgID)
	}
	provisioner := a.settlementProvisioner
	if provisioner == nil {
		a.mu.Unlock()
		return ProviderLiquidityPool{}, ErrProviderSettlementProvisionerMissing
	}
	warmTTL := a.settlementWarmTTL
	now := a.now()
	currentPool.Status = ProviderLiquidityPoolStatusRecovering
	currentPool.LastFailureAt = &now
	a.settlementPools[providerOrgID] = currentPool
	recoveryPool := currentPool
	recoveryPool.AvailableToAllocateCents = recoveryPool.TotalSpendableCents
	neededReserve := currentPool.ReservedOutstandingCents
	a.mu.Unlock()

	result, err := provisioner.EnsureProviderLiquidity(EnsureProviderLiquidityInput{
		ProviderOrgID:      providerOrgID,
		Binding:            settlementBinding,
		NeededReserveCents: neededReserve,
		CurrentPool:        recoveryPool,
	})
	if err != nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		pool := a.settlementPools[providerOrgID]
		failedAt := a.now()
		pool.Status = ProviderLiquidityPoolStatusDisconnected
		pool.LastFailureAt = &failedAt
		a.settlementPools[providerOrgID] = pool
		return ProviderLiquidityPool{}, fmt.Errorf("ensure provider liquidity: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	pool := a.settlementPools[providerOrgID]
	healthyAt := a.now()
	pool.ProviderSettlementBindingID = settlementBinding.ID
	pool.ProviderOrgID = providerOrgID
	pool.Asset = settlementBinding.Asset
	pool.Status = ProviderLiquidityPoolStatusHealthy
	pool.ReadyChannelCount = result.ReadyChannelCount
	pool.TotalSpendableCents = result.TotalSpendableCents
	pool.AvailableToAllocateCents = result.TotalSpendableCents - pool.ReservedOutstandingCents
	if pool.AvailableToAllocateCents < 0 {
		pool.AvailableToAllocateCents = 0
	}
	pool.LastHealthyAt = &healthyAt
	pool.DisconnectReason = ""
	if result.WarmUntil != nil {
		pool.WarmUntil = result.WarmUntil
	} else if warmTTL > 0 {
		warmUntil := healthyAt.Add(warmTTL)
		pool.WarmUntil = &warmUntil
	}
	if err := a.resumeProviderSettlementOrdersLocked(providerOrgID); err != nil {
		return ProviderLiquidityPool{}, err
	}
	a.settlementPools[providerOrgID] = pool
	return pool, nil
}

func (a *App) WarmProviderSettlementPool(providerOrgID string, minimumAvailableCents int64) (ProviderLiquidityPool, error) {
	a.mu.Lock()
	settlementBinding, hasSettlementBinding := a.activeSettlementBindingLocked(providerOrgID)
	if !hasSettlementBinding || settlementBinding.Status != "active" {
		a.mu.Unlock()
		return ProviderLiquidityPool{}, ErrProviderSettlementPoolUnavailable
	}
	currentPool, ok := a.settlementPools[providerOrgID]
	if !ok {
		a.mu.Unlock()
		return ProviderLiquidityPool{}, fmt.Errorf("no settlement pool for provider %s", providerOrgID)
	}
	provisioner := a.settlementProvisioner
	if provisioner == nil {
		a.mu.Unlock()
		return ProviderLiquidityPool{}, ErrProviderSettlementProvisionerMissing
	}
	warmTTL := a.settlementWarmTTL
	a.mu.Unlock()

	if minimumAvailableCents < 0 {
		minimumAvailableCents = 0
	}
	if currentPool.Status == ProviderLiquidityPoolStatusDisconnected ||
		currentPool.Status == ProviderLiquidityPoolStatusRecovering ||
		currentPool.Status == ProviderLiquidityPoolStatusSuspended {
		return ProviderLiquidityPool{}, ErrProviderSettlementPoolUnavailable
	}

	neededReserve := minimumAvailableCents
	if currentPool.AvailableToAllocateCents > neededReserve {
		neededReserve = currentPool.AvailableToAllocateCents
	}
	result, err := provisioner.EnsureProviderLiquidity(EnsureProviderLiquidityInput{
		ProviderOrgID:      providerOrgID,
		Binding:            settlementBinding,
		NeededReserveCents: neededReserve,
		CurrentPool:        currentPool,
	})
	if err != nil {
		return ProviderLiquidityPool{}, fmt.Errorf("ensure provider liquidity: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	pool := a.settlementPools[providerOrgID]
	now := a.now()
	pool.ProviderSettlementBindingID = settlementBinding.ID
	pool.ProviderOrgID = providerOrgID
	pool.Asset = settlementBinding.Asset
	pool.Status = ProviderLiquidityPoolStatusHealthy
	pool.ReadyChannelCount = result.ReadyChannelCount
	pool.TotalSpendableCents = result.TotalSpendableCents
	pool.AvailableToAllocateCents = result.TotalSpendableCents - pool.ReservedOutstandingCents
	if pool.AvailableToAllocateCents < 0 {
		pool.AvailableToAllocateCents = 0
	}
	pool.LastHealthyAt = &now
	pool.DisconnectReason = ""
	if result.WarmUntil != nil {
		pool.WarmUntil = result.WarmUntil
	} else if warmTTL > 0 {
		warmUntil := now.Add(warmTTL)
		pool.WarmUntil = &warmUntil
	}
	a.settlementPools[providerOrgID] = pool
	return pool, nil
}

func (a *App) maybeReserveProviderSettlementLiquidity(providerOrgID string, totalBudgetCents int64) (*ProviderLiquidityReservation, error) {
	a.mu.Lock()
	carrierBinding, hasCarrierBinding := a.activeCarrierBindingLocked(providerOrgID)
	if hasCarrierBinding && carrierBinding.Status == "suspended" {
		a.mu.Unlock()
		return nil, ErrProviderSuspended
	}
	if !hasCarrierBinding {
		a.mu.Unlock()
		return nil, nil
	}

	settlementBinding, hasSettlementBinding := a.activeSettlementBindingLocked(providerOrgID)
	if !hasSettlementBinding {
		a.mu.Unlock()
		return nil, ErrProviderSettlementBindingRequired
	}
	if settlementBinding.Status != "active" {
		a.mu.Unlock()
		return nil, ErrProviderSettlementPoolUnavailable
	}

	currentPool := a.settlementPools[providerOrgID]
	if currentPool.Status == ProviderLiquidityPoolStatusDisconnected ||
		currentPool.Status == ProviderLiquidityPoolStatusRecovering ||
		currentPool.Status == ProviderLiquidityPoolStatusSuspended ||
		currentPool.Status == ProviderLiquidityPoolStatusDegraded {
		a.mu.Unlock()
		return nil, ErrProviderSettlementPoolUnavailable
	}

	bufferBPS := a.providerSettlementBufferBPSLocked()
	warmTTL := a.settlementWarmTTL
	provisioner := a.settlementProvisioner
	a.mu.Unlock()

	if provisioner == nil {
		return nil, ErrProviderSettlementProvisionerMissing
	}

	neededReserve := totalBudgetCents + (totalBudgetCents*bufferBPS)/10_000
	result, err := provisioner.EnsureProviderLiquidity(EnsureProviderLiquidityInput{
		ProviderOrgID:      providerOrgID,
		Binding:            settlementBinding,
		NeededReserveCents: neededReserve,
		CurrentPool:        currentPool,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure provider liquidity: %w", err)
	}
	if result.TotalSpendableCents < currentPool.ReservedOutstandingCents+neededReserve {
		return nil, ErrProviderSettlementPoolUnavailable
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	pool := a.settlementPools[providerOrgID]
	pool.ProviderSettlementBindingID = settlementBinding.ID
	pool.ProviderOrgID = providerOrgID
	pool.Asset = settlementBinding.Asset
	pool.Status = ProviderLiquidityPoolStatusHealthy
	pool.ReadyChannelCount = result.ReadyChannelCount
	pool.TotalSpendableCents = result.TotalSpendableCents
	pool.ReservedOutstandingCents += neededReserve
	pool.AvailableToAllocateCents = result.TotalSpendableCents - pool.ReservedOutstandingCents
	pool.LastHealthyAt = &now
	pool.DisconnectReason = ""
	if result.WarmUntil != nil {
		pool.WarmUntil = result.WarmUntil
	} else if warmTTL > 0 {
		warmUntil := now.Add(warmTTL)
		pool.WarmUntil = &warmUntil
	}
	a.settlementPools[providerOrgID] = pool

	reservationID := fmt.Sprintf("plr_%d", len(a.settlementReservations)+1)
	reservation := ProviderLiquidityReservation{
		ID:                          reservationID,
		ProviderSettlementBindingID: settlementBinding.ID,
		ProviderOrgID:               providerOrgID,
		ReservedCents:               neededReserve,
		ChannelID:                   result.ChannelID,
		ReuseSource:                 result.ReuseSource,
		Status:                      "reserved",
		CreatedAt:                   now,
	}
	if a.settlementReservations == nil {
		a.settlementReservations = make(map[string]ProviderLiquidityReservation)
	}
	a.settlementReservations[reservation.ID] = reservation
	return &reservation, nil
}

func (a *App) attachProviderSettlementReservation(reservationID, orderID string) error {
	if strings.TrimSpace(reservationID) == "" || strings.TrimSpace(orderID) == "" {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	reservation, ok := a.settlementReservations[reservationID]
	if !ok {
		return fmt.Errorf("settlement reservation not found: %s", reservationID)
	}
	reservation.OrderID = orderID
	reservation.Status = "active"
	a.settlementReservations[reservationID] = reservation
	if a.settlementReservationByOrder == nil {
		a.settlementReservationByOrder = make(map[string]string)
	}
	a.settlementReservationByOrder[orderID] = reservationID
	return nil
}

func (a *App) releaseProviderSettlementReservation(reservationID string) {
	if strings.TrimSpace(reservationID) == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	reservation, ok := a.settlementReservations[reservationID]
	if !ok {
		return
	}
	if reservation.Status == "released" {
		return
	}
	pool := a.settlementPools[reservation.ProviderOrgID]
	pool.ReservedOutstandingCents -= reservation.ReservedCents
	if pool.ReservedOutstandingCents < 0 {
		pool.ReservedOutstandingCents = 0
	}
	pool.AvailableToAllocateCents = pool.TotalSpendableCents - pool.ReservedOutstandingCents
	a.settlementPools[reservation.ProviderOrgID] = pool
	now := a.now()
	reservation.Status = "released"
	reservation.ReleasedAt = &now
	a.settlementReservations[reservation.ID] = reservation
}

func (a *App) resumeProviderSettlementOrdersLocked(providerOrgID string) error {
	for _, reservation := range a.settlementReservations {
		if reservation.ProviderOrgID != providerOrgID || reservation.OrderID == "" || reservation.Status != "active" {
			continue
		}
		order, err := a.orders.Get(reservation.OrderID)
		if err != nil || order == nil {
			continue
		}
		if order.Status == core.OrderStatusAwaitingPaymentRail {
			order.Status = core.OrderStatusRunning
		}
		for i := range order.Milestones {
			if order.Milestones[i].State == core.MilestoneStatePaused &&
				slices.Contains(order.Milestones[i].AnomalyFlags, "provider_settlement_disconnected") {
				order.Milestones[i].State = core.MilestoneStateRunning
				order.Milestones[i].AnomalyFlags = slices.DeleteFunc(order.Milestones[i].AnomalyFlags, func(flag string) bool {
					return flag == "provider_settlement_disconnected"
				})
			}
		}
		if err := a.orders.Save(order); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) providerSettlementBufferBPSLocked() int64 {
	if a.settlementBufferBPS <= 0 {
		return 1000
	}
	return a.settlementBufferBPS
}

func (a *App) reindexSettlementBindings() {
	a.settlementBindingsByOrg = make(map[string]int)
	for i, binding := range a.settlementBindings {
		a.settlementBindingsByOrg[binding.ProviderOrgID] = i
	}
}

func (a *App) activeCarrierBindingLocked(providerOrgID string) (ProviderCarrierBinding, bool) {
	if a.carrierBindingsByOrg == nil {
		a.reindexCarrierBindings()
	}
	if idx, ok := a.carrierBindingsByOrg[providerOrgID]; ok && idx >= 0 && idx < len(a.carrierBindings) {
		return a.carrierBindings[idx], true
	}
	return ProviderCarrierBinding{}, false
}

func (a *App) activeSettlementBindingLocked(providerOrgID string) (ProviderSettlementBinding, bool) {
	if a.settlementBindingsByOrg == nil {
		a.reindexSettlementBindings()
	}
	if idx, ok := a.settlementBindingsByOrg[providerOrgID]; ok && idx >= 0 && idx < len(a.settlementBindings) {
		return a.settlementBindings[idx], true
	}
	return ProviderSettlementBinding{}, false
}
