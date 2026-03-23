package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultProviderSettlementP2PPort            = 8228
	defaultProviderSettlementPollInterval       = 2 * time.Second
	defaultProviderSettlementWaitTimeout        = 90 * time.Second
	defaultProviderSettlementOpenChannelRetries = 10
	defaultProviderSettlementAcceptRetries      = 10
	defaultProviderSettlementAcceptRetryEvery   = 6
	defaultProviderSettlementCentsPerUnit       = int64(100)
	defaultProviderSettlementMinFundingUnits    = int64(1)
)

type FNNProviderSettlementProvisionerConfig struct {
	TreasuryRPCURL       string
	TreasuryP2PHost      string
	TreasuryP2PPort      int
	PollInterval         time.Duration
	WaitTimeout          time.Duration
	OpenChannelRetries   int
	AcceptChannelRetries int
	AcceptRetryEvery     int
	CentsPerUnit         int64
	MinFundingUnits      int64
}

type fnnProviderSettlementProvisioner struct {
	cfg FNNProviderSettlementProvisionerConfig
}

type providerSettlementRawFNNClient struct {
	client *providerSettlementRawRPCClient
}

type providerSettlementRawRPCClient struct {
	endpoint   string
	httpClient *http.Client
}

type providerSettlementNodeInfo struct {
	NodeID                            string                             `json:"node_id"`
	AutoAcceptChannelCKBFundingAmount string                             `json:"auto_accept_channel_ckb_funding_amount"`
	UDTCfgInfos                       []providerSettlementNodeUDTCfgInfo `json:"udt_cfg_infos"`
}

type providerSettlementNodeUDTCfgInfo struct {
	Name             string                             `json:"name"`
	Script           providerSettlementRawUDTTypeScript `json:"script"`
	AutoAcceptAmount string                             `json:"auto_accept_amount"`
}

type providerSettlementRawChannelList struct {
	Channels []providerSettlementRawChannel `json:"channels"`
}

type providerSettlementRawChannel struct {
	ChannelID            string                             `json:"channel_id"`
	State                any                                `json:"state"`
	Enabled              bool                               `json:"enabled"`
	FundingUDTTypeScript providerSettlementRawUDTTypeScript `json:"funding_udt_type_script"`
}

type providerSettlementRawUDTTypeScript struct {
	CodeHash string `json:"code_hash"`
	HashType string `json:"hash_type"`
	Args     string `json:"args"`
}

func NewFNNProviderSettlementProvisionerFromEnv() (ProviderSettlementProvisioner, error) {
	treasuryRPCURL := strings.TrimSpace(os.Getenv("PROVIDER_SETTLEMENT_FNN_TREASURY_RPC_URL"))
	if treasuryRPCURL == "" {
		return nil, nil
	}

	treasuryP2PHost := strings.TrimSpace(os.Getenv("PROVIDER_SETTLEMENT_FNN_TREASURY_P2P_HOST"))
	if treasuryP2PHost == "" {
		return nil, errors.New("PROVIDER_SETTLEMENT_FNN_TREASURY_P2P_HOST is required when PROVIDER_SETTLEMENT_FNN_TREASURY_RPC_URL is set")
	}

	cfg := FNNProviderSettlementProvisionerConfig{
		TreasuryRPCURL:       treasuryRPCURL,
		TreasuryP2PHost:      treasuryP2PHost,
		TreasuryP2PPort:      envIntOrDefault("PROVIDER_SETTLEMENT_FNN_TREASURY_P2P_PORT", defaultProviderSettlementP2PPort),
		PollInterval:         envDurationMillisOrDefault("PROVIDER_SETTLEMENT_FNN_POLL_INTERVAL_MILLISECONDS", defaultProviderSettlementPollInterval),
		WaitTimeout:          envDurationSecondsOrDefault("PROVIDER_SETTLEMENT_FNN_WAIT_TIMEOUT_SECONDS", defaultProviderSettlementWaitTimeout),
		OpenChannelRetries:   envIntOrDefault("PROVIDER_SETTLEMENT_FNN_OPEN_CHANNEL_RETRIES", defaultProviderSettlementOpenChannelRetries),
		AcceptChannelRetries: envIntOrDefault("PROVIDER_SETTLEMENT_FNN_ACCEPT_CHANNEL_RETRIES", defaultProviderSettlementAcceptRetries),
		AcceptRetryEvery:     envIntOrDefault("PROVIDER_SETTLEMENT_FNN_ACCEPT_RETRY_INTERVAL_ATTEMPTS", defaultProviderSettlementAcceptRetryEvery),
		CentsPerUnit:         envInt64OrDefault("PROVIDER_SETTLEMENT_FNN_CENTS_PER_UNIT", defaultProviderSettlementCentsPerUnit),
		MinFundingUnits:      envInt64OrDefault("PROVIDER_SETTLEMENT_FNN_MIN_FUNDING_UNITS", defaultProviderSettlementMinFundingUnits),
	}
	return NewFNNProviderSettlementProvisioner(cfg)
}

