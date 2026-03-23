package settlement

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

	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
	"github.com/nervosnetwork/ckb-sdk-go/v2/address"
	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/secp256k1"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/v2/systemscript"
	"github.com/nervosnetwork/ckb-sdk-go/v2/transaction/signer"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

const (
	defaultBuyerDepositCKBQueryPageLimit = 100
	defaultBuyerDepositCKBQueryMaxPages  = 20
	defaultBuyerDepositCKBFeeRate        = 1000
	defaultBuyerDepositKeyRetryLimit     = 256
)

type CKBBuyerDepositWalletConfig struct {
	RPCURL               string
	MasterSeed           string
	Network              types.Network
	UDTTypeScript        platform.UDTTypeScript
	UDTCellDepTxHash     string
	UDTCellDepIndex      uint32
	QueryPageLimit       int
	QueryMaxPages        int
	ConfirmationBlocks   uint64
	RawUnitsPerWholeUSDI int64
}

type ckbBuyerDepositWallet struct {
	cfg       CKBBuyerDepositWalletConfig
	rawClient *buyerDepositCKBRPCClient
}

type buyerDepositCKBRPCClient struct {
	endpoint   string
	httpClient *http.Client
}

type buyerDepositCKBCell struct {
	BlockNumber string            `json:"block_number"`
	OutPoint    *types.OutPoint   `json:"out_point"`
	Output      *types.CellOutput `json:"output"`
	OutputData  string            `json:"output_data"`
}

func NewCKBBuyerDepositWallet(cfg CKBBuyerDepositWalletConfig) (BuyerDepositWallet, error) {
	if strings.TrimSpace(cfg.RPCURL) == "" {
		return nil, errors.New("buyer deposit ckb rpc url is required")
	}
	if strings.TrimSpace(cfg.MasterSeed) == "" {
		return nil, errors.New("buyer deposit wallet master seed is required")
	}
	if strings.TrimSpace(cfg.UDTTypeScript.CodeHash) == "" || strings.TrimSpace(cfg.UDTTypeScript.HashType) == "" || strings.TrimSpace(cfg.UDTTypeScript.Args) == "" {
		return nil, errors.New("buyer deposit udt type script is required")
	}
	if strings.TrimSpace(cfg.UDTCellDepTxHash) == "" {
		return nil, errors.New("buyer deposit udt cell dep tx hash is required")
	}
	if cfg.Network != types.NetworkMain && cfg.Network != types.NetworkTest && cfg.Network != types.NetworkPreview {
		cfg.Network = types.NetworkTest
	}
	if cfg.QueryPageLimit <= 0 {
		cfg.QueryPageLimit = defaultBuyerDepositCKBQueryPageLimit
	}
	if cfg.QueryMaxPages <= 0 {
		cfg.QueryMaxPages = defaultBuyerDepositCKBQueryMaxPages
	}
	if cfg.RawUnitsPerWholeUSDI <= 0 {
		cfg.RawUnitsPerWholeUSDI = defaultBuyerDepositRawUnitsPerWholeUSDI
	}
	return &ckbBuyerDepositWallet{
		cfg: cfg,
		rawClient: &buyerDepositCKBRPCClient{
			endpoint:   strings.TrimRight(strings.TrimSpace(cfg.RPCURL), "/"),
			httpClient: &http.Client{Timeout: 20 * time.Second},
		},
	}, nil
}

func NewBuyerDepositServiceFromEnvE(funding FundingRecordRepository) (*BuyerDepositService, error) {
	wallet, options, err := loadBuyerDepositServiceOptionsFromEnv()
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, nil
	}
	addresses, sweeps, err := loadBuyerDepositRepositoriesOrMemory()
	if err != nil {
		return nil, err
	}
	options.Addresses = addresses
	options.Sweeps = sweeps
	options.Funding = funding
	options.Wallet = wallet
	return NewBuyerDepositService(options), nil
}

