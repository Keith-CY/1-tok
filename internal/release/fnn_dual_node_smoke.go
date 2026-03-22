package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultChannelFeeProportionalMillionths = "0x4B0"

type FNNDualNodeSmokeConfig struct {
	InvoiceRPCURL       string
	PayerRPCURL         string
	InvoiceP2PHost      string
	PayerP2PHost        string
	P2PPort             int
	FundingAmount       string
	AcceptFundingAmount string
	PollInterval        time.Duration
	WaitTimeout         time.Duration
	OpenChannelRetries  int
	Adapter             FNNAdapterSmokeConfig
}

type FNNDualNodeSmokeSummary struct {
	InvoicePeerID      string                 `json:"invoicePeerId"`
	PayerPeerID        string                 `json:"payerPeerId"`
	ChannelTemporaryID string                 `json:"channelTemporaryId"`
	InvoiceAddress     string                 `json:"invoiceAddress"`
	PayerAddress       string                 `json:"payerAddress"`
	Adapter            FNNAdapterSmokeSummary `json:"adapter"`
}

func FNNDualNodeSmokeConfigFromEnv() FNNDualNodeSmokeConfig {
	adapter := FNNAdapterSmokeConfigFromEnv()
	adapter.IncludePayment = true

	return FNNDualNodeSmokeConfig{
		InvoiceRPCURL:       envOrDefault("RELEASE_FNN_DUAL_INVOICE_RPC_URL", "http://fnn:8227"),
		PayerRPCURL:         envOrDefault("RELEASE_FNN_DUAL_PAYER_RPC_URL", "http://fnn2:8227"),
		InvoiceP2PHost:      envOrDefault("RELEASE_FNN_DUAL_INVOICE_P2P_HOST", "fnn"),
		PayerP2PHost:        envOrDefault("RELEASE_FNN_DUAL_PAYER_P2P_HOST", "fnn2"),
		P2PPort:             envIntOrDefault("RELEASE_FNN_DUAL_P2P_PORT", 8228),
		FundingAmount:       envOrDefault("RELEASE_FNN_DUAL_FUNDING_AMOUNT", "10000000000"),
		AcceptFundingAmount: strings.TrimSpace(envOrDefault("RELEASE_FNN_DUAL_ACCEPT_FUNDING_AMOUNT", "")),
		PollInterval:        envDurationMillisOrDefault("RELEASE_FNN_DUAL_POLL_INTERVAL_MILLISECONDS", 2000),
		WaitTimeout:         envDurationSecondsOrDefault("RELEASE_FNN_DUAL_WAIT_TIMEOUT_SECONDS", 90),
		OpenChannelRetries:  envIntOrDefault("RELEASE_FNN_DUAL_OPEN_CHANNEL_RETRIES", 10),
		Adapter:             adapter,
	}
}