func NewFNNProviderSettlementProvisioner(cfg FNNProviderSettlementProvisionerConfig) (ProviderSettlementProvisioner, error) {
	if strings.TrimSpace(cfg.TreasuryRPCURL) == "" {
		return nil, errors.New("treasury rpc url is required")
	}
	if strings.TrimSpace(cfg.TreasuryP2PHost) == "" {
		return nil, errors.New("treasury p2p host is required")
	}
	if cfg.TreasuryP2PPort <= 0 {
		cfg.TreasuryP2PPort = defaultProviderSettlementP2PPort
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultProviderSettlementPollInterval
	}
	if cfg.WaitTimeout <= 0 {
		cfg.WaitTimeout = defaultProviderSettlementWaitTimeout
	}
	if cfg.OpenChannelRetries <= 0 {
		cfg.OpenChannelRetries = defaultProviderSettlementOpenChannelRetries
	}
	if cfg.AcceptChannelRetries <= 0 {
		cfg.AcceptChannelRetries = defaultProviderSettlementAcceptRetries
	}
	if cfg.AcceptRetryEvery <= 0 {
		cfg.AcceptRetryEvery = defaultProviderSettlementAcceptRetryEvery
	}
	if cfg.CentsPerUnit <= 0 {
		cfg.CentsPerUnit = defaultProviderSettlementCentsPerUnit
	}
	if cfg.MinFundingUnits <= 0 {
		cfg.MinFundingUnits = defaultProviderSettlementMinFundingUnits
	}
	return &fnnProviderSettlementProvisioner{cfg: cfg}, nil
}