func loadBuyerDepositServiceOptionsFromEnv() (BuyerDepositWallet, BuyerDepositServiceOptions, error) {
	if !buyerDepositEnvEnabled() {
		return nil, BuyerDepositServiceOptions{}, nil
	}

	rawUnitsPerWhole := envInt64WithFallback("BUYER_DEPOSIT_RAW_UNITS_PER_WHOLE_USDI", defaultBuyerDepositRawUnitsPerWholeUSDI)
	confirmationBlocks := envUint64WithFallback("BUYER_DEPOSIT_CONFIRMATION_BLOCKS", 24)
	minSweepRaw := rawUnitsPerWhole * 10
	if raw := strings.TrimSpace(os.Getenv("BUYER_DEPOSIT_MIN_USDI")); raw != "" {
		minSweepRaw = (parseAmountToCents64(raw) * rawUnitsPerWhole) / 100
	}

	udtTypeScriptRaw := firstNonEmptyEnv("BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON", "FIBER_USDI_UDT_TYPE_SCRIPT_JSON")
	if strings.TrimSpace(udtTypeScriptRaw) == "" {
		if runtimeconfig.RequireExternalDependencies() {
			return nil, BuyerDepositServiceOptions{}, errors.New("BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON or FIBER_USDI_UDT_TYPE_SCRIPT_JSON is required")
		}
		return nil, BuyerDepositServiceOptions{}, nil
	}
	var udtTypeScript platform.UDTTypeScript
	if err := json.Unmarshal([]byte(udtTypeScriptRaw), &udtTypeScript); err != nil {
		return nil, BuyerDepositServiceOptions{}, fmt.Errorf("parse buyer deposit udt type script: %w", err)
	}

	wallet, err := NewCKBBuyerDepositWallet(CKBBuyerDepositWalletConfig{
		RPCURL:               firstNonEmptyEnv("BUYER_DEPOSIT_CKB_RPC_URL", "FNN_CKB_RPC_URL", "FNN2_CKB_RPC_URL"),
		MasterSeed:           firstNonEmptyEnv("BUYER_DEPOSIT_WALLET_MASTER_SEED", "BUYER_DEPOSIT_WALLET_SEED"),
		Network:              envCKBNetwork("BUYER_DEPOSIT_CKB_NETWORK", types.NetworkTest),
		UDTTypeScript:        udtTypeScript,
		UDTCellDepTxHash:     strings.TrimSpace(os.Getenv("BUYER_DEPOSIT_UDT_CELL_DEP_TX_HASH")),
		UDTCellDepIndex:      uint32(envIntWithFallback("BUYER_DEPOSIT_UDT_CELL_DEP_INDEX", 0)),
		QueryPageLimit:       envIntWithFallback("BUYER_DEPOSIT_CKB_QUERY_PAGE_LIMIT", defaultBuyerDepositCKBQueryPageLimit),
		QueryMaxPages:        envIntWithFallback("BUYER_DEPOSIT_CKB_QUERY_MAX_PAGES", defaultBuyerDepositCKBQueryMaxPages),
		ConfirmationBlocks:   confirmationBlocks,
		RawUnitsPerWholeUSDI: rawUnitsPerWhole,
	})
	if err != nil {
		return nil, BuyerDepositServiceOptions{}, err
	}

	treasuryAddress := strings.TrimSpace(os.Getenv("BUYER_DEPOSIT_TREASURY_ADDRESS"))
	if treasuryAddress == "" {
		return nil, BuyerDepositServiceOptions{}, errors.New("BUYER_DEPOSIT_TREASURY_ADDRESS is required")
	}

	return wallet, BuyerDepositServiceOptions{
		Asset:                defaultSettlementAsset(firstNonEmptyEnv("BUYER_DEPOSIT_ASSET", "MARKETPLACE_SETTLEMENT_ASSET")),
		TreasuryAddress:      treasuryAddress,
		MinSweepAmountRaw:    minSweepRaw,
		ConfirmationBlocks:   confirmationBlocks,
		RawUnitsPerWholeUSDI: rawUnitsPerWhole,
	}, nil
}

func buyerDepositEnvEnabled() bool {
	if envBoolTruthy("BUYER_DEPOSIT_ENABLE") {
		return true
	}
	return firstNonEmptyEnv("BUYER_DEPOSIT_WALLET_MASTER_SEED", "BUYER_DEPOSIT_WALLET_SEED") != ""
}