func RunFNNDualNodeSmoke(ctx context.Context, cfg FNNDualNodeSmokeConfig) (FNNDualNodeSmokeSummary, error) {
	if strings.TrimSpace(cfg.InvoiceRPCURL) == "" {
		return FNNDualNodeSmokeSummary{}, errors.New("invoice rpc url is required")
	}
	if strings.TrimSpace(cfg.PayerRPCURL) == "" {
		return FNNDualNodeSmokeSummary{}, errors.New("payer rpc url is required")
	}
	if strings.TrimSpace(cfg.InvoiceP2PHost) == "" {
		cfg.InvoiceP2PHost = "fnn"
	}
	if strings.TrimSpace(cfg.PayerP2PHost) == "" {
		cfg.PayerP2PHost = "fnn2"
	}
	if cfg.P2PPort == 0 {
		cfg.P2PPort = 8228
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.WaitTimeout <= 0 {
		cfg.WaitTimeout = 90 * time.Second
	}
	if cfg.OpenChannelRetries <= 0 {
		cfg.OpenChannelRetries = 10
	}
	if strings.TrimSpace(cfg.FundingAmount) == "" {
		cfg.FundingAmount = "10000000000"
	}

	invoiceNode := newReleaseRawFNNClient(cfg.InvoiceRPCURL)
	payerNode := newReleaseRawFNNClient(cfg.PayerRPCURL)

	invoiceInfo, err := invoiceNode.NodeInfo(ctx)
	if err != nil {
		return FNNDualNodeSmokeSummary{}, fmt.Errorf("invoice node info: %w", err)
	}
	payerInfo, err := payerNode.NodeInfo(ctx)
	if err != nil {
		return FNNDualNodeSmokeSummary{}, fmt.Errorf("payer node info: %w", err)
	}

	invoicePeerID, err := derivePeerIDFromNodeID(invoiceInfo.NodeID)
	if err != nil {
		return FNNDualNodeSmokeSummary{}, fmt.Errorf("derive invoice peer id: %w", err)
	}
	payerPeerID, err := derivePeerIDFromNodeID(payerInfo.NodeID)
	if err != nil {
		return FNNDualNodeSmokeSummary{}, fmt.Errorf("derive payer peer id: %w", err)
	}

	invoiceAddress := multiaddrForP2P(cfg.InvoiceP2PHost, cfg.P2PPort, invoicePeerID)
	payerAddress := multiaddrForP2P(cfg.PayerP2PHost, cfg.P2PPort, payerPeerID)
	if err := connectDualNodePeers(ctx, payerNode, invoiceNode, invoiceAddress, payerAddress); err != nil {
		return FNNDualNodeSmokeSummary{}, err
	}

	fundingAmountHex, err := releaseHexQuantity(cfg.FundingAmount)
	if err != nil {
		return FNNDualNodeSmokeSummary{}, fmt.Errorf("funding amount: %w", err)
	}
	acceptFundingHex := strings.TrimSpace(cfg.AcceptFundingAmount)
	if acceptFundingHex == "" {
		acceptFundingHex = strings.TrimSpace(invoiceInfo.AutoAcceptChannelCKBFundingAmount)
	}
	if acceptFundingHex == "" {
		acceptFundingHex = fundingAmountHex
	} else if !strings.HasPrefix(strings.ToLower(acceptFundingHex), "0x") {
		acceptFundingHex, err = releaseHexQuantity(acceptFundingHex)
		if err != nil {
			return FNNDualNodeSmokeSummary{}, fmt.Errorf("accept funding amount: %w", err)
		}
	}

	temporaryChannelID, err := openChannelWithRetry(ctx, payerNode, invoicePeerID, fundingAmountHex, cfg.OpenChannelRetries, cfg.PollInterval, func(ctx context.Context) error {
		return connectDualNodePeers(ctx, payerNode, invoiceNode, invoiceAddress, payerAddress)
	})
	if err != nil {
		return FNNDualNodeSmokeSummary{}, fmt.Errorf("open channel: %w", err)
	}
	if strings.TrimSpace(temporaryChannelID) == "" {
		return FNNDualNodeSmokeSummary{}, errors.New("open channel returned empty temporary channel id")
	}
	if err := acceptChannelWithRetry(ctx, invoiceNode, temporaryChannelID, acceptFundingHex, cfg.OpenChannelRetries, cfg.PollInterval); err != nil {
		return FNNDualNodeSmokeSummary{}, fmt.Errorf("accept channel: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, cfg.WaitTimeout)
	defer cancel()
	if err := waitForChannelReady(waitCtx, invoiceNode, payerNode, invoicePeerID, payerPeerID, cfg.PollInterval); err != nil {
		return FNNDualNodeSmokeSummary{}, err
	}

	adapterSummary := FNNAdapterSmokeSummary{}
	if strings.TrimSpace(cfg.Adapter.BaseURL) != "" {
		adapterSummary, err = RunFNNAdapterSmoke(ctx, cfg.Adapter)
		if err != nil {
			return FNNDualNodeSmokeSummary{}, fmt.Errorf("adapter smoke: %w", err)
		}
	}

	return FNNDualNodeSmokeSummary{
		InvoicePeerID:      invoicePeerID,
		PayerPeerID:        payerPeerID,
		ChannelTemporaryID: temporaryChannelID,
		InvoiceAddress:     invoiceAddress,
		PayerAddress:       payerAddress,
		Adapter:            adapterSummary,
	}, nil
}

func writeFNNDualNodeArtifact(summary FNNDualNodeSmokeSummary) error {
	return WriteJSONArtifact(os.Getenv("RELEASE_FNN_DUAL_OUTPUT_PATH"), summary)
}

type releaseRawFNNClient struct {
	client *rawFNNRPCClient
}

type rawNodeInfo struct {
	NodeID                            string `json:"node_id"`
	AutoAcceptChannelCKBFundingAmount string `json:"auto_accept_channel_ckb_funding_amount"`
}

type rawChannelList struct {
	Channels []rawChannelState `json:"channels"`
}

type rawChannelState struct {
	State   any  `json:"state"`
	Enabled bool `json:"enabled"`
}

func newReleaseRawFNNClient(endpoint string) releaseRawFNNClient {
	return releaseRawFNNClient{
		client: &rawFNNRPCClient{
			endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		},
	}
}

func (c releaseRawFNNClient) NodeInfo(ctx context.Context) (rawNodeInfo, error) {
	var result rawNodeInfo
	if err := c.client.Call(ctx, "node_info", []any{}, &result); err != nil {
		return rawNodeInfo{}, err
	}
	return result, nil
}

func (c releaseRawFNNClient) ConnectPeer(ctx context.Context, address string) error {
	var result map[string]any
	return c.client.Call(ctx, "connect_peer", []any{map[string]any{"address": address}}, &result)
}

func (c releaseRawFNNClient) OpenChannel(ctx context.Context, peerID, fundingAmountHex string) (string, error) {
	var result struct {
		TemporaryChannelID string `json:"temporary_channel_id"`
	}
	if err := c.client.Call(ctx, "open_channel", []any{map[string]any{
		"peer_id":                         peerID,
		"funding_amount":                  fundingAmountHex,
		"tlc_fee_proportional_millionths": defaultChannelFeeProportionalMillionths,
	}}, &result); err != nil {
		return "", err
	}
	return result.TemporaryChannelID, nil
}

func (c releaseRawFNNClient) CreateInvoice(ctx context.Context, asset, amount string, udtTypeScript map[string]string) (string, error) {
	hexAmount, err := releaseHexQuantity(amount)
	if err != nil {
		return "", err
	}
	params := map[string]any{
		"amount":   hexAmount,
		"currency": releaseMapAssetToCurrency(asset),
	}
	if strings.EqualFold(strings.TrimSpace(asset), "USDI") {
		params["udt_type_script"] = map[string]any{
			"code_hash": udtTypeScript["code_hash"],
			"hash_type": udtTypeScript["hash_type"],
			"args":      udtTypeScript["args"],
		}
	}
	var result struct {
		InvoiceAddress string `json:"invoice_address"`
	}
	if err := c.client.Call(ctx, "new_invoice", []any{params}, &result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.InvoiceAddress), nil
}

func (c releaseRawFNNClient) AcceptChannel(ctx context.Context, temporaryChannelID, fundingAmountHex string) error {
	var result map[string]any
	return c.client.Call(ctx, "accept_channel", []any{map[string]any{
		"temporary_channel_id": temporaryChannelID,
		"funding_amount":       fundingAmountHex,
	}}, &result)
}

func (c releaseRawFNNClient) ListChannels(ctx context.Context, peerID string) (rawChannelList, error) {
	var result rawChannelList
	if err := c.client.Call(ctx, "list_channels", []any{map[string]any{"peer_id": peerID}}, &result); err != nil {
		return rawChannelList{}, err
	}
	return result, nil
}

func connectDualNodePeers(ctx context.Context, payerNode, invoiceNode releaseRawFNNClient, invoiceAddress, payerAddress string) error {
	if err := payerNode.ConnectPeer(ctx, invoiceAddress); err != nil && !isDualNodeAlreadyConnected(err) {
		return fmt.Errorf("connect payer -> invoice: %w", err)
	}
	if err := invoiceNode.ConnectPeer(ctx, payerAddress); err != nil && !isDualNodeAlreadyConnected(err) {
		return fmt.Errorf("connect invoice -> payer: %w", err)
	}
	return nil
}

func releaseMapAssetToCurrency(asset string) string {
	if strings.EqualFold(strings.TrimSpace(asset), "CKB") {
		if scoped := strings.TrimSpace(os.Getenv("FIBER_INVOICE_CURRENCY_CKB")); scoped != "" {
			return scoped
		}
		if global := strings.TrimSpace(os.Getenv("FIBER_INVOICE_CURRENCY")); global != "" {
			return global
		}
		return "Fibt"
	}
	if scoped := strings.TrimSpace(os.Getenv("FIBER_INVOICE_CURRENCY_USDI")); scoped != "" {
		return scoped
	}
	return releaseMapAssetToCurrency("CKB")
}

func waitForChannelReady(ctx context.Context, invoiceNode, payerNode releaseRawFNNClient, invoicePeerID, payerPeerID string, pollInterval time.Duration) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		payerChannels, err := payerNode.ListChannels(ctx, invoicePeerID)
		if err != nil {
			return fmt.Errorf("list payer channels: %w", err)
		}
		invoiceChannels, err := invoiceNode.ListChannels(ctx, payerPeerID)
		if err != nil {
			return fmt.Errorf("list invoice channels: %w", err)
		}
		if firstChannelReady(payerChannels) && firstChannelReady(invoiceChannels) {
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.New("timeout waiting dual fnn channel to reach CHANNEL_READY")
		case <-ticker.C:
		}
	}
}

func openChannelWithRetry(ctx context.Context, payerNode releaseRawFNNClient, peerID, fundingAmountHex string, attempts int, delay time.Duration, reconnect func(context.Context) error) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		temporaryChannelID, err := payerNode.OpenChannel(ctx, peerID, fundingAmountHex)
		if err == nil {
			return temporaryChannelID, nil
		}
		lastErr = err
		if !isPeerInitPendingError(err) || attempt == attempts {
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

func isPeerInitPendingError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "waiting for peer to send init message") ||
		(strings.Contains(message, "feature not found") && strings.Contains(message, "peer"))
}