func (p *fnnProviderSettlementProvisioner) EnsureProviderLiquidity(input EnsureProviderLiquidityInput) (EnsureProviderLiquidityResult, error) {
	if strings.TrimSpace(input.Binding.NodeRPCURL) == "" {
		return EnsureProviderLiquidityResult{}, errors.New("provider settlement binding nodeRpcUrl is required")
	}
	if strings.TrimSpace(input.Binding.P2PAddress) == "" {
		return EnsureProviderLiquidityResult{}, errors.New("provider settlement binding p2pAddress is required")
	}

	overallTimeout := p.cfg.WaitTimeout * time.Duration(maxInt(1, p.cfg.OpenChannelRetries))
	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	treasuryNode := newProviderSettlementRawFNNClient(p.cfg.TreasuryRPCURL)
	providerNode := newProviderSettlementRawFNNClient(input.Binding.NodeRPCURL)

	treasuryInfo, err := treasuryNode.NodeInfo(ctx)
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("treasury node info: %w", err)
	}
	providerInfo, err := providerNode.NodeInfo(ctx)
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("provider node info: %w", err)
	}

	treasuryPeerID, err := deriveProviderSettlementPeerIDFromNodeID(treasuryInfo.NodeID)
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("derive treasury peer id: %w", err)
	}
	providerPeerID, err := deriveProviderSettlementPeerIDFromNodeID(providerInfo.NodeID)
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("derive provider peer id: %w", err)
	}
	if expected := strings.TrimSpace(input.Binding.PeerID); expected != "" && expected != providerPeerID {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("provider settlement binding peerId mismatch: binding=%s actual=%s", expected, providerPeerID)
	}

	providerP2PAddress, err := providerSettlementNormalizeP2PAddress(ctx, input.Binding.P2PAddress)
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("normalize provider p2p address: %w", err)
	}
	treasuryP2PAddress, err := providerSettlementNormalizeP2PAddress(ctx, providerSettlementMultiaddrForP2P(p.cfg.TreasuryP2PHost, p.cfg.TreasuryP2PPort, treasuryPeerID))
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("normalize treasury p2p address: %w", err)
	}
	if err := connectProviderSettlementPeers(ctx, treasuryNode, providerNode, providerP2PAddress, treasuryP2PAddress); err != nil {
		return EnsureProviderLiquidityResult{}, err
	}

	treasuryChannels, err := treasuryNode.ListChannels(ctx, providerPeerID)
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("list treasury channels: %w", err)
	}
	providerChannels, err := providerNode.ListChannels(ctx, treasuryPeerID)
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("list provider channels: %w", err)
	}

	readyChannelID, bothReady := matchingReadyChannelID(treasuryChannels, providerChannels, input.Binding.UDTTypeScript)
	if bothReady && input.CurrentPool.AvailableToAllocateCents >= input.NeededReserveCents {
		return EnsureProviderLiquidityResult{
			ChannelID:           readyChannelID,
			ReuseSource:         ProviderLiquidityReuseReused,
			ReadyChannelCount:   countReadyChannels(treasuryChannels, input.Binding.UDTTypeScript),
			TotalSpendableCents: maxInt64(input.CurrentPool.TotalSpendableCents, input.CurrentPool.ReservedOutstandingCents+input.CurrentPool.AvailableToAllocateCents),
			WarmUntil:           input.CurrentPool.WarmUntil,
		}, nil
	}

	fundingUnits := neededReserveToFundingUnits(input.NeededReserveCents, p.cfg.CentsPerUnit, p.cfg.MinFundingUnits)
	fundingHex, err := providerSettlementDecimalToHexQuantity(fundingUnits)
	if err != nil {
		return EnsureProviderLiquidityResult{}, fmt.Errorf("funding amount: %w", err)
	}
	acceptFundingHex := providerSettlementAcceptFundingHex(providerInfo, input.Binding.UDTTypeScript)
	if acceptFundingHex == "" {
		acceptFundingHex = "0x1"
	}

	reconnect := func(ctx context.Context) error {
		return connectProviderSettlementPeers(ctx, treasuryNode, providerNode, providerP2PAddress, treasuryP2PAddress)
	}

	var temporaryChannelID string
	for provisionAttempt := 1; provisionAttempt <= maxInt(1, p.cfg.OpenChannelRetries); provisionAttempt++ {
		temporaryChannelID, err = openProviderSettlementChannelWithRetry(ctx, treasuryNode, providerPeerID, fundingHex, input.Binding.UDTTypeScript, p.cfg.OpenChannelRetries, p.cfg.PollInterval, reconnect)
		if err != nil {
			return EnsureProviderLiquidityResult{}, fmt.Errorf("open channel: %w", err)
		}
		if err := acceptProviderSettlementChannelWithRetry(ctx, providerNode, temporaryChannelID, acceptFundingHex, p.cfg.AcceptChannelRetries, p.cfg.PollInterval); err != nil {
			if provisionAttempt < maxInt(1, p.cfg.OpenChannelRetries) && isProviderSettlementProvisionRetryable(err) {
				if reconnectErr := reconnect(ctx); reconnectErr != nil {
					return EnsureProviderLiquidityResult{}, reconnectErr
				}
				continue
			}
			return EnsureProviderLiquidityResult{}, fmt.Errorf("accept channel: %w", err)
		}

		if err := waitForProviderSettlementChannelReady(ctx, treasuryNode, providerNode, providerPeerID, treasuryPeerID, temporaryChannelID, acceptFundingHex, input.Binding.UDTTypeScript, p.cfg.PollInterval, p.cfg.AcceptRetryEvery, p.cfg.AcceptChannelRetries); err != nil {
			if provisionAttempt < maxInt(1, p.cfg.OpenChannelRetries) && isProviderSettlementProvisionRetryable(err) {
				if reconnectErr := reconnect(ctx); reconnectErr != nil {
					return EnsureProviderLiquidityResult{}, reconnectErr
				}
				continue
			}
			return EnsureProviderLiquidityResult{}, err
		}
		break
	}

	fundingCents := fundingUnitsToCents(fundingUnits, p.cfg.CentsPerUnit)
	return EnsureProviderLiquidityResult{
		ChannelID:           temporaryChannelID,
		ReuseSource:         ProviderLiquidityReuseNewChannel,
		ReadyChannelCount:   maxInt(countReadyChannels(treasuryChannels, input.Binding.UDTTypeScript)+1, 1),
		TotalSpendableCents: input.CurrentPool.TotalSpendableCents + fundingCents,
	}, nil
}

