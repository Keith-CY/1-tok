package release

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/platform"
	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/bech32"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

var ensureDemoFNNBootstrapFunc = ensureDemoFNNBootstrapLiquidity
var requestUSDIFaucetFunc = requestUSDIFaucet

const (
	demoTopUpPayerMinimumCKBShannons = int64(10_000_000_000)
	demoFaucetAmountCKB              = "100000"
	demoFaucetWait                   = 20 * time.Second
	demoFaucetRetryDelay             = 5 * time.Second
	demoBalancePollInterval          = 5 * time.Second
	demoBalanceWaitTimeout           = 2 * time.Minute
	demoFaucetMaxAttempts            = 2
	demoUSDIFaucetRequestAttempts    = 3
	demoCKBBalanceMaxPages           = 20
	demoCentsPerFNNUnit              = int64(100)
	demoMinFNNFundingUnits           = int64(1)
)

func ensureDemoFNNBootstrapLiquidity(ctx context.Context, cfg DemoRunConfig) error {
	udtRaw := firstNonEmptyString(cfg.USDI.BuyerTopUpUDTTypeScriptJSON, cfg.USDI.ProviderSettlementUDTTypeScriptJSON)
	if strings.TrimSpace(udtRaw) == "" {
		return errors.New("demo fnn bootstrap udt type script json is required")
	}
	udtTypeScript, err := parseProviderSettlementUDTTypeScriptJSON(udtRaw)
	if err != nil {
		return fmt.Errorf("parse demo fnn bootstrap udt type script: %w", err)
	}

	payerRPCURL := firstNonEmptyString(cfg.USDI.PayerRPCURL, "http://fnn2:8227")
	invoiceRPCURL := firstNonEmptyString(cfg.USDI.BuyerTopUpInvoiceRPCURL, "http://fnn:8227")
	providerRPCURL := firstNonEmptyString(cfg.USDI.ProviderSettlementRPCURL, "http://provider-fnn:8227")
	ckbRPCURL := firstNonEmptyString(cfg.USDI.CKBRPCURL, "https://testnet.ckbapp.dev/")

	payerNode := newReleaseRawFNNClient(payerRPCURL)
	invoiceNode := newReleaseRawFNNClient(invoiceRPCURL)
	providerNode := newReleaseRawFNNClient(providerRPCURL)

	payerInfo, err := payerNode.NodeInfo(ctx)
	if err != nil {
		return fmt.Errorf("payer node info: %w", err)
	}
	invoiceInfo, err := invoiceNode.NodeInfo(ctx)
	if err != nil {
		return fmt.Errorf("invoice node info: %w", err)
	}
	providerInfo, err := providerNode.NodeInfo(ctx)
	if err != nil {
		return fmt.Errorf("provider node info: %w", err)
	}

	payerAddress, err := deriveCKBTestnetAddressFromLockArgs(payerInfo.DefaultFundingLockScript.Args)
	if err != nil {
		return fmt.Errorf("derive payer ckb address: %w", err)
	}
	invoiceAddress, err := deriveCKBTestnetAddressFromLockArgs(invoiceInfo.DefaultFundingLockScript.Args)
	if err != nil {
		return fmt.Errorf("derive invoice ckb address: %w", err)
	}
	providerAddress, err := deriveCKBTestnetAddressFromLockArgs(providerInfo.DefaultFundingLockScript.Args)
	if err != nil {
		return fmt.Errorf("derive provider ckb address: %w", err)
	}

	buyerTopUpNeededCents := cfg.Demo.MinBuyerBalanceCents
	if topUpAmountCents := parseDemoAmountToCents(cfg.Demo.BuyerTopUpAmount); topUpAmountCents > buyerTopUpNeededCents {
		buyerTopUpNeededCents = topUpAmountCents
	}
	payerRequiredUSDI := demoNeededReserveToFundingUnits(buyerTopUpNeededCents, demoCentsPerFNNUnit, demoMinFNNFundingUnits)
	if cfg.Demo.MinProviderLiquidityCents > 0 {
		payerRequiredUSDI += demoNeededReserveToFundingUnits(cfg.Demo.MinProviderLiquidityCents, demoCentsPerFNNUnit, demoMinFNNFundingUnits)
	}
	invoiceRequiredUSDI := rawNodeUDTAutoAcceptAmount(invoiceInfo, udtTypeScript)
	providerRequiredUSDI := rawNodeUDTAutoAcceptAmount(providerInfo, udtTypeScript)

	invoiceRequiredCKB := demoMaxInt64(parseHexQuantityToInt64(invoiceInfo.AutoAcceptChannelCKBFundingAmount), 1)
	providerRequiredCKB := demoMaxInt64(parseHexQuantityToInt64(providerInfo.AutoAcceptChannelCKBFundingAmount), 1)

	ckbClient := newReleaseCKBRPCClient(ckbRPCURL)
	httpClient := &http.Client{Timeout: 15 * time.Second}

	if err := ensureCKBBalanceOrRequestFaucet(ctx, ckbClient, httpClient, cfg.USDI, payerAddress, payerInfo.DefaultFundingLockScript, demoTopUpPayerMinimumCKBShannons, "payer-bootstrap"); err != nil {
		return fmt.Errorf("ensure payer ckb: %w", err)
	}
	if err := ensureCKBBalanceOrRequestFaucet(ctx, ckbClient, httpClient, cfg.USDI, invoiceAddress, invoiceInfo.DefaultFundingLockScript, invoiceRequiredCKB, "invoice-bootstrap"); err != nil {
		return fmt.Errorf("ensure invoice ckb: %w", err)
	}
	if err := ensureCKBBalanceOrRequestFaucet(ctx, ckbClient, httpClient, cfg.USDI, providerAddress, providerInfo.DefaultFundingLockScript, providerRequiredCKB, "provider-bootstrap"); err != nil {
		return fmt.Errorf("ensure provider ckb: %w", err)
	}

	if err := ensureUSDIBalanceOrRequestFaucet(ctx, ckbClient, httpClient, cfg.USDI, payerAddress, payerInfo.DefaultFundingLockScript, udtTypeScript, payerRequiredUSDI, "payer-bootstrap"); err != nil {
		return fmt.Errorf("ensure payer usdi: %w", err)
	}
	if invoiceRequiredUSDI > 0 {
		if err := ensureUSDIBalanceOrRequestFaucet(ctx, ckbClient, httpClient, cfg.USDI, invoiceAddress, invoiceInfo.DefaultFundingLockScript, udtTypeScript, invoiceRequiredUSDI, "invoice-bootstrap"); err != nil {
			return fmt.Errorf("ensure invoice usdi: %w", err)
		}
	}
	if providerRequiredUSDI > 0 {
		if err := ensureUSDIBalanceOrRequestFaucet(ctx, ckbClient, httpClient, cfg.USDI, providerAddress, providerInfo.DefaultFundingLockScript, udtTypeScript, providerRequiredUSDI, "provider-bootstrap"); err != nil {
			return fmt.Errorf("ensure provider usdi: %w", err)
		}
	}

	return nil
}