func isDualNodeAlreadyConnected(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already connected") || strings.Contains(message, "already exists")
}

func acceptChannelWithRetry(ctx context.Context, invoiceNode releaseRawFNNClient, temporaryChannelID, fundingAmountHex string, attempts int, delay time.Duration) error {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		err := invoiceNode.AcceptChannel(ctx, temporaryChannelID, fundingAmountHex)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isAcceptChannelPendingError(err) || attempt == attempts {
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

func isAcceptChannelPendingError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no channel with temp id") ||
		(strings.Contains(message, "temporary_channel_id") && strings.Contains(message, "not found"))
}

func firstChannelReady(channels rawChannelList) bool {
	if len(channels.Channels) == 0 {
		return false
	}
	return normalizeRawChannelState(channels.Channels[0].State) == "CHANNEL_READY"
}

func normalizeRawChannelState(state any) string {
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

func multiaddrForP2P(host string, port int, peerID string) string {
	return fmt.Sprintf("/dns4/%s/tcp/%d/p2p/%s", host, port, peerID)
}

func releaseHexQuantity(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("amount is required")
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "0x") {
		return strings.ToLower(trimmed), nil
	}
	if strings.Contains(trimmed, ".") {
		parts := strings.SplitN(trimmed, ".", 2)
		if len(parts) != 2 || strings.Trim(parts[1], "0") != "" {
			return "", fmt.Errorf("invalid positive integer amount %q", value)
		}
		trimmed = parts[0]
	}
	amount, ok := new(big.Int).SetString(trimmed, 10)
	if !ok || amount.Sign() <= 0 {
		return "", fmt.Errorf("invalid positive integer amount %q", value)
	}
	return fmt.Sprintf("0x%s", amount.Text(16)), nil
}

func derivePeerIDFromNodeID(nodeID string) (string, error) {
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
	return encodeBase58(raw), nil
}

func encodeBase58(data []byte) string {
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

func envDurationMillisOrDefault(key string, fallbackMillis int) time.Duration {
	return time.Duration(envIntOrDefault(key, fallbackMillis)) * time.Millisecond
}

func envDurationSecondsOrDefault(key string, fallbackSeconds int) time.Duration {
	return time.Duration(envIntOrDefault(key, fallbackSeconds)) * time.Second
}

type rawFNNRPCClient struct {
	endpoint   string
	httpClient *http.Client
}

func (c *rawFNNRPCClient) Call(ctx context.Context, method string, params any, target any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID(),
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
			Code    int    `json:"code"`
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
