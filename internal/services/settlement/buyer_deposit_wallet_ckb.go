package settlement

import (
	"bytes"
	"context"
	"encoding/binary"
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

	"github.com/btcsuite/btcd/btcec"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/bech32"
	"github.com/nervosnetwork/ckb-sdk-go/v2/crypto/blake2b"
	"github.com/nervosnetwork/ckb-sdk-go/v2/rpc"
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

type buyerDepositSecp256k1Key struct {
	privateKey *btcec.PrivateKey
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
	network := envCKBNetwork("BUYER_DEPOSIT_CKB_NETWORK", types.NetworkTest)
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
	udtTypeScript, err := parseBuyerDepositUDTTypeScriptJSON(udtTypeScriptRaw)
	if err != nil {
		return nil, BuyerDepositServiceOptions{}, fmt.Errorf("parse buyer deposit udt type script: %w", err)
	}

	wallet, err := NewCKBBuyerDepositWallet(CKBBuyerDepositWalletConfig{
		RPCURL:               firstNonEmptyEnv("BUYER_DEPOSIT_CKB_RPC_URL", "FNN_CKB_RPC_URL", "FNN2_CKB_RPC_URL"),
		MasterSeed:           firstNonEmptyEnv("BUYER_DEPOSIT_WALLET_MASTER_SEED", "BUYER_DEPOSIT_WALLET_SEED"),
		Network:              network,
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

	treasuryAddress, err := resolveBuyerDepositTreasuryAddress(network)
	if err != nil {
		return nil, BuyerDepositServiceOptions{}, err
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

func resolveBuyerDepositTreasuryAddress(network types.Network) (string, error) {
	if treasuryAddress := strings.TrimSpace(os.Getenv("BUYER_DEPOSIT_TREASURY_ADDRESS")); treasuryAddress != "" {
		return treasuryAddress, nil
	}
	rpcURL := firstNonEmptyEnv("BUYER_DEPOSIT_TREASURY_RPC_URL", "PROVIDER_SETTLEMENT_FNN_TREASURY_RPC_URL", "FNN_PAYER_RPC_URL")
	if rpcURL == "" {
		return "", errors.New("BUYER_DEPOSIT_TREASURY_ADDRESS is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	treasuryAddress, err := discoverBuyerDepositTreasuryAddressFromRPC(ctx, rpcURL, network)
	if err != nil {
		return "", fmt.Errorf("resolve buyer deposit treasury address from %s: %w", rpcURL, err)
	}
	return treasuryAddress, nil
}

func parseBuyerDepositUDTTypeScriptJSON(raw string) (platform.UDTTypeScript, error) {
	var payload map[string]any
	trimmed := strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		if strings.Contains(trimmed, `\"`) {
			normalized := strings.ReplaceAll(trimmed, `\"`, `"`)
			if retryErr := json.Unmarshal([]byte(normalized), &payload); retryErr != nil {
				return platform.UDTTypeScript{}, err
			}
		} else {
			return platform.UDTTypeScript{}, err
		}
	}
	readString := func(keys ...string) string {
		for _, key := range keys {
			value, ok := payload[key]
			if !ok {
				continue
			}
			if text, ok := value.(string); ok {
				return strings.TrimSpace(text)
			}
		}
		return ""
	}
	script := platform.UDTTypeScript{
		CodeHash: readString("codeHash", "code_hash"),
		HashType: readString("hashType", "hash_type"),
		Args:     readString("args"),
	}
	if strings.TrimSpace(script.CodeHash) == "" || strings.TrimSpace(script.HashType) == "" || strings.TrimSpace(script.Args) == "" {
		return platform.UDTTypeScript{}, errors.New("missing code hash, hash type, or args")
	}
	return script, nil
}

func (w *ckbBuyerDepositWallet) DeriveAddress(index int) (string, error) {
	key, err := w.deriveKey(index)
	if err != nil {
		return "", err
	}
	script := buyerDepositSecp256k1LockScript(key.PubKeyCompressed())
	return encodeBuyerDepositAddress(script, w.cfg.Network)
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

	senderScript, err := decodeBuyerDepositAddressScript(senderAddress)
	if err != nil {
		return BuyerDepositSweepResult{}, err
	}
	treasuryScript, err := decodeBuyerDepositAddressScript(treasuryAddress)
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

	treasuryOutputData := encodeBuyerDepositSudtAmount(big.NewInt(confirmedRawUnits))
	treasuryOutput := &types.CellOutput{
		Lock: treasuryScript,
		Type: udtScript,
	}
	treasuryOutput.Capacity = treasuryOutput.OccupiedCapacity(treasuryOutputData)

	changeOutput := &types.CellOutput{
		Lock: senderScript,
	}
	changeOutputData := []byte{}
	changeOccupied := changeOutput.OccupiedCapacity(changeOutputData)

	secpDep, err := buyerDepositSecpCellDep(w.cfg.Network)
	if err != nil {
		return BuyerDepositSweepResult{}, errors.New("buyer deposit secp256k1 sighash-all dep is unavailable for configured network")
	}
	cellDeps := []*types.CellDep{
		secpDep,
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
	if totalInputCapacity < treasuryOutput.Capacity+fee+changeOccupied {
		feeCells, err := w.rawClient.listPureCKBCells(ctx, lockScript, w.cfg.QueryPageLimit, w.cfg.QueryMaxPages)
		if err != nil {
			return BuyerDepositSweepResult{}, err
		}
		existingInputs := make(map[string]struct{}, len(tx.Inputs))
		for _, input := range tx.Inputs {
			if input == nil || input.PreviousOutput == nil {
				continue
			}
			existingInputs[buyerDepositOutPointKey(input.PreviousOutput)] = struct{}{}
		}
		for _, cell := range feeCells {
			if cell.OutPoint == nil || cell.Output == nil {
				continue
			}
			if _, exists := existingInputs[buyerDepositOutPointKey(cell.OutPoint)]; exists {
				continue
			}
			tx.Inputs = append(tx.Inputs, &types.CellInput{
				Since:          0,
				PreviousOutput: cell.OutPoint,
			})
			tx.Witnesses = append(tx.Witnesses, []byte{})
			existingInputs[buyerDepositOutPointKey(cell.OutPoint)] = struct{}{}
			totalInputCapacity += cell.Output.Capacity
			fee = tx.CalculateFee(defaultBuyerDepositCKBFeeRate)
			if totalInputCapacity >= treasuryOutput.Capacity+fee+changeOccupied {
				break
			}
		}
	}
	switch {
	case totalInputCapacity >= treasuryOutput.Capacity+fee+changeOccupied:
		changeOutput.Capacity = totalInputCapacity - treasuryOutput.Capacity - fee
	case totalInputCapacity >= treasuryOutput.Capacity+fee:
		tx.Outputs = []*types.CellOutput{treasuryOutput}
		tx.OutputsData = [][]byte{treasuryOutputData}
	default:
		return BuyerDepositSweepResult{}, errors.New("buyer deposit sweep: insufficient ckb capacity to sweep confirmed usdi")
	}

	inputGroup := make([]int, 0, len(tx.Inputs))
	for index := range tx.Inputs {
		inputGroup = append(inputGroup, index)
	}
	signature, err := signBuyerDepositTransaction(tx, inputGroup, tx.Witnesses[0], key)
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

func buyerDepositOutPointKey(outPoint *types.OutPoint) string {
	if outPoint == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", outPoint.TxHash.Hex(), outPoint.Index)
}

func (w *ckbBuyerDepositWallet) deriveKey(index int) (*buyerDepositSecp256k1Key, error) {
	seed := strings.TrimSpace(w.cfg.MasterSeed)
	for attempt := 0; attempt < defaultBuyerDepositKeyRetryLimit; attempt++ {
		hash := blake2b.Blake256([]byte(fmt.Sprintf("%s:%d:%d", seed, index, attempt)))
		scalar := new(big.Int).SetBytes(hash)
		if scalar.Sign() <= 0 || scalar.Cmp(btcec.S256().N) >= 0 {
			continue
		}
		privateKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), hash)
		return &buyerDepositSecp256k1Key{privateKey: privateKey}, nil
	}
	return nil, fmt.Errorf("unable to derive valid buyer deposit key for index %d", index)
}

func (k *buyerDepositSecp256k1Key) Bytes() []byte {
	if k == nil || k.privateKey == nil {
		return nil
	}
	return k.privateKey.Serialize()
}

func (k *buyerDepositSecp256k1Key) PubKeyCompressed() []byte {
	if k == nil || k.privateKey == nil {
		return nil
	}
	return k.privateKey.PubKey().SerializeCompressed()
}

func (k *buyerDepositSecp256k1Key) Sign(data []byte) ([]byte, error) {
	if k == nil || k.privateKey == nil {
		return nil, errors.New("buyer deposit key is required")
	}
	compact, err := btcec.SignCompact(btcec.S256(), k.privateKey, data, true)
	if err != nil {
		return nil, err
	}
	if len(compact) != 65 {
		return nil, fmt.Errorf("unexpected compact signature length %d", len(compact))
	}
	recoveryID := compact[0] - 27
	if recoveryID >= 4 {
		recoveryID -= 4
	}
	signature := append([]byte{}, compact[1:]...)
	signature = append(signature, recoveryID)
	return signature, nil
}

func signBuyerDepositTransaction(tx *types.Transaction, group []int, witnessPlaceholder []byte, key *buyerDepositSecp256k1Key) ([]byte, error) {
	inputsLen := len(tx.Inputs)
	for i := 0; i < len(group); i++ {
		if i > 0 && group[i] <= group[i-1] {
			return nil, fmt.Errorf("group index is not in ascending order")
		}
		if group[i] > inputsLen {
			return nil, fmt.Errorf("group index %d is greater than input length %d", group[i], inputsLen)
		}
	}

	msg := tx.ComputeHash().Bytes()
	lengthBuffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(lengthBuffer, uint64(len(witnessPlaceholder)))
	msg = append(msg, lengthBuffer...)
	msg = append(msg, witnessPlaceholder...)

	indexes := make([]int, 0, len(group)+len(tx.Witnesses))
	for i := 1; i < len(group); i++ {
		indexes = append(indexes, group[i])
	}
	for i := inputsLen; i < len(tx.Witnesses); i++ {
		indexes = append(indexes, i)
	}
	for _, witnessIndex := range indexes {
		witness := tx.Witnesses[witnessIndex]
		witnessLength := make([]byte, 8)
		binary.LittleEndian.PutUint64(witnessLength, uint64(len(witness)))
		msg = append(msg, witnessLength...)
		msg = append(msg, witness...)
	}

	return key.Sign(blake2b.Blake256(msg))
}

func buyerDepositSecp256k1LockScript(compressedPubKey []byte) *types.Script {
	return &types.Script{
		CodeHash: types.HexToHash("0x9bd7e06f3ecf4be0f2fcd2188b23f1b9fcc88e5d4b65a8637b17723bbda3cce8"),
		HashType: types.HashTypeType,
		Args:     blake2b.Blake160(compressedPubKey),
	}
}

func buyerDepositSecpCellDep(network types.Network) (*types.CellDep, error) {
	switch network {
	case types.NetworkMain:
		return &types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash("0x71a7ba8fc96349fea0ed3a5c47992e3b4084b031a42264a018e0072e8172e46c"),
				Index:  0,
			},
			DepType: types.DepTypeDepGroup,
		}, nil
	case types.NetworkTest, types.NetworkPreview:
		return &types.CellDep{
			OutPoint: &types.OutPoint{
				TxHash: types.HexToHash("0xf8de3bb47d055cdf460d93a2a6e1b05f7432f9777c8c474abf4eec1d4aee5d37"),
				Index:  0,
			},
			DepType: types.DepTypeDepGroup,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported buyer deposit network %q", network)
	}
}

func encodeBuyerDepositSudtAmount(amount *big.Int) []byte {
	out := make([]byte, 16)
	amount.FillBytes(out)
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out
}

func encodeBuyerDepositAddress(script *types.Script, network types.Network) (string, error) {
	if script == nil {
		return "", errors.New("buyer deposit address script is required")
	}
	payload := []byte{0x00}
	payload = append(payload, script.CodeHash.Bytes()...)
	hashType, err := types.SerializeHashTypeByte(script.HashType)
	if err != nil {
		return "", err
	}
	payload = append(payload, hashType)
	payload = append(payload, script.Args...)
	payload, err = bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		return "", err
	}
	hrp, err := buyerDepositAddressHRP(network)
	if err != nil {
		return "", err
	}
	return bech32.EncodeWithBech32m(hrp, payload)
}

func decodeBuyerDepositAddressScript(encoded string) (*types.Script, error) {
	encoding, hrp, decoded, err := bech32.Decode(encoded)
	if err != nil {
		return nil, err
	}
	network, err := buyerDepositNetworkFromHRP(hrp)
	if err != nil {
		return nil, err
	}
	payload, err := bech32.ConvertBits(decoded, 5, 8, false)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, errors.New("buyer deposit address payload is empty")
	}
	switch payload[0] {
	case 0x00:
		if encoding != bech32.BECH32M {
			return nil, errors.New("payload header 0x00 must use bech32m")
		}
		if len(payload) < 34 {
			return nil, errors.New("buyer deposit full address payload is too short")
		}
		hashType, err := types.DeserializeHashTypeByte(payload[33])
		if err != nil {
			return nil, err
		}
		return &types.Script{
			CodeHash: types.BytesToHash(payload[1:33]),
			HashType: hashType,
			Args:     payload[34:],
		}, nil
	case 0x01:
		if encoding != bech32.BECH32 {
			return nil, errors.New("payload header 0x01 must use bech32")
		}
		if len(payload) < 2 {
			return nil, errors.New("buyer deposit short address payload is too short")
		}
		codeHash, hashType, err := buyerDepositShortCodeHash(network, payload[1])
		if err != nil {
			return nil, err
		}
		return &types.Script{
			CodeHash: codeHash,
			HashType: hashType,
			Args:     payload[2:],
		}, nil
	case 0x02, 0x04:
		if encoding != bech32.BECH32 {
			return nil, errors.New("payload header 0x02 or 0x04 must use bech32")
		}
		if len(payload) < 33 {
			return nil, errors.New("buyer deposit full bech32 payload is too short")
		}
		hashType := types.HashTypeData
		if payload[0] == 0x04 {
			hashType = types.HashTypeType
		}
		return &types.Script{
			CodeHash: types.BytesToHash(payload[1:33]),
			HashType: hashType,
			Args:     payload[33:],
		}, nil
	default:
		return nil, fmt.Errorf("unsupported buyer deposit address payload header 0x%x", payload[0])
	}
}

func buyerDepositAddressHRP(network types.Network) (string, error) {
	switch network {
	case types.NetworkMain:
		return "ckb", nil
	case types.NetworkTest, types.NetworkPreview:
		return "ckt", nil
	default:
		return "", fmt.Errorf("unsupported buyer deposit network %q", network)
	}
}

func buyerDepositNetworkFromHRP(hrp string) (types.Network, error) {
	switch hrp {
	case "ckb":
		return types.NetworkMain, nil
	case "ckt":
		return types.NetworkTest, nil
	default:
		return 0, fmt.Errorf("unsupported buyer deposit address hrp %q", hrp)
	}
}

func buyerDepositShortCodeHash(network types.Network, codeHashIndex byte) (types.Hash, types.ScriptHashType, error) {
	switch codeHashIndex {
	case 0x00:
		return types.HexToHash("0x9bd7e06f3ecf4be0f2fcd2188b23f1b9fcc88e5d4b65a8637b17723bbda3cce8"), types.HashTypeType, nil
	case 0x01:
		return types.HexToHash("0x5c5069eb0857efc65e1bca0c07df34c31663b3622fd3876c876320fc9634e2a8"), types.HashTypeType, nil
	case 0x02:
		if network == types.NetworkMain {
			return types.HexToHash("0xd369597ff47f29fbc0d47d2e3775370d1250b85140c670e4718af712983a2354"), types.HashTypeType, nil
		}
		return types.HexToHash("0x3419a1c09eb2567f6552ee7a8ecffd64155cffe0f1796e6e61ec088d740c1356"), types.HashTypeType, nil
	default:
		return types.Hash{}, "", fmt.Errorf("unsupported buyer deposit short code hash index 0x%x", codeHashIndex)
	}
}

func (w *ckbBuyerDepositWallet) lockScriptForRecord(record BuyerDepositAddress) (platform.UDTTypeScript, error) {
	addr, err := decodeBuyerDepositAddressScript(record.Address)
	if err != nil {
		return platform.UDTTypeScript{}, err
	}
	return platform.UDTTypeScript{
		CodeHash: addr.CodeHash.Hex(),
		HashType: string(addr.HashType),
		Args:     "0x" + hex.EncodeToString(addr.Args),
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

func discoverBuyerDepositTreasuryAddressFromRPC(ctx context.Context, endpoint string, network types.Network) (string, error) {
	client := &buyerDepositCKBRPCClient{
		endpoint:   strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	var result struct {
		DefaultFundingLockScript struct {
			CodeHash string `json:"code_hash"`
			HashType string `json:"hash_type"`
			Args     string `json:"args"`
		} `json:"default_funding_lock_script"`
	}
	if err := client.Call(ctx, "node_info", []any{}, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.DefaultFundingLockScript.CodeHash) == "" || strings.TrimSpace(result.DefaultFundingLockScript.Args) == "" {
		return "", errors.New("node_info missing default_funding_lock_script")
	}
	args, err := decodeHexBytes(result.DefaultFundingLockScript.Args)
	if err != nil {
		return "", fmt.Errorf("decode treasury lock args: %w", err)
	}
	return encodeBuyerDepositAddress(&types.Script{
		CodeHash: types.HexToHash(result.DefaultFundingLockScript.CodeHash),
		HashType: parseCKBHashType(result.DefaultFundingLockScript.HashType),
		Args:     args,
	}, network)
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

func (c *buyerDepositCKBRPCClient) listPureCKBCells(ctx context.Context, lockScript platform.UDTTypeScript, pageLimit, maxPages int) ([]buyerDepositCKBCell, error) {
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
		for _, cell := range result.Objects {
			if cell.Output == nil || cell.Output.Type != nil {
				continue
			}
			cells = append(cells, cell)
		}
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
