package settlement

import (
	"context"
	"testing"
	"time"
)

type stubBuyerDepositWallet struct {
	derivedAddresses map[int]string
	deriveCalls      []int
	balances         map[string]BuyerDepositChainBalance
	sweepResults     map[string]BuyerDepositSweepResult
	sweepCalls       []string
}

func (s *stubBuyerDepositWallet) DeriveAddress(index int) (string, error) {
	s.deriveCalls = append(s.deriveCalls, index)
	if address, ok := s.derivedAddresses[index]; ok {
		return address, nil
	}
	return "", nil
}

func (s *stubBuyerDepositWallet) QueryBalance(_ context.Context, record BuyerDepositAddress) (BuyerDepositChainBalance, error) {
	return s.balances[record.Address], nil
}

func (s *stubBuyerDepositWallet) SweepToTreasury(_ context.Context, record BuyerDepositAddress, treasuryAddress string, _ uint64) (BuyerDepositSweepResult, error) {
	s.sweepCalls = append(s.sweepCalls, record.Address+"->"+treasuryAddress)
	return s.sweepResults[record.Address], nil
}

func TestBuyerDepositServiceEnsureAddressReusesBuyerOrgAddress(t *testing.T) {
	wallet := &stubBuyerDepositWallet{
		derivedAddresses: map[int]string{
			0: "ckt1qyqbuyer0address",
		},
	}
	service := NewBuyerDepositService(BuyerDepositServiceOptions{
		Addresses:            NewMemoryBuyerDepositAddressRepository(),
		Sweeps:               NewMemoryBuyerDepositSweepRepository(),
		Funding:              NewMemoryFundingRecordRepository(),
		Wallet:               wallet,
		Asset:                "USDI",
		TreasuryAddress:      "ckt1qyqtreasuryaddress",
		MinSweepAmountRaw:    1_000_000_000,
		ConfirmationBlocks:   24,
		RawUnitsPerWholeUSDI: 100_000_000,
	})

	first, err := service.EnsureAddress(context.Background(), "buyer_1")
	if err != nil {
		t.Fatalf("ensure address first: %v", err)
	}
	second, err := service.EnsureAddress(context.Background(), "buyer_1")
	if err != nil {
		t.Fatalf("ensure address second: %v", err)
	}

	if first.Address != "ckt1qyqbuyer0address" || second.Address != first.Address {
		t.Fatalf("expected stable derived address, first=%+v second=%+v", first, second)
	}
	if len(wallet.deriveCalls) != 1 || wallet.deriveCalls[0] != 0 {
		t.Fatalf("expected one derive call for index 0, got %+v", wallet.deriveCalls)
	}
}

func TestBuyerDepositServiceSyncCreditsEntireConfirmedBalanceOnceThresholdReached(t *testing.T) {
	wallet := &stubBuyerDepositWallet{
		derivedAddresses: map[int]string{
			0: "ckt1qyqbuyer0address",
		},
		balances: map[string]BuyerDepositChainBalance{
			"ckt1qyqbuyer0address": {
				Address:            "ckt1qyqbuyer0address",
				RawOnChainUnits:    1_300_000_000,
				RawConfirmedUnits:  1_300_000_000,
				ConfirmationBlocks: 24,
			},
		},
		sweepResults: map[string]BuyerDepositSweepResult{
			"ckt1qyqbuyer0address": {
				SweepTxHash:     "0xsweep123",
				SweptRawUnits:   1_300_000_000,
				TreasuryAddress: "ckt1qyqtreasuryaddress",
			},
		},
	}
	funding := NewMemoryFundingRecordRepository()
	sweeps := NewMemoryBuyerDepositSweepRepository()
	service := NewBuyerDepositService(BuyerDepositServiceOptions{
		Addresses:            NewMemoryBuyerDepositAddressRepository(),
		Sweeps:               sweeps,
		Funding:              funding,
		Wallet:               wallet,
		Asset:                "USDI",
		TreasuryAddress:      "ckt1qyqtreasuryaddress",
		MinSweepAmountRaw:    1_000_000_000,
		ConfirmationBlocks:   24,
		RawUnitsPerWholeUSDI: 100_000_000,
		Now: func() time.Time {
			return time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
		},
	})

	if _, err := service.EnsureAddress(context.Background(), "buyer_1"); err != nil {
		t.Fatalf("ensure address: %v", err)
	}
	if _, err := service.SyncDeposits(context.Background()); err != nil {
		t.Fatalf("sync deposits: %v", err)
	}

	records, err := funding.List(FundingRecordFilter{Kind: FundingRecordKindBuyerTopUp, BuyerOrgID: "buyer_1"})
	if err != nil {
		t.Fatalf("list funding records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one buyer credit, got %+v", records)
	}
	if records[0].Amount != "13.00" || records[0].State != "SETTLED" || records[0].ExternalID != "0xsweep123" {
		t.Fatalf("unexpected buyer topup credit: %+v", records[0])
	}

	sweepRecords, err := sweeps.List(BuyerDepositSweepFilter{BuyerOrgID: "buyer_1"})
	if err != nil {
		t.Fatalf("list sweep records: %v", err)
	}
	if len(sweepRecords) != 1 {
		t.Fatalf("expected one sweep record, got %+v", sweepRecords)
	}
	if sweepRecords[0].AmountRaw != 1_300_000_000 || sweepRecords[0].TreasuryAddress != "ckt1qyqtreasuryaddress" {
		t.Fatalf("unexpected sweep record: %+v", sweepRecords[0])
	}
	if len(wallet.sweepCalls) != 1 {
		t.Fatalf("expected one sweep call, got %+v", wallet.sweepCalls)
	}
}