func newProviderSettlementRawFNNClient(endpoint string) providerSettlementRawFNNClient {
	return providerSettlementRawFNNClient{
		client: &providerSettlementRawRPCClient{
			endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		},
	}
}

func (c providerSettlementRawFNNClient) NodeInfo(ctx context.Context) (providerSettlementNodeInfo, error) {
	var result providerSettlementNodeInfo
	if err := c.client.Call(ctx, "node_info", []any{}, &result); err != nil {
		return providerSettlementNodeInfo{}, err
	}
	return result, nil
}

func (c providerSettlementRawFNNClient) ConnectPeer(ctx context.Context, address string) error {
	var result map[string]any
	return c.client.Call(ctx, "connect_peer", []any{map[string]any{"address": address}}, &result)
}

func (c providerSettlementRawFNNClient) OpenChannel(ctx context.Context, peerID, fundingAmountHex string, udtTypeScript UDTTypeScript) (string, error) {
	var result struct {
		TemporaryChannelID string `json:"temporary_channel_id"`
	}
	args := map[string]any{
		"peer_id":                         peerID,
		"funding_amount":                  fundingAmountHex,
		"tlc_fee_proportional_millionths": "0x0",
	}
	if hasProviderSettlementUDTScript(udtTypeScript) {
		args["funding_udt_type_script"] = map[string]any{
			"code_hash": udtTypeScript.CodeHash,
			"hash_type": udtTypeScript.HashType,
			"args":      udtTypeScript.Args,
		}
	}
	if err := c.client.Call(ctx, "open_channel", []any{args}, &result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.TemporaryChannelID), nil
}

func (c providerSettlementRawFNNClient) AcceptChannel(ctx context.Context, temporaryChannelID, fundingAmountHex string) error {
	var result map[string]any
	return c.client.Call(ctx, "accept_channel", []any{map[string]any{
		"temporary_channel_id": temporaryChannelID,
		"funding_amount":       fundingAmountHex,
	}}, &result)
}

func (c providerSettlementRawFNNClient) ListChannels(ctx context.Context, peerID string) (providerSettlementRawChannelList, error) {
	var result providerSettlementRawChannelList
	if err := c.client.Call(ctx, "list_channels", []any{map[string]any{"peer_id": peerID}}, &result); err != nil {
		return providerSettlementRawChannelList{}, err
	}
	return result, nil
}

func connectProviderSettlementPeers(ctx context.Context, treasuryNode, providerNode providerSettlementRawFNNClient, providerP2PAddress, treasuryP2PAddress string) error {
	if err := treasuryNode.ConnectPeer(ctx, providerP2PAddress); err != nil && !isProviderSettlementConnectRetryableSuccess(err) {
		return fmt.Errorf("connect treasury -> provider: %w", err)
	}
	if err := providerNode.ConnectPeer(ctx, treasuryP2PAddress); err != nil && !isProviderSettlementConnectRetryableSuccess(err) {
		return fmt.Errorf("connect provider -> treasury: %w", err)
	}
	return nil
}

