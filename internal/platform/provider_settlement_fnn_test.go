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
					"udt_cfg_infos": []map[string]any{
						{
							"name":               "USDI",
							"auto_accept_amount": "0x3b9aca00",
							"script": map[string]any{
								"code_hash": "0xudt",
								"hash_type": "type",
								"args":      "0x01",
							},
						},
					},
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
		TreasuryP2PHost:      "localhost",
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
			P2PAddress:    "/ip4/127.0.0.1/tcp/8228/p2p/" + providerPeerID,
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
	udtTypeScript, ok := treasuryOpenPayload["funding_udt_type_script"].(map[string]any)
	if !ok {
		t.Fatalf("missing funding_udt_type_script in open payload: %+v", treasuryOpenPayload)
	}
	if udtTypeScript["code_hash"] != "0xudt" || udtTypeScript["hash_type"] != "type" || udtTypeScript["args"] != "0x01" {
		t.Fatalf("unexpected funding_udt_type_script: %+v", udtTypeScript)
	}
	if got := providerAcceptPayload["temporary_channel_id"]; got != "tmp_provider_1" {
		t.Fatalf("temporary_channel_id = %v, want tmp_provider_1", got)
	}
	if got := providerAcceptPayload["funding_amount"]; got != "0x3b9aca00" {
		t.Fatalf("accept funding_amount = %v, want 0x3b9aca00", got)
	}
}