func (w *ckbBuyerDepositWallet) DeriveAddress(index int) (string, error) {
	key, err := w.deriveKey(index)
	if err != nil {
		return "", err
	}
	script := systemscript.Secp256K1Blake160SignhashAll(key)
	return address.Address{Script: script, Network: w.cfg.Network}.Encode()
}

func (w *ckbBuyerDepositWallet) QueryBalance(ctx context.Context, record BuyerDepositAddress) (BuyerDepositChainBalance, error) {
	lockScript, err := w.lockScriptForRecord(record)
	if err != nil {
		return BuyerDepositChainBalance{}, err
	}
	tipNumber, err := w.rawClient.tipBlockNumber(ctx)
	if err != nil {
		return BuyerDepositChainBalance{}, err
	}
	cells, err := w.rawClient.listSUDTCells(ctx, lockScript, w.cfg.UDTTypeScript, w.cfg.QueryPageLimit, w.cfg.QueryMaxPages)
	if err != nil {
		return BuyerDepositChainBalance{}, err
	}
	onChain := int64(0)
	confirmed := int64(0)
	for _, cell := range cells {
		amount := parseXUDTAmount(cell.OutputData)
		onChain += amount
		if w.isCellConfirmed(tipNumber, cell.BlockNumber) {
			confirmed += amount
		}
	}
	return BuyerDepositChainBalance{
		Address:            record.Address,
		RawOnChainUnits:    onChain,
		RawConfirmedUnits:  confirmed,
		ConfirmationBlocks: w.cfg.ConfirmationBlocks,
	}, nil
}