func (c *providerSettlementRawRPCClient) Call(ctx context.Context, method string, params any, target any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("%d", time.Now().UnixNano()),
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var rpc struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&rpc); err != nil {
		return err
	}
	if rpc.Error != nil {
		return fmt.Errorf("%s: %s", method, rpc.Error.Message)
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(rpc.Result, target)
}

func openProviderSettlementChannelWithRetry(ctx context.Context, treasuryNode providerSettlementRawFNNClient, peerID, fundingHex string, udtTypeScript UDTTypeScript, attempts int, delay time.Duration, reconnect func(context.Context) error) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		channelID, err := treasuryNode.OpenChannel(ctx, peerID, fundingHex, udtTypeScript)
		if err == nil {
			return channelID, nil
		}
		lastErr = err
		if !isProviderSettlementOpenChannelRetryable(err) || attempt == attempts {
			break
		}
		if reconnect != nil {
			if reconnectErr := reconnect(ctx); reconnectErr != nil {
				return "", reconnectErr
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", lastErr
}

func acceptProviderSettlementChannelWithRetry(ctx context.Context, providerNode providerSettlementRawFNNClient, temporaryChannelID, fundingHex string, attempts int, delay time.Duration) error {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		err := providerNode.AcceptChannel(ctx, temporaryChannelID, fundingHex)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isProviderSettlementAcceptRetryable(err) || attempt == attempts {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func waitForProviderSettlementChannelReady(ctx context.Context, treasuryNode, providerNode providerSettlementRawFNNClient, providerPeerID, treasuryPeerID, temporaryChannelID, acceptFundingHex string, udtTypeScript UDTTypeScript, pollInterval time.Duration, acceptRetryEvery, acceptRetries int) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	attempt := 0
	for {
		attempt++
		treasuryChannels, err := treasuryNode.ListChannels(ctx, providerPeerID)
		if err != nil {
			return fmt.Errorf("list treasury channels: %w", err)
		}
		providerChannels, err := providerNode.ListChannels(ctx, treasuryPeerID)
		if err != nil {
			return fmt.Errorf("list provider channels: %w", err)
		}
		if _, ok := matchingReadyChannelID(treasuryChannels, providerChannels, udtTypeScript); ok {
			return nil
		}
		if strings.TrimSpace(temporaryChannelID) != "" &&
			strings.TrimSpace(acceptFundingHex) != "" &&
			acceptRetryEvery > 0 &&
			attempt%acceptRetryEvery == 0 &&
			firstProviderSettlementChannelState(providerChannels, udtTypeScript) == "AWAITING_CHANNEL_READY" {
			if err := acceptProviderSettlementChannelWithRetry(ctx, providerNode, temporaryChannelID, acceptFundingHex, acceptRetries, pollInterval); err != nil && !isProviderSettlementAcceptRetryable(err) {
				return fmt.Errorf("re-accept channel: %w", err)
			}
		}

		select {
		case <-ctx.Done():
			return errors.New("timeout waiting provider settlement channel to reach CHANNEL_READY")
		case <-ticker.C:
		}
	}
}

func matchingReadyChannelID(treasuryChannels, providerChannels providerSettlementRawChannelList, udtTypeScript UDTTypeScript) (string, bool) {
	treasuryID := firstReadyProviderSettlementChannelID(treasuryChannels, udtTypeScript)
	providerID := firstReadyProviderSettlementChannelID(providerChannels, udtTypeScript)
	if treasuryID == "" || providerID == "" {
		return "", false
	}
	if treasuryID != "" {
		return treasuryID, true
	}
	return providerID, true
}

func providerSettlementAcceptFundingHex(info providerSettlementNodeInfo, udtTypeScript UDTTypeScript) string {
	if hasProviderSettlementUDTScript(udtTypeScript) {
		for _, cfg := range info.UDTCfgInfos {
			if providerSettlementUDTScriptsEqual(cfg.Script, udtTypeScript) {
				return strings.TrimSpace(cfg.AutoAcceptAmount)
			}
		}
	}
	return strings.TrimSpace(info.AutoAcceptChannelCKBFundingAmount)
}

func providerSettlementUDTScriptsEqual(left providerSettlementRawUDTTypeScript, right UDTTypeScript) bool {
	return strings.TrimSpace(left.CodeHash) == strings.TrimSpace(right.CodeHash) &&
		strings.TrimSpace(left.HashType) == strings.TrimSpace(right.HashType) &&
		strings.TrimSpace(left.Args) == strings.TrimSpace(right.Args)
}

func firstReadyProviderSettlementChannelID(channels providerSettlementRawChannelList, udtTypeScript UDTTypeScript) string {
	for _, channel := range channels.Channels {
		if normalizeProviderSettlementChannelState(channel.State) != "CHANNEL_READY" {
			continue
		}
		if hasProviderSettlementUDTScript(udtTypeScript) && !providerSettlementUDTScriptsMatch(channel.FundingUDTTypeScript, udtTypeScript) {
			continue
		}
		return strings.TrimSpace(channel.ChannelID)
	}
	return ""
}

func firstProviderSettlementChannelState(channels providerSettlementRawChannelList, udtTypeScript UDTTypeScript) string {
	for _, channel := range channels.Channels {
		if hasProviderSettlementUDTScript(udtTypeScript) && !providerSettlementUDTScriptsMatch(channel.FundingUDTTypeScript, udtTypeScript) {
			continue
		}
		return normalizeProviderSettlementChannelState(channel.State)
	}
	return ""
}

func countReadyChannels(channels providerSettlementRawChannelList, udtTypeScript UDTTypeScript) int {
	count := 0
	for _, channel := range channels.Channels {
		if normalizeProviderSettlementChannelState(channel.State) != "CHANNEL_READY" {
			continue
		}
		if hasProviderSettlementUDTScript(udtTypeScript) && !providerSettlementUDTScriptsMatch(channel.FundingUDTTypeScript, udtTypeScript) {
			continue
		}
		count++
	}
	return count
}

func normalizeProviderSettlementChannelState(state any) string {
	switch typed := state.(type) {
	case string:
		return strings.ToUpper(strings.TrimSpace(typed))
	case map[string]any:
		if name, ok := typed["state_name"].(string); ok {
			return strings.ToUpper(strings.TrimSpace(name))
		}
		if name, ok := typed["state"].(string); ok {
			return strings.ToUpper(strings.TrimSpace(name))
		}
	}
	return ""
}

func providerSettlementUDTScriptsMatch(left providerSettlementRawUDTTypeScript, right UDTTypeScript) bool {
	return strings.EqualFold(strings.TrimSpace(left.CodeHash), strings.TrimSpace(right.CodeHash)) &&
		strings.EqualFold(strings.TrimSpace(left.HashType), strings.TrimSpace(right.HashType)) &&
		strings.EqualFold(strings.TrimSpace(left.Args), strings.TrimSpace(right.Args))
}

func hasProviderSettlementUDTScript(script UDTTypeScript) bool {
	return strings.TrimSpace(script.CodeHash) != "" &&
		strings.TrimSpace(script.HashType) != "" &&
		strings.TrimSpace(script.Args) != ""
}

func providerSettlementMultiaddrForP2P(host string, port int, peerID string) string {
	return fmt.Sprintf("/dns4/%s/tcp/%d/p2p/%s", host, port, peerID)
}

func providerSettlementNormalizeP2PAddress(ctx context.Context, address string) (string, error) {
	trimmed := strings.TrimSpace(address)
	if !strings.HasPrefix(trimmed, "/dns4/") {
		return trimmed, nil
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) < 7 || parts[1] != "dns4" || parts[3] != "tcp" || parts[5] != "p2p" {
		return "", fmt.Errorf("unsupported dns4 multiaddr %q", address)
	}
	ip, err := providerSettlementResolveIPv4(ctx, parts[2])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", ip, parts[4], parts[6]), nil
}

func providerSettlementResolveIPv4(ctx context.Context, host string) (string, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, strings.TrimSpace(host))
	if err != nil {
		return "", fmt.Errorf("resolve host %q: %w", host, err)
	}
	for _, address := range addresses {
		if ipv4 := address.IP.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}
	return "", fmt.Errorf("resolve host %q: no ipv4 address found", host)
}

func neededReserveToFundingUnits(neededReserveCents, centsPerUnit, minFundingUnits int64) int64 {
	if neededReserveCents <= 0 {
		return minFundingUnits
	}
	if centsPerUnit <= 0 {
		centsPerUnit = defaultProviderSettlementCentsPerUnit
	}
	units := neededReserveCents / centsPerUnit
	if neededReserveCents%centsPerUnit != 0 {
		units++
	}
	if units < minFundingUnits {
		return minFundingUnits
	}
	return units
}

func fundingUnitsToCents(fundingUnits, centsPerUnit int64) int64 {
	if fundingUnits <= 0 {
		return 0
	}
	if centsPerUnit <= 0 {
		centsPerUnit = defaultProviderSettlementCentsPerUnit
	}
	return fundingUnits * centsPerUnit
}

func providerSettlementDecimalToHexQuantity(value int64) (string, error) {
	if value <= 0 {
		return "", errors.New("amount must be positive")
	}
	return fmt.Sprintf("0x%s", big.NewInt(value).Text(16)), nil
}

func isProviderSettlementOpenChannelRetryable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "peer not found") ||
		strings.Contains(message, "peer not ready") ||
		strings.Contains(message, "temporarily unavailable") ||
		strings.Contains(message, "waiting for peer to send init message")
}

