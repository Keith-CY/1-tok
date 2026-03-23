package settlement

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/chenyu/1-tok/internal/platform"
	"github.com/nervosnetwork/ckb-sdk-go/v2/types"
)

func TestLoadBuyerDepositServiceOptionsFromEnvResolvesTreasuryAddressFromRPC(t *testing.T) {
	const (
		seed           = "buyer-deposit-test-seed"
		udtCellDepHash = "0xaec423c2af7fe844b476333190096b10fc5726e6d9ac58a9b71f71ffac204fee"
		lockArgs       = "0x1111111111111111111111111111111111111111"
	)
	treasuryScript := &types.Script{
		CodeHash: types.HexToHash("0x9bd7e06f3ecf4be0f2fcd2188b23f1b9fcc88e5d4b65a8637b17723bbda3cce8"),
		HashType: types.HashTypeType,
		Args:     mustBuyerDepositHexBytes(t, lockArgs),
	}
	expectedAddress, err := encodeBuyerDepositAddress(treasuryScript, types.NetworkTest)
	if err != nil {
		t.Fatalf("encode expected address: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":"1","result":{"default_funding_lock_script":{"code_hash":"%s","hash_type":"type","args":"%s"}}}`, treasuryScript.CodeHash.Hex(), lockArgs)
	}))
	defer server.Close()

	restore := setBuyerDepositEnv(t, map[string]string{
		"BUYER_DEPOSIT_ENABLE":               "true",
		"BUYER_DEPOSIT_WALLET_MASTER_SEED":   seed,
		"BUYER_DEPOSIT_CKB_RPC_URL":          "https://testnet.ckb.dev/",
		"BUYER_DEPOSIT_CKB_NETWORK":          "testnet",
		"BUYER_DEPOSIT_TREASURY_RPC_URL":     server.URL,
		"BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON": `{"code_hash":"0xcc9dc33ef234e14bc788c43a4848556a5fb16401a04662fc55db9bb201987037","hash_type":"type","args":"0x71fd1985b2971a9903e4d8ed0d59e6710166985217ca0681437883837b86162f"}`,
		"BUYER_DEPOSIT_UDT_CELL_DEP_TX_HASH": udtCellDepHash,
		"BUYER_DEPOSIT_UDT_CELL_DEP_INDEX":   "0",
	})
	defer restore()

	wallet, options, err := loadBuyerDepositServiceOptionsFromEnv()
	if err != nil {
		t.Fatalf("load buyer deposit options: %v", err)
	}
	if wallet == nil {
		t.Fatal("expected buyer deposit wallet")
	}
	if options.TreasuryAddress != expectedAddress {
		t.Fatalf("treasury address = %q, want %q", options.TreasuryAddress, expectedAddress)
	}
}

func setBuyerDepositEnv(t *testing.T, values map[string]string) func() {
	t.Helper()
	keys := []string{
		"BUYER_DEPOSIT_ENABLE",
		"BUYER_DEPOSIT_WALLET_MASTER_SEED",
		"BUYER_DEPOSIT_WALLET_SEED",
		"BUYER_DEPOSIT_CKB_RPC_URL",
		"BUYER_DEPOSIT_CKB_NETWORK",
		"BUYER_DEPOSIT_TREASURY_ADDRESS",
		"BUYER_DEPOSIT_TREASURY_RPC_URL",
		"BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON",
		"BUYER_DEPOSIT_UDT_CELL_DEP_TX_HASH",
		"BUYER_DEPOSIT_UDT_CELL_DEP_INDEX",
	}
	original := make(map[string]*string, len(keys))
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if !ok {
			original[key] = nil
			_ = os.Unsetenv(key)
			continue
		}
		copied := value
		original[key] = &copied
	}
	for _, key := range keys {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	return func() {
		for _, key := range keys {
			if original[key] == nil {
				_ = os.Unsetenv(key)
				continue
			}
			_ = os.Setenv(key, *original[key])
		}
	}
}

func mustBuyerDepositHexBytes(t *testing.T, raw string) []byte {
	t.Helper()
	out, err := decodeHexBytes(raw)
	if err != nil {
		t.Fatalf("decode hex bytes %q: %v", raw, err)
	}
	return out
}

func TestCKBBuyerDepositWalletSweepToTreasuryUsesPureCKBCellsForFees(t *testing.T) {
	const (
		seed           = "buyer-deposit-sweep-test-seed"
		udtCellDepHash = "0xaec423c2af7fe844b476333190096b10fc5726e6d9ac58a9b71f71ffac204fee"
		rawUnits       = int64(1_000_000_000)
	)

	udtTypeScript := platform.UDTTypeScript{
		CodeHash: "0xcc9dc33ef234e14bc788c43a4848556a5fb16401a04662fc55db9bb201987037",
		HashType: "type",
		Args:     "0x71fd1985b2971a9903e4d8ed0d59e6710166985217ca0681437883837b86162f",
	}
	treasuryScript := &types.Script{
		CodeHash: types.HexToHash("0x9bd7e06f3ecf4be0f2fcd2188b23f1b9fcc88e5d4b65a8637b17723bbda3cce8"),
		HashType: types.HashTypeType,
		Args:     mustBuyerDepositHexBytes(t, "0x3333333333333333333333333333333333333333"),
	}
	treasuryAddress, err := encodeBuyerDepositAddress(treasuryScript, types.NetworkTest)
	if err != nil {
		t.Fatalf("encode treasury address: %v", err)
	}

	walletIface, err := NewCKBBuyerDepositWallet(CKBBuyerDepositWalletConfig{
		RPCURL:               "http://127.0.0.1",
		MasterSeed:           seed,
		Network:              types.NetworkTest,
		UDTTypeScript:        udtTypeScript,
		UDTCellDepTxHash:     udtCellDepHash,
		UDTCellDepIndex:      0,
		QueryPageLimit:       100,
		QueryMaxPages:        20,
		ConfirmationBlocks:   24,
		RawUnitsPerWholeUSDI: defaultBuyerDepositRawUnitsPerWholeUSDI,
	})
	if err != nil {
		t.Fatalf("new buyer deposit wallet: %v", err)
	}
	wallet := walletIface.(*ckbBuyerDepositWallet)
	key, err := wallet.deriveKey(0)
	if err != nil {
		t.Fatalf("derive buyer deposit key: %v", err)
	}

	depositAddress, err := wallet.DeriveAddress(0)
	if err != nil {
		t.Fatalf("derive deposit address: %v", err)
	}
	senderScript, err := decodeBuyerDepositAddressScript(depositAddress)
	if err != nil {
		t.Fatalf("decode sender script: %v", err)
	}
	lockScript, err := wallet.lockScriptForRecord(BuyerDepositAddress{
		BuyerOrgID:      "buyer_1",
		Asset:           "USDI",
		Address:         depositAddress,
		DerivationIndex: 0,
	})
	if err != nil {
		t.Fatalf("lock script for record: %v", err)
	}
	udtArgs, err := decodeHexBytes(udtTypeScript.Args)
	if err != nil {
		t.Fatalf("decode udt args: %v", err)
	}
	udtScript := &types.Script{
		CodeHash: types.HexToHash(udtTypeScript.CodeHash),
		HashType: parseCKBHashType(udtTypeScript.HashType),
		Args:     udtArgs,
	}
	treasuryOutputData := encodeBuyerDepositSudtAmount(big.NewInt(rawUnits))
	sudtCellCapacity := (&types.CellOutput{
		Lock: treasuryScript,
		Type: udtScript,
	}).OccupiedCapacity(treasuryOutputData)
	sudtCell := buyerDepositCKBCell{
		BlockNumber: "0x1",
		OutPoint: &types.OutPoint{
			TxHash: types.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
			Index:  0,
		},
		Output: &types.CellOutput{
			Capacity: sudtCellCapacity,
			Lock:     senderScript,
			Type:     udtScript,
		},
		OutputData: "0x00ca9a3b000000000000000000000000",
	}
	pureCKBCell := buyerDepositCKBCell{
		BlockNumber: "0x1",
		OutPoint: &types.OutPoint{
			TxHash: types.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"),
			Index:  0,
		},
		Output: &types.CellOutput{
			Capacity: 1_000_000_000,
			Lock:     senderScript,
		},
		OutputData: "0x",
	}

	var sendCalls int
	var sentInputCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode rpc request: %v", err)
		}
		switch req.Method {
		case "get_tip_block_number":
			_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":"1","result":"0x100"}`)
		case "get_cells":
			filtered := strings.Contains(string(req.Params[0]), `"filter"`)
			result := struct {
				Objects    []buyerDepositCKBCell `json:"objects"`
				LastCursor string                `json:"last_cursor"`
			}{LastCursor: "0x"}
			if filtered {
				result.Objects = []buyerDepositCKBCell{sudtCell}
			} else {
				result.Objects = []buyerDepositCKBCell{pureCKBCell}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "1",
				"result":  result,
			})
		case "send_transaction":
			sendCalls++
			var tx types.Transaction
			if err := json.Unmarshal(req.Params[0], &tx); err != nil {
				t.Fatalf("decode send_transaction payload: %v", err)
			}
			sentInputCount = len(tx.Inputs)
			if len(tx.Witnesses) == 0 {
				t.Fatal("expected at least one witness")
			}
			witnessArgs, err := types.DeserializeWitnessArgs(tx.Witnesses[0])
			if err != nil {
				t.Fatalf("deserialize witness args: %v", err)
			}
			placeholder := &types.WitnessArgs{
				Lock:       make([]byte, 65),
				InputType:  witnessArgs.InputType,
				OutputType: witnessArgs.OutputType,
			}
			tx.Witnesses[0] = placeholder.Serialize()
			inputGroup := make([]int, 0, len(tx.Inputs))
			for index := range tx.Inputs {
				inputGroup = append(inputGroup, index)
			}
			expectedSignature, err := signBuyerDepositTransaction(&tx, inputGroup, tx.Witnesses[0], key)
			if err != nil {
				t.Fatalf("sign transaction: %v", err)
			}
			if got := hex.EncodeToString(witnessArgs.Lock); got != hex.EncodeToString(expectedSignature) {
				t.Fatalf("witness lock signature mismatch\n got: %s\nwant: %s", got, hex.EncodeToString(expectedSignature))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "1",
				"result":  "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			})
		default:
			t.Fatalf("unexpected rpc method %s", req.Method)
		}
	}))
	defer server.Close()
	wallet.rawClient.endpoint = server.URL
	wallet.cfg.RPCURL = server.URL

	result, err := wallet.SweepToTreasury(context.Background(), BuyerDepositAddress{
		BuyerOrgID:      "buyer_1",
		Asset:           "USDI",
		Address:         depositAddress,
		DerivationIndex: 0,
	}, treasuryAddress, 24)
	if err != nil {
		t.Fatalf("sweep to treasury: %v", err)
	}
	if sendCalls != 1 {
		t.Fatalf("send transaction calls = %d, want 1", sendCalls)
	}
	if sentInputCount != 2 {
		t.Fatalf("send transaction input count = %d, want 2", sentInputCount)
	}
	if result.SweptRawUnits != rawUnits {
		t.Fatalf("swept raw units = %d, want %d", result.SweptRawUnits, rawUnits)
	}
	if result.TreasuryAddress != treasuryAddress {
		t.Fatalf("treasury address = %q, want %q", result.TreasuryAddress, treasuryAddress)
	}
	if result.SweepTxHash == "" {
		t.Fatal("expected sweep tx hash")
	}
	if _, err := wallet.rawClient.listPureCKBCells(context.Background(), lockScript, 10, 1); err != nil {
		t.Fatalf("list pure ckb cells: %v", err)
	}
}