type releaseCKBRPCClient struct {
	endpoint   string
	httpClient *http.Client
}

func newReleaseCKBRPCClient(endpoint string) *releaseCKBRPCClient {
	return &releaseCKBRPCClient{
		endpoint:   strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *releaseCKBRPCClient) Call(ctx context.Context, method string, params any, target any) error {
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

	res, err := c.httpClient.Do(req)
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

func ensureCKBBalanceOrRequestFaucet(ctx context.Context, ckbClient *releaseCKBRPCClient, httpClient *http.Client, cfg USDIMarketplaceE2EConfig, address string, lockScript rawScript, requiredAmount int64, label string) error {
	if requiredAmount <= 0 {
		return nil
	}
	current, err := queryCKBBalanceForLockScript(ctx, ckbClient, lockScript)
	if err == nil && current >= requiredAmount {
		return nil
	}
	if err := requestCKBFaucet(ctx, httpClient, cfg, address, label); err != nil {
		return err
	}
	return waitForCKBBalanceThreshold(ctx, ckbClient, lockScript, requiredAmount)
}

func ensureUSDIBalanceOrRequestFaucet(ctx context.Context, ckbClient *releaseCKBRPCClient, httpClient *http.Client, cfg USDIMarketplaceE2EConfig, address string, lockScript rawScript, typeScript platform.UDTTypeScript, requiredAmount int64, label string) error {
	if requiredAmount <= 0 {
		return nil
	}
	current, err := queryUSDIBalanceForLockScript(ctx, ckbClient, lockScript, typeScript)
	if err == nil && current >= requiredAmount {
		return nil
	}
	var lastErr error
	for attempt := 1; attempt <= demoFaucetMaxAttempts; attempt++ {
		if err := requestUSDIFaucet(ctx, httpClient, cfg, address, label); err != nil {
			lastErr = err
			continue
		}
		if err := waitForUSDIBalanceThreshold(ctx, ckbClient, lockScript, typeScript, requiredAmount); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("usdi balance still below required threshold for %s", label)
}

func requestCKBFaucet(ctx context.Context, httpClient *http.Client, cfg USDIMarketplaceE2EConfig, address, label string) error {
	primaryURL := strings.TrimRight(firstNonEmptyString(cfg.CKBFaucetAPIBase, "https://faucet-api.nervos.org"), "/") + "/claim_events"
	primaryPayload := map[string]any{
		"claim_event": map[string]string{
			"address_hash": address,
			"amount":       demoFaucetAmountCKB,
		},
	}
	if err := postDemoJSON(ctx, httpClient, primaryURL, primaryPayload); err == nil {
		return sleepWithContext(ctx, demoFaucetWait)
	}

	fallbackURL := strings.TrimRight(firstNonEmptyString(cfg.CKBFaucetFallbackAPIBase, "https://ckb-utilities.random-walk.co.jp/api"), "/") + "/faucet"
	if err := postDemoJSON(ctx, httpClient, fallbackURL, map[string]string{
		"address": address,
		"token":   "ckb",
	}); err != nil {
		return fmt.Errorf("request ckb faucet for %s: %w", label, err)
	}
	return sleepWithContext(ctx, demoFaucetWait)
}

func requestUSDIFaucet(ctx context.Context, httpClient *http.Client, cfg USDIMarketplaceE2EConfig, address, label string) error {
	return requestUSDIFaucetWithRetry(ctx, httpClient, cfg, address, label, demoUSDIFaucetRequestAttempts, demoFaucetRetryDelay, demoFaucetWait)
}

func requestUSDIFaucetWithRetry(ctx context.Context, httpClient *http.Client, cfg USDIMarketplaceE2EConfig, address, label string, maxAttempts int, retryDelay, settleWait time.Duration) error {
	url := strings.TrimRight(firstNonEmptyString(cfg.USDIFaucetAPIBase, "https://ckb-utilities.random-walk.co.jp/api"), "/") + "/faucet"
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := postDemoJSON(ctx, httpClient, url, map[string]string{
			"address": address,
			"token":   "usdi",
		}); err != nil {
			lastErr = fmt.Errorf("request usdi faucet for %s: %w", label, err)
			if attempt < maxAttempts && retryDelay > 0 {
				if sleepErr := sleepWithContext(ctx, retryDelay); sleepErr != nil {
					return sleepErr
				}
			}
			continue
		}
		return sleepWithContext(ctx, settleWait)
	}
	return lastErr
}

func postDemoJSON(ctx context.Context, httpClient *http.Client, endpoint string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("status %d", res.StatusCode)
	}
	return nil
}

func waitForCKBBalanceThreshold(ctx context.Context, ckbClient *releaseCKBRPCClient, lockScript rawScript, requiredAmount int64) error {
	deadline := time.Now().Add(demoBalanceWaitTimeout)
	for {
		balance, err := queryCKBBalanceForLockScript(ctx, ckbClient, lockScript)
		if err == nil && balance >= requiredAmount {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("ckb balance still below required threshold (%d < %d)", balance, requiredAmount)
		}
		if err := sleepWithContext(ctx, demoBalancePollInterval); err != nil {
			return err
		}
	}
}

func waitForUSDIBalanceThreshold(ctx context.Context, ckbClient *releaseCKBRPCClient, lockScript rawScript, typeScript platform.UDTTypeScript, requiredAmount int64) error {
	deadline := time.Now().Add(demoBalanceWaitTimeout)
	for {
		balance, err := queryUSDIBalanceForLockScript(ctx, ckbClient, lockScript, typeScript)
		if err == nil && balance >= requiredAmount {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return err
			}
			return fmt.Errorf("usdi balance still below required threshold (%d < %d)", balance, requiredAmount)
		}
		if err := sleepWithContext(ctx, demoBalancePollInterval); err != nil {
			return err
		}
	}
}