func isProviderSettlementConnectRetryableSuccess(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already connected") ||
		strings.Contains(message, "already exists")
}

func isProviderSettlementAcceptRetryable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no channel with temp id") ||
		strings.Contains(message, "temporary_channel_id") && strings.Contains(message, "not found")
}

func isProviderSettlementProvisionRetryable(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return isProviderSettlementAcceptRetryable(err) ||
		strings.Contains(message, "timeout waiting provider settlement channel to reach channel_ready") ||
		strings.Contains(message, "failed to fund channel") ||
		strings.Contains(message, "error decoding response body")
}

func deriveProviderSettlementPeerIDFromNodeID(nodeID string) (string, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(nodeID), "0x")
	pubkey, err := hex.DecodeString(trimmed)
	if err != nil {
		return "", fmt.Errorf("decode node id: %w", err)
	}
	if len(pubkey) != 33 {
		return "", fmt.Errorf("expected 33-byte compressed pubkey, got %d bytes", len(pubkey))
	}
	digest := sha256.Sum256(pubkey)
	raw := append([]byte{0x12, 0x20}, digest[:]...)
	return encodeProviderSettlementBase58(raw), nil
}

func encodeProviderSettlementBase58(data []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	number := new(big.Int).SetBytes(data)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)
	encoded := make([]byte, 0, len(data)*2)
	for number.Cmp(zero) > 0 {
		number.DivMod(number, base, mod)
		encoded = append(encoded, alphabet[mod.Int64()])
	}
	for _, b := range data {
		if b != 0 {
			break
		}
		encoded = append(encoded, alphabet[0])
	}
	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}
	if len(encoded) == 0 {
		return string(alphabet[0])
	}
	return string(encoded)
}

func envIntOrDefault(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envInt64OrDefault(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envDurationMillisOrDefault(key string, fallback time.Duration) time.Duration {
	return time.Duration(envIntOrDefault(key, int(fallback/time.Millisecond))) * time.Millisecond
}

func envDurationSecondsOrDefault(key string, fallback time.Duration) time.Duration {
	return time.Duration(envIntOrDefault(key, int(fallback/time.Second))) * time.Second
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