func (w *ckbBuyerDepositWallet) SweepToTreasury(ctx context.Context, record BuyerDepositAddress, treasuryAddress string, confirmationBlocks uint64) (BuyerDepositSweepResult, error) {
	if strings.TrimSpace(treasuryAddress) == "" {
		return BuyerDepositSweepResult{}, errors.New("buyer deposit treasury address is required")
	}

	key, err := w.deriveKey(record.DerivationIndex)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	senderAddress, err := w.DeriveAddress(record.DerivationIndex)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(senderAddress), strings.TrimSpace(record.Address)) {
		return BuyerDepositSweepResult{}, errors.New("buyer deposit address derivation mismatch")
	}

	lockScript, err := w.lockScriptForRecord(record)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	tipNumber, err := w.rawClient.tipBlockNumber(ctx)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	cells, err := w.rawClient.listSUDTCells(ctx, lockScript, w.cfg.UDTTypeScript, w.cfg.QueryPageLimit, w.cfg.QueryMaxPages)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	requiredConfirmations := w.cfg.ConfirmationBlocks
	if confirmationBlocks > 0 {
		requiredConfirmations = confirmationBlocks
	}
	confirmedCells := make([]buyerDepositCKBCell, 0, len(cells))
	confirmedRawUnits := int64(0)
	totalInputCapacity := uint64(0)
	for _, cell := range cells {
		if cell.OutPoint == nil || cell.Output == nil {
			continue
		}
		if requiredConfirmations > 0 && !isBuyerDepositCellConfirmed(tipNumber, cell.BlockNumber, requiredConfirmations) {
			continue
		}
		amount := parseXUDTAmount(cell.OutputData)
		if amount <= 0 {
			continue
		}
		confirmedCells = append(confirmedCells, cell)
		confirmedRawUnits += amount
		totalInputCapacity += cell.Output.Capacity
	}
	if confirmedRawUnits <= 0 || len(confirmedCells) == 0 {
		return BuyerDepositSweepResult{TreasuryAddress: treasuryAddress}, nil
	}

	client, err := rpc.Dial(w.cfg.RPCURL)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	defer client.Close()

	senderAddr, err := address.Decode(senderAddress)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	treasuryAddr, err := address.Decode(treasuryAddress)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}

	udtArgs, err := decodeHexBytes(w.cfg.UDTTypeScript.Args)
	if err != nil {
		return BuyerDepositSweepResult{}, fmt.Errorf("buyer deposit udt args: %w", err)
	}
	udtScript := &types.Script{
		CodeHash: types.HexToHash(w.cfg.UDTTypeScript.CodeHash),
		HashType: parseCKBHashType(w.cfg.UDTTypeScript.HashType),
		Args:     udtArgs,
	}

	inputs := make([]*types.CellInput, 0, len(confirmedCells))
	for _, cell := range confirmedCells {
		inputs = append(inputs, &types.CellInput{
			Since:          0,
			PreviousOutput: cell.OutPoint,
		})
	}

	treasuryOutputData := systemscript.EncodeSudtAmount(big.NewInt(confirmedRawUnits))
	treasuryOutput := &types.CellOutput{
		Lock: treasuryAddr.Script,
		Type: udtScript,
	}
	treasuryOutput.Capacity = treasuryOutput.OccupiedCapacity(treasuryOutputData)

	changeOutput := &types.CellOutput{
		Lock: senderAddr.Script,
	}
	changeOutputData := []byte{}
	changeOccupied := changeOutput.OccupiedCapacity(changeOutputData)

	secpInfo := systemscript.GetInfo(w.cfg.Network, systemscript.Secp256k1Blake160SighashAll)
	if secpInfo == nil || secpInfo.OutPoint == nil {
		return BuyerDepositSweepResult{}, errors.New("buyer deposit secp256k1 sighash-all dep is unavailable for configured network")
	}
	cellDeps := []*types.CellDep{
		{
			OutPoint: secpInfo.OutPoint,
			DepType:  secpInfo.DepType,
		},
		{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash(w.cfg.UDTCellDepTxHash),
				Index:  w.cfg.UDTCellDepIndex,
			},
			DepType: types.DepTypeCode,
		},
	}

	tx := &types.Transaction{
		Version:     0,
		CellDeps:    cellDeps,
		HeaderDeps:  []types.Hash{},
		Inputs:      inputs,
		Outputs:     []*types.CellOutput{treasuryOutput, changeOutput},
		OutputsData: [][]byte{treasuryOutputData, changeOutputData},
		Witnesses:   make([][]byte, len(inputs)),
	}

	witnessPlaceholder := (&types.WitnessArgs{Lock: make([]byte, 65)}).Serialize()
	if len(tx.Witnesses) == 0 {
		return BuyerDepositSweepResult{}, errors.New("buyer deposit sweep requires at least one input")
	}
	tx.Witnesses[0] = witnessPlaceholder
	for index := 1; index < len(tx.Witnesses); index++ {
		tx.Witnesses[index] = []byte{}
	}

	fee := tx.CalculateFee(defaultBuyerDepositCKBFeeRate)
	switch {
	case totalInputCapacity >= treasuryOutput.Capacity+fee+changeOccupied:
		changeOutput.Capacity = totalInputCapacity - treasuryOutput.Capacity - fee
	case totalInputCapacity >= treasuryOutput.Capacity+fee:
		tx.Outputs = []*types.CellOutput{treasuryOutput}
		tx.OutputsData = [][]byte{treasuryOutputData}
	default:
		return BuyerDepositSweepResult{}, errors.New("buyer deposit sweep: insufficient ckb capacity to sweep confirmed usdi")
	}

	inputGroup := make([]int, 0, len(inputs))
	for index := range inputs {
		inputGroup = append(inputGroup, index)
	}
	signature, err := signer.SignTransaction(tx, inputGroup, tx.Witnesses[0], key)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	tx.Witnesses[0] = (&types.WitnessArgs{Lock: signature}).Serialize()

	txHash, err := client.SendTransaction(ctx, tx)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	return BuyerDepositSweepResult{
		SweepTxHash:     txHash.Hex(),
		SweptRawUnits:   confirmedRawUnits,
		TreasuryAddress: treasuryAddress,
	}, nil
}

func (w *ckbBuyerDepositWallet) deriveKey(index int) (*secp256k1.Secp256k1Key, error) {
	seed := strings.TrimSpace(w.cfg.MasterSeed)
	for attempt := 0; attempt < defaultBuyerDepositKeyRetryLimit; attempt++ {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", seed, index, attempt)))
		key, err := secp256k1.ToKey(hash[:])
		if err == nil {
			return key, nil
		}
	}
	return nil, fmt.Errorf("unable to derive valid buyer deposit key for index %d", index)
}