func queryCKBBalanceForLockScript(ctx context.Context, ckbClient *releaseCKBRPCClient, lockScript rawScript) (int64, error) {
	var total int64
	cursor := "0x"
	for page := 0; page < demoCKBBalanceMaxPages; page++ {
		params := []any{map[string]any{"script": lockScript, "script_type": "lock"}, "asc", "0x64"}
		if cursor != "0x" {
			params = append(params, cursor)
		}
		var result struct {
			Objects []struct {
				Output struct {
					Capacity string `json:"capacity"`
				} `json:"output"`
			} `json:"objects"`
			LastCursor string `json:"last_cursor"`
		}
		if err := ckbClient.Call(ctx, "get_cells", params, &result); err != nil {
			return 0, err
		}
		pageSum := int64(0)
		for _, object := range result.Objects {
			pageSum += parseHexQuantityToInt64(object.Output.Capacity)
		}
		total += pageSum
		if len(result.Objects) == 0 || strings.TrimSpace(result.LastCursor) == "" || result.LastCursor == cursor {
			break
		}
		cursor = result.LastCursor
	}
	return total, nil
}

func queryUSDIBalanceForLockScript(ctx context.Context, ckbClient *releaseCKBRPCClient, lockScript rawScript, typeScript platform.UDTTypeScript) (int64, error) {
	var total int64
	cursor := "0x"
	for page := 0; page < demoCKBBalanceMaxPages; page++ {
		params := []any{map[string]any{
			"script":      lockScript,
			"script_type": "lock",
			"filter": map[string]any{
				"script": map[string]string{
					"code_hash": typeScript.CodeHash,
					"hash_type": typeScript.HashType,
					"args":      typeScript.Args,
				},
			},
		}, "asc", "0x64"}
		if cursor != "0x" {
			params = append(params, cursor)
		}
		var result struct {
			Objects []struct {
				OutputData string `json:"output_data"`
			} `json:"objects"`
			LastCursor string `json:"last_cursor"`
		}
		if err := ckbClient.Call(ctx, "get_cells", params, &result); err != nil {
			return 0, err
		}
		for _, object := range result.Objects {
			total += parseXUDTAmount(object.OutputData)
		}
		if len(result.Objects) == 0 || strings.TrimSpace(result.LastCursor) == "" || result.LastCursor == cursor {
			break
		}
		cursor = result.LastCursor
	}
	return total, nil
}