func TestBuyerDepositServiceSyncIsIdempotentForExistingSweepTxHash(t *testing.T) {
	wallet := &stubBuyerDepositWallet{
		derivedAddresses: map[int]string{
			0: "ckt1qyqbuyer0address",
		},
		balances: map[string]BuyerDepositChainBalance{
			"ckt1qyqbuyer0address": {
				Address:            "ckt1qyqbuyer0address",
				RawOnChainUnits:    1_300_000_000,
				RawConfirmedUnits:  1_300_000_000,
				ConfirmationBlocks: 24,
			},
		},
		sweepResults: map[string]BuyerDepositSweepResult{
			"ckt1qyqbuyer0address": {
				SweepTxHash:     "0xsweep123",
				SweptRawUnits:   1_300_000_000,
				TreasuryAddress: "ckt1qyqtreasuryaddress",
			},
		},
	}
	funding := NewMemoryFundingRecordRepository()
	sweeps := NewMemoryBuyerDepositSweepRepository()
	service := NewBuyerDepositService(BuyerDepositServiceOptions{
		Addresses:            NewMemoryBuyerDepositAddressRepository(),
		Sweeps:               sweeps,
		Funding:              funding,
		Wallet:               wallet,
		Asset:                "USDI",
		TreasuryAddress:      "ckt1qyqtreasuryaddress",
		MinSweepAmountRaw:    1_000_000_000,
		ConfirmationBlocks:   24,
		RawUnitsPerWholeUSDI: 100_000_000,
		Now: func() time.Time {
			return time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)
		},
	})

	if _, err := service.EnsureAddress(context.Background(), "buyer_1"); err != nil {
		t.Fatalf("ensure address: %v", err)
	}
	if _, err := service.SyncDeposits(context.Background()); err != nil {
		t.Fatalf("first sync deposits: %v", err)
	}
	if _, err := service.SyncDeposits(context.Background()); err != nil {
		t.Fatalf("second sync deposits: %v", err)
	}

	records, err := funding.List(FundingRecordFilter{Kind: FundingRecordKindBuyerTopUp, BuyerOrgID: "buyer_1"})
	if err != nil {
		t.Fatalf("list funding records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one buyer credit after repeated sync, got %+v", records)
	}

	sweepRecords, err := sweeps.List(BuyerDepositSweepFilter{BuyerOrgID: "buyer_1"})
	if err != nil {
		t.Fatalf("list sweep records: %v", err)
	}
	if len(sweepRecords) != 1 {
		t.Fatalf("expected one sweep record after repeated sync, got %+v", sweepRecords)
	}
	if len(wallet.sweepCalls) != 2 {
		t.Fatalf("expected wallet sweep to be attempted twice, got %+v", wallet.sweepCalls)
	}
}

func TestBuyerDepositServiceSyncLeavesBelowThresholdPending(t *testing.T) {
	wallet := &stubBuyerDepositWallet{
		derivedAddresses: map[int]string{
			0: "ckt1qyqbuyer0address",
		},
		balances: map[string]BuyerDepositChainBalance{
			"ckt1qyqbuyer0address": {
				Address:            "ckt1qyqbuyer0address",
				RawOnChainUnits:    500_000_000,
				RawConfirmedUnits:  500_000_000,
				ConfirmationBlocks: 24,
			},
		},
	}
	funding := NewMemoryFundingRecordRepository()
	service := NewBuyerDepositService(BuyerDepositServiceOptions{
		Addresses:            NewMemoryBuyerDepositAddressRepository(),
		Sweeps:               NewMemoryBuyerDepositSweepRepository(),
		Funding:              funding,
		Wallet:               wallet,
		Asset:                "USDI",
		TreasuryAddress:      "ckt1qyqtreasuryaddress",
		MinSweepAmountRaw:    1_000_000_000,
		ConfirmationBlocks:   24,
		RawUnitsPerWholeUSDI: 100_000_000,
	})

	if _, err := service.EnsureAddress(context.Background(), "buyer_1"); err != nil {
		t.Fatalf("ensure address: %v", err)
	}
	if _, err := service.SyncDeposits(context.Background()); err != nil {
		t.Fatalf("sync deposits: %v", err)
	}

	records, err := funding.List(FundingRecordFilter{Kind: FundingRecordKindBuyerTopUp, BuyerOrgID: "buyer_1"})
	if err != nil {
		t.Fatalf("list funding records: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no buyer credit below threshold, got %+v", records)
	}
	if len(wallet.sweepCalls) != 0 {
		t.Fatalf("expected no sweep calls below threshold, got %+v", wallet.sweepCalls)
	}
}