func (w *ckbBuyerDepositWallet) lockScriptForRecord(record BuyerDepositAddress) (platform.UDTTypeScript, error) {
	addr, err := address.Decode(record.Address)
	if err != nil {
		return platform.UDTTypeScript{}, err
	}
	return platform.UDTTypeScript{
		CodeHash: addr.Script.CodeHash.Hex(),
		HashType: string(addr.Script.HashType),
		Args:     "0x" + hex.EncodeToString(addr.Script.Args),
	}, nil
}

func (w *ckbBuyerDepositWallet) isCellConfirmed(tipNumber uint64, blockNumber string) bool {
	if w.cfg.ConfirmationBlocks == 0 {
		return true
	}
	return isBuyerDepositCellConfirmed(tipNumber, blockNumber, w.cfg.ConfirmationBlocks)
}

func isBuyerDepositCellConfirmed(tipNumber uint64, blockNumber string, confirmationBlocks uint64) bool {
	if confirmationBlocks == 0 {
		return true
	}
	height := parseHexUint64(blockNumber)
	if height == 0 {
		return false
	}
	return tipNumber >= height+confirmationBlocks
}

func (c *buyerDepositCKBRPCClient) Call(ctx context.Context, method string, params any, target any) error {
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

	var rpcResponse struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&rpcResponse); err != nil {
		return err
	}
	if rpcResponse.Error != nil {
		return fmt.Errorf("%s: %s", method, rpcResponse.Error.Message)
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(rpcResponse.Result, target)
}

func (c *buyerDepositCKBRPCClient) tipBlockNumber(ctx context.Context) (uint64, error) {
	var result string
	if err := c.Call(ctx, "get_tip_block_number", []any{}, &result); err != nil {
		return 0, err
	}
	return parseHexUint64(result), nil
}

func (c *buyerDepositCKBRPCClient) listSUDTCells(ctx context.Context, lockScript platform.UDTTypeScript, typeScript platform.UDTTypeScript, pageLimit, maxPages int) ([]buyerDepositCKBCell, error) {
	cells := make([]buyerDepositCKBCell, 0)
	cursor := "0x"
	for page := 0; page < maxPages; page++ {
		params := []any{map[string]any{
			"script": map[string]string{
				"code_hash": lockScript.CodeHash,
				"hash_type": lockScript.HashType,
				"args":      lockScript.Args,
			},
			"script_type": "lock",
			"filter": map[string]any{
				"script": map[string]string{
					"code_hash": typeScript.CodeHash,
					"hash_type": typeScript.HashType,
					"args":      typeScript.Args,
				},
			},
		}, "asc", fmt.Sprintf("0x%x", pageLimit)}
		if cursor != "0x" {
			params = append(params, cursor)
		}
		var result struct {
			Objects    []buyerDepositCKBCell `json:"objects"`
			LastCursor string                `json:"last_cursor"`
		}
		if err := c.Call(ctx, "get_cells", params, &result); err != nil {
			return nil, err
		}
		cells = append(cells, result.Objects...)
		if len(result.Objects) == 0 || strings.TrimSpace(result.LastCursor) == "" || result.LastCursor == cursor {
			break
		}
		cursor = result.LastCursor
	}
	return cells, nil
}

func parseCKBHashType(raw string) types.ScriptHashType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "data":
		return types.HashTypeData
	case "data1":
		return types.HashTypeData1
	case "data2":
		return types.HashTypeData2
	default:
		return types.HashTypeType
	}
}

func parseHexUint64(raw string) uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if strings.HasPrefix(strings.ToLower(raw), "0x") {
		value, err := strconv.ParseUint(raw[2:], 16, 64)
		if err != nil {
			return 0
		}
		return value
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

func decodeHexBytes(raw string) ([]byte, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(raw, "0x"), "0X"))
	if raw == "" {
		return nil, nil
	}
	return hex.DecodeString(raw)
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

func envIntWithFallback(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64WithFallback(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envUint64WithFallback(key string, fallback uint64) uint64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBoolTruthy(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func envCKBNetwork(key string, fallback types.Network) types.Network {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "main", "mainnet":
		return types.NetworkMain
	case "preview":
		return types.NetworkPreview
	case "test", "testnet":
		return types.NetworkTest
	default:
		return fallback
	}
}