func parseXUDTAmount(outputData string) int64 {
	data := strings.TrimPrefix(strings.TrimSpace(outputData), "0x")
	if len(data) < 32 {
		return 0
	}
	raw, err := hex.DecodeString(data[:32])
	if err != nil {
		return 0
	}
	for left, right := 0, len(raw)-1; left < right; left, right = left+1, right-1 {
		raw[left], raw[right] = raw[right], raw[left]
	}
	value := big.NewInt(0).SetBytes(raw)
	return value.Int64()
}

func rawNodeUDTAutoAcceptAmount(info rawNodeInfo, udtTypeScript platform.UDTTypeScript) int64 {
	for _, cfg := range info.UDTCfgInfos {
		if strings.EqualFold(strings.TrimSpace(cfg.Script.CodeHash), strings.TrimSpace(udtTypeScript.CodeHash)) &&
			strings.EqualFold(strings.TrimSpace(cfg.Script.HashType), strings.TrimSpace(udtTypeScript.HashType)) &&
			strings.EqualFold(strings.TrimSpace(cfg.Script.Args), strings.TrimSpace(udtTypeScript.Args)) {
			return parseHexQuantityToInt64(cfg.AutoAcceptAmount)
		}
	}
	return 0
}

func parseHexQuantityToInt64(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if strings.HasPrefix(strings.ToLower(raw), "0x") {
		value, ok := big.NewInt(0).SetString(raw[2:], 16)
		if !ok {
			return 0
		}
		return value.Int64()
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func deriveCKBTestnetAddressFromLockArgs(lockArgs string) (string, error) {
	lockArgs = strings.TrimSpace(lockArgs)
	if !strings.HasPrefix(strings.ToLower(lockArgs), "0x") {
		return "", errors.New("lock args must be hex")
	}
	argsBytes, err := hex.DecodeString(lockArgs[2:])
	if err != nil {
		return "", err
	}
	if len(argsBytes) != 20 {
		return "", fmt.Errorf("lock args must be 20 bytes, got %d", len(argsBytes))
	}
	payload := append([]byte{0x01, 0x00}, argsBytes...)
	data, err := convertBits(payload, 8, 5, true)
	if err != nil {
		return "", err
	}
	checksum := bech32CreateChecksum("ckt", data)
	combined := append(data, checksum...)
	var builder strings.Builder
	builder.WriteString("ckt1")
	for _, value := range combined {
		if int(value) >= len(bech32Charset) {
			return "", errors.New("bech32 value out of range")
		}
		builder.WriteByte(bech32Charset[value])
	}
	return builder.String(), nil
}

func decodeCKBAddressToRawScript(encoded string) (rawScript, error) {
	encoding, hrp, decoded, err := bech32.Decode(strings.TrimSpace(encoded))
	if err != nil {
		return rawScript{}, err
	}
	network, err := releaseNetworkFromAddressHRP(hrp)
	if err != nil {
		return rawScript{}, err
	}
	payload, err := bech32.ConvertBits(decoded, 5, 8, false)
	if err != nil {
		return rawScript{}, err
	}
	if len(payload) == 0 {
		return rawScript{}, errors.New("ckb address payload is empty")
	}
	switch payload[0] {
	case 0x00:
		if encoding != bech32.BECH32M {
			return rawScript{}, errors.New("payload header 0x00 must use bech32m")
		}
		if len(payload) < 34 {
			return rawScript{}, errors.New("full ckb address payload is too short")
		}
		hashType, err := types.DeserializeHashTypeByte(payload[33])
		if err != nil {
			return rawScript{}, err
		}
		return rawScript{
			CodeHash: "0x" + hex.EncodeToString(payload[1:33]),
			HashType: string(hashType),
			Args:     "0x" + hex.EncodeToString(payload[34:]),
		}, nil
	case 0x01:
		if encoding != bech32.BECH32 {
			return rawScript{}, errors.New("payload header 0x01 must use bech32")
		}
		if len(payload) < 2 {
			return rawScript{}, errors.New("short ckb address payload is too short")
		}
		codeHash, hashType, err := releaseShortCodeHash(network, payload[1])
		if err != nil {
			return rawScript{}, err
		}
		return rawScript{
			CodeHash: codeHash,
			HashType: hashType,
			Args:     "0x" + hex.EncodeToString(payload[2:]),
		}, nil
	case 0x02, 0x04:
		if encoding != bech32.BECH32 {
			return rawScript{}, errors.New("payload header 0x02 or 0x04 must use bech32")
		}
		if len(payload) < 33 {
			return rawScript{}, errors.New("full bech32 ckb address payload is too short")
		}
		hashType := "data"
		if payload[0] == 0x04 {
			hashType = "type"
		}
		return rawScript{
			CodeHash: "0x" + hex.EncodeToString(payload[1:33]),
			HashType: hashType,
			Args:     "0x" + hex.EncodeToString(payload[33:]),
		}, nil
	default:
		return rawScript{}, fmt.Errorf("unsupported ckb address payload header 0x%x", payload[0])
	}
}

func releaseNetworkFromAddressHRP(hrp string) (types.Network, error) {
	switch strings.TrimSpace(hrp) {
	case "ckb":
		return types.NetworkMain, nil
	case "ckt":
		return types.NetworkTest, nil
	default:
		return 0, fmt.Errorf("unsupported ckb address hrp %q", hrp)
	}
}

func releaseShortCodeHash(network types.Network, codeHashIndex byte) (string, string, error) {
	switch codeHashIndex {
	case 0x00:
		return "0x9bd7e06f3ecf4be0f2fcd2188b23f1b9fcc88e5d4b65a8637b17723bbda3cce8", "type", nil
	case 0x01:
		return "0x5c5069eb0857efc65e1bca0c07df34c31663b3622fd3876c876320fc9634e2a8", "type", nil
	case 0x02:
		if network == types.NetworkMain {
			return "0xd369597ff47f29fbc0d47d2e3775370d1250b85140c670e4718af712983a2354", "type", nil
		}
		return "0x3419a1c09eb2567f6552ee7a8ecffd64155cffe0f1796e6e61ec088d740c1356", "type", nil
	default:
		return "", "", fmt.Errorf("unsupported short code hash index 0x%x", codeHashIndex)
	}
}

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

func convertBits(data []byte, fromBits, toBits uint, pad bool) ([]byte, error) {
	var acc int
	var bits uint
	maxv := (1 << toBits) - 1
	ret := make([]byte, 0, len(data)*int(fromBits)/int(toBits))
	for _, value := range data {
		if int(value)>>fromBits != 0 {
			return nil, errors.New("invalid bech32 data range")
		}
		acc = (acc << fromBits) | int(value)
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			ret = append(ret, byte((acc>>bits)&maxv))
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, errors.New("invalid incomplete bech32 group")
	}
	return ret, nil
}

func bech32CreateChecksum(hrp string, data []byte) []byte {
	values := append(bech32HrpExpand(hrp), data...)
	values = append(values, []byte{0, 0, 0, 0, 0, 0}...)
	polymod := bech32Polymod(values) ^ 1
	checksum := make([]byte, 6)
	for i := range checksum {
		checksum[i] = byte((polymod >> (5 * (5 - i))) & 31)
	}
	return checksum
}

func bech32HrpExpand(hrp string) []byte {
	expanded := make([]byte, 0, len(hrp)*2+1)
	for _, char := range hrp {
		expanded = append(expanded, byte(char>>5))
	}
	expanded = append(expanded, 0)
	for _, char := range hrp {
		expanded = append(expanded, byte(char&31))
	}
	return expanded
}

func bech32Polymod(values []byte) int {
	generators := [5]int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1
	for _, value := range values {
		top := chk >> 25
		chk = ((chk & 0x1ffffff) << 5) ^ int(value)
		for i := 0; i < 5; i++ {
			if (top>>i)&1 == 1 {
				chk ^= generators[i]
			}
		}
	}
	return chk
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func demoMaxInt64(left, right int64) int64 {
	if left >= right {
		return left
	}
	return right
}

func demoNeededReserveToFundingUnits(neededReserveCents, centsPerUnit, minFundingUnits int64) int64 {
	if neededReserveCents <= 0 {
		return minFundingUnits
	}
	if centsPerUnit <= 0 {
		centsPerUnit = demoCentsPerFNNUnit
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
