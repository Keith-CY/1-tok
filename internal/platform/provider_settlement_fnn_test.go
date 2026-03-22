package platform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewFNNProviderSettlementProvisionerFromEnvReturnsNilWithoutConfig(t *testing.T) {
	t.Setenv("PROVIDER_SETTLEMENT_FNN_TREASURY_RPC_URL", "")
	t.Setenv("PROVIDER_SETTLEMENT_FNN_TREASURY_P2P_HOST", "")

	provisioner, err := NewFNNProviderSettlementProvisionerFromEnv()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if provisioner != nil {
		t.Fatalf("expected nil provisioner, got %T", provisioner)
	}
}

func TestNewFNNProviderSettlementProvisionerFromEnvRequiresTreasuryP2PHost(t *testing.T) {
	t.Setenv("PROVIDER_SETTLEMENT_FNN_TREASURY_RPC_URL", "http://fnn2:8227")
	t.Setenv("PROVIDER_SETTLEMENT_FNN_TREASURY_P2P_HOST", "")

	_, err := NewFNNProviderSettlementProvisionerFromEnv()
	if err == nil || !strings.Contains(err.Error(), "PROVIDER_SETTLEMENT_FNN_TREASURY_P2P_HOST") {
		t.Fatalf("expected missing treasury p2p host error, got %v", err)
	}
}

func TestFNNProviderSettlementProvisioner_OpensChannelWhenPoolCannotReuse(t *testing.T) {
	const providerPeerID = "QmdWHo7ejoqhoXN56bFAWQtFtewPkrUSMA64Wp9XnB4n13"

	var treasuryOpenPayload map[string]any
	var treasuryMethods []string
	treasuryNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read treasury request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode treasury request: %v", err)
		}
		treasuryMethods = append(treasuryMethods, rpc.Method)
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id": "0x032222222222222222222222222222222222222222222222222222222222222222",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"channel_id": "ch_ready_1",
							"state":      "CHANNEL_READY",
							"enabled":    true,
							"funding_udt_type_script": map[string]any{
								"code_hash": "0xudt",
								"hash_type": "type",
								"args":      "0x01",
							},
						},
					},
				},
			})
		case "open_channel":
			if len(rpc.Params) != 1 {
				t.Fatalf("unexpected open_channel params: %+v", rpc.Params)
			}
			arg, ok := rpc.Params[0].(map[string]any)
			if !ok {
				t.Fatalf("unexpected open_channel arg type: %T", rpc.Params[0])
			}
			treasuryOpenPayload = arg
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"temporary_channel_id": "tmp_provider_1",
				},
			})
		default:
			t.Fatalf("unexpected treasury method %q", rpc.Method)
		}
	}))
	defer treasuryNode.Close()

	var providerAcceptPayload map[string]any
	var providerMethods []string
	providerNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read provider request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		providerMethods = append(providerMethods, rpc.Method)
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id":                                "0x021111111111111111111111111111111111111111111111111111111111111111",
					"auto_accept_channel_ckb_funding_amount": "0x2540be400",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"channel_id": "ch_ready_1",
							"state":      "CHANNEL_READY",
							"enabled":    true,
							"funding_udt_type_script": map[string]any{
								"code_hash": "0xudt",
								"hash_type": "type",
								"args":      "0x01",
							},
						},
					},
				},
			})
		case "accept_channel":
			if len(rpc.Params) != 1 {
				t.Fatalf("unexpected accept_channel params: %+v", rpc.Params)
			}
			arg, ok := rpc.Params[0].(map[string]any)
			if !ok {
				t.Fatalf("unexpected accept_channel arg type: %T", rpc.Params[0])
			}
			providerAcceptPayload = arg
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		default:
			t.Fatalf("unexpected provider method %q", rpc.Method)
		}
	}))
	defer providerNode.Close()

	provisioner, err := NewFNNProviderSettlementProvisioner(FNNProviderSettlementProvisionerConfig{
		TreasuryRPCURL:       treasuryNode.URL,
		TreasuryP2PHost:      "fnn2",
		TreasuryP2PPort:      8228,
		PollInterval:         5 * time.Millisecond,
		WaitTimeout:          time.Second,
		OpenChannelRetries:   1,
		AcceptChannelRetries: 1,
	})
	if err != nil {
		t.Fatalf("new provisioner: %v", err)
	}

	result, err := provisioner.EnsureProviderLiquidity(EnsureProviderLiquidityInput{
		ProviderOrgID:      "provider_1",
		NeededReserveCents: 1_980,
		Binding: ProviderSettlementBinding{
			ID:            "psb_1",
			ProviderOrgID: "provider_1",
			Asset:         "USDI",
			PeerID:        providerPeerID,
			P2PAddress:    "/dns4/provider/tcp/8228/p2p/" + providerPeerID,
			NodeRPCURL:    providerNode.URL,
			UDTTypeScript: UDTTypeScript{
				CodeHash: "0xudt",
				HashType: "type",
				Args:     "0x01",
			},
		},
		CurrentPool: ProviderLiquidityPool{},
	})
	if err != nil {
		t.Fatalf("ensure provider liquidity: %v", err)
	}
	if result.ReuseSource != ProviderLiquidityReuseNewChannel {
		t.Fatalf("reuse source = %s, want new_channel", result.ReuseSource)
	}
	if result.ChannelID != "tmp_provider_1" {
		t.Fatalf("channel id = %s, want tmp_provider_1", result.ChannelID)
	}
	if got := treasuryOpenPayload["funding_amount"]; got != "0x14" {
		t.Fatalf("funding_amount = %v, want 0x14", got)
	}
	if got := providerAcceptPayload["temporary_channel_id"]; got != "tmp_provider_1" {
		t.Fatalf("temporary_channel_id = %v, want tmp_provider_1", got)
	}
}