func TestFNNProviderSettlementProvisioner_RetriesOpenChannelAfterPeerInitRace(t *testing.T) {
	const providerPeerID = "QmdWHo7ejoqhoXN56bFAWQtFtewPkrUSMA64Wp9XnB4n13"

	treasuryConnectCalls := 0
	treasuryOpenCalls := 0
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
			treasuryConnectCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			state := "AWAITING_CHANNEL_READY"
			if treasuryOpenCalls >= 2 {
				state = "CHANNEL_READY"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"channel_id": "ch_ready_1",
							"state":      state,
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
			treasuryOpenCalls++
			if treasuryOpenCalls == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"jsonrpc": "2.0",
					"id":      1,
					"error": map[string]any{
						"message": "Invalid parameter: Peer PeerId(QmPeer)'s feature not found, waiting for peer to send Init message",
					},
				})
				return
			}
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

	providerConnectCalls := 0
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
					"udt_cfg_infos": []map[string]any{
						{
							"name":               "USDI",
							"auto_accept_amount": "0x3b9aca00",
							"script": map[string]any{
								"code_hash": "0xudt",
								"hash_type": "type",
								"args":      "0x01",
							},
						},
					},
				},
			})
		case "connect_peer":
			providerConnectCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "accept_channel":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			state := "AWAITING_CHANNEL_READY"
			if treasuryOpenCalls >= 2 {
				state = "CHANNEL_READY"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"channel_id": "ch_ready_1",
							"state":      state,
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
		default:
			t.Fatalf("unexpected provider method %q", rpc.Method)
		}
	}))
	defer providerNode.Close()

	provisioner, err := NewFNNProviderSettlementProvisioner(FNNProviderSettlementProvisionerConfig{
		TreasuryRPCURL:       treasuryNode.URL,
		TreasuryP2PHost:      "localhost",
		TreasuryP2PPort:      8228,
		PollInterval:         5 * time.Millisecond,
		WaitTimeout:          time.Second,
		OpenChannelRetries:   3,
		AcceptChannelRetries: 1,
	})
	if err != nil {
		t.Fatalf("new provisioner: %v", err)
	}

	_, err = provisioner.EnsureProviderLiquidity(EnsureProviderLiquidityInput{
		ProviderOrgID:      "provider_1",
		NeededReserveCents: 1_980,
		Binding: ProviderSettlementBinding{
			ID:            "psb_1",
			ProviderOrgID: "provider_1",
			Asset:         "USDI",
			PeerID:        providerPeerID,
			P2PAddress:    "/ip4/127.0.0.1/tcp/8228/p2p/" + providerPeerID,
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
	if treasuryOpenCalls < 2 {
		t.Fatalf("open_channel calls = %d, want at least 2", treasuryOpenCalls)
	}
	if treasuryConnectCalls < 2 || providerConnectCalls < 2 {
		t.Fatalf("connect_peer calls treasury=%d provider=%d, want reconnect on retry", treasuryConnectCalls, providerConnectCalls)
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
		TreasuryP2PHost:      "localhost",
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
			P2PAddress:    "/ip4/127.0.0.1/tcp/8228/p2p/" + providerPeerID,
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

func TestFNNProviderSettlementProvisioner_ReacceptsWhileAwaitingChannelReady(t *testing.T) {
	const providerPeerID = "QmdWHo7ejoqhoXN56bFAWQtFtewPkrUSMA64Wp9XnB4n13"

	treasuryListCalls := 0
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
		case "open_channel":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"temporary_channel_id": "tmp_provider_1",
				},
			})
		case "list_channels":
			treasuryListCalls++
			state := "AWAITING_CHANNEL_READY"
			if treasuryListCalls >= 3 {
				state = "CHANNEL_READY"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"channel_id": "ch_ready_1",
							"state":      state,
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
		default:
			t.Fatalf("unexpected treasury method %q", rpc.Method)
		}
	}))
	defer treasuryNode.Close()

	providerListCalls := 0
	providerAcceptCalls := 0
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
					"udt_cfg_infos": []map[string]any{
						{
							"name":               "USDI",
							"auto_accept_amount": "0x3b9aca00",
							"script": map[string]any{
								"code_hash": "0xudt",
								"hash_type": "type",
								"args":      "0x01",
							},
						},
					},
				},
			})
		case "connect_peer":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "accept_channel":
			providerAcceptCalls++
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "list_channels":
			providerListCalls++
			state := "AWAITING_CHANNEL_READY"
			if providerAcceptCalls >= 2 && providerListCalls >= 3 {
				state = "CHANNEL_READY"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"channels": []map[string]any{
						{
							"channel_id": "ch_ready_1",
							"state":      state,
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
		default:
			t.Fatalf("unexpected provider method %q", rpc.Method)
		}
	}))
	defer providerNode.Close()

	provisioner, err := NewFNNProviderSettlementProvisioner(FNNProviderSettlementProvisionerConfig{
		TreasuryRPCURL:       treasuryNode.URL,
		TreasuryP2PHost:      "localhost",
		TreasuryP2PPort:      8228,
		PollInterval:         5 * time.Millisecond,
		WaitTimeout:          time.Second,
		OpenChannelRetries:   1,
		AcceptChannelRetries: 1,
		AcceptRetryEvery:     2,
	})
	if err != nil {
		t.Fatalf("new provisioner: %v", err)
	}

	_, err = provisioner.EnsureProviderLiquidity(EnsureProviderLiquidityInput{
		ProviderOrgID:      "provider_1",
		NeededReserveCents: 1_980,
		Binding: ProviderSettlementBinding{
			ID:            "psb_1",
			ProviderOrgID: "provider_1",
			Asset:         "USDI",
			PeerID:        providerPeerID,
			P2PAddress:    "/ip4/127.0.0.1/tcp/8228/p2p/" + providerPeerID,
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
	if providerAcceptCalls < 2 {
		t.Fatalf("accept_channel calls = %d, want at least 2", providerAcceptCalls)
	}
}

func TestFNNProviderSettlementProvisioner_NormalizesDNSPeerAddressesBeforeConnect(t *testing.T) {
	const providerPeerID = "QmdWHo7ejoqhoXN56bFAWQtFtewPkrUSMA64Wp9XnB4n13"

	var treasuryConnectAddress string
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
			arg := rpc.Params[0].(map[string]any)
			treasuryConnectAddress, _ = arg["address"].(string)
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
		default:
			t.Fatalf("unexpected treasury method %q", rpc.Method)
		}
	}))
	defer treasuryNode.Close()

	var providerConnectAddress string
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
		switch rpc.Method {
		case "node_info":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"node_id": "0x021111111111111111111111111111111111111111111111111111111111111111",
				},
			})
		case "connect_peer":
			arg := rpc.Params[0].(map[string]any)
			providerConnectAddress, _ = arg["address"].(string)
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
		default:
			t.Fatalf("unexpected provider method %q", rpc.Method)
		}
	}))
	defer providerNode.Close()

	provisioner, err := NewFNNProviderSettlementProvisioner(FNNProviderSettlementProvisionerConfig{
		TreasuryRPCURL:       treasuryNode.URL,
		TreasuryP2PHost:      "localhost",
		TreasuryP2PPort:      8228,
		PollInterval:         5 * time.Millisecond,
		WaitTimeout:          time.Second,
		OpenChannelRetries:   1,
		AcceptChannelRetries: 1,
	})
	if err != nil {
		t.Fatalf("new provisioner: %v", err)
	}

	_, err = provisioner.EnsureProviderLiquidity(EnsureProviderLiquidityInput{
		ProviderOrgID:      "provider_1",
		NeededReserveCents: 1_000,
		Binding: ProviderSettlementBinding{
			ID:            "psb_1",
			ProviderOrgID: "provider_1",
			Asset:         "USDI",
			PeerID:        providerPeerID,
			P2PAddress:    "/dns4/localhost/tcp/8228/p2p/" + providerPeerID,
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

	if treasuryConnectAddress != "/ip4/127.0.0.1/tcp/8228/p2p/"+providerPeerID {
		t.Fatalf("treasury connect address = %q", treasuryConnectAddress)
	}
	if providerConnectAddress == "" || !strings.HasPrefix(providerConnectAddress, "/ip4/127.0.0.1/tcp/8228/p2p/") {
		t.Fatalf("provider connect address = %q", providerConnectAddress)
	}
}

func TestNeededReserveToFundingUnitsRoundsUpToWholeUSDITokens(t *testing.T) {
	if got := neededReserveToFundingUnits(1_980, 100, 1); got != 20 {
		t.Fatalf("neededReserveToFundingUnits(1980) = %d, want 20", got)
	}
	if got := neededReserveToFundingUnits(100, 100, 1); got != 1 {
		t.Fatalf("neededReserveToFundingUnits(100) = %d, want 1", got)
	}
	if got := neededReserveToFundingUnits(0, 100, 1); got != 1 {
		t.Fatalf("neededReserveToFundingUnits(0) = %d, want 1", got)
	}
}