func TestFNNProviderSettlementProvisioner_ReusesReadyChannelWhenPoolCapacitySufficient(t *testing.T) {
	const providerPeerID = "QmdWHo7ejoqhoXN56bFAWQtFtewPkrUSMA64Wp9XnB4n13"

	var treasuryOpenCalled bool
	treasuryNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read treasury request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode treasury request: %v", err)
		}
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id": "0x032222222222222222222222222222222222222222222222222222222222222222",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"channel_id": "ch_reuse_1",
							"state":      "CHANNEL_READY",
							"enabled":    true,
							"funding_udt_type_script": map[string]any{
								"code_hash": "0xudt",
								"hash_type": "type",
								"args":      "0x01",
							},
						},
					},
				},
			})
		case "open_channel":
			treasuryOpenCalled = true
			t.Fatal("did not expect open_channel during reuse path")
		default:
			t.Fatalf("unexpected treasury method %q", rpc.Method)
		}
	}))
	defer treasuryNode.Close()

	var providerAcceptCalled bool
	providerNode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read provider request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id":                                "0x021111111111111111111111111111111111111111111111111111111111111111",
					"auto_accept_channel_ckb_funding_amount": "0x2540be400",
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"channel_id": "ch_reuse_1",
							"state":      "CHANNEL_READY",
							"enabled":    true,
							"funding_udt_type_script": map[string]any{
								"code_hash": "0xudt",
								"hash_type": "type",
								"args":      "0x01",
							},
						},
					},
				},
			})
		case "accept_channel":
			providerAcceptCalled = true
			t.Fatal("did not expect accept_channel during reuse path")
		default:
			t.Fatalf("unexpected provider method %q", rpc.Method)
		}
	}))
	defer providerNode.Close()

	provisioner, err := NewFNNProviderSettlementProvisioner(FNNProviderSettlementProvisionerConfig{
		TreasuryRPCURL:       treasuryNode.URL,
		TreasuryP2PHost:      "fnn2",
		TreasuryP2PPort:      8228,
		PollInterval:         5 * time.Millisecond,
		WaitTimeout:          time.Second,
		OpenChannelRetries:   1,
		AcceptChannelRetries: 1,
		MinFundingUnits:      1,
	})
	if err != nil {
		t.Fatalf("new provisioner: %v", err)
	}

	result, err := provisioner.EnsureProviderLiquidity(EnsureProviderLiquidityInput{
		ProviderOrgID:      "provider_1",
		NeededReserveCents: 1_000,
		Binding: ProviderSettlementBinding{
			ID:            "psb_1",
			ProviderOrgID: "provider_1",
			Asset:         "USDI",
			PeerID:        providerPeerID,
			P2PAddress:    "/dns4/provider/tcp/8228/p2p/" + providerPeerID,
			NodeRPCURL:    providerNode.URL,
			UDTTypeScript: UDTTypeScript{
				CodeHash: "0xudt",
				HashType: "type",
				Args:     "0x01",
			},
		},
		CurrentPool: ProviderLiquidityPool{
			TotalSpendableCents:      8_000,
			ReservedOutstandingCents: 1_000,
			AvailableToAllocateCents: 7_000,
			Status:                   ProviderLiquidityPoolStatusHealthy,
		},
	})
	if err != nil {
		t.Fatalf("ensure provider liquidity: %v", err)
	}
	if result.ReuseSource != ProviderLiquidityReuseReused {
		t.Fatalf("reuse source = %s, want reused", result.ReuseSource)
	}
	if result.ChannelID != "ch_reuse_1" {
		t.Fatalf("channel id = %s, want ch_reuse_1", result.ChannelID)
	}
	if treasuryOpenCalled || providerAcceptCalled {
		t.Fatalf("expected reuse path without open/accept, open=%t accept=%t", treasuryOpenCalled, providerAcceptCalled)
	}
}
