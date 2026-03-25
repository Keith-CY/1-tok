package settlement

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBuyerDepositRawUnitsPerWholeUSDI int64 = 1_000_000

type BuyerDepositAddress struct {
	ID              string    `json:"id"`
	BuyerOrgID      string    `json:"buyerOrgId"`
	Asset           string    `json:"asset"`
	Address         string    `json:"address"`
	DerivationIndex int       `json:"derivationIndex"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type BuyerDepositChainBalance struct {
	Address            string `json:"address"`
	RawOnChainUnits    int64  `json:"rawOnChainUnits"`
	RawConfirmedUnits  int64  `json:"rawConfirmedUnits"`
	ConfirmationBlocks uint64 `json:"confirmationBlocks"`
}

type BuyerDepositSweepResult struct {
	SweepTxHash     string `json:"sweepTxHash"`
	SweptRawUnits   int64  `json:"sweptRawUnits"`
	TreasuryAddress string `json:"treasuryAddress,omitempty"`
}

type BuyerDepositSummary struct {
	BuyerOrgID           string `json:"buyerOrgId"`
	Asset                string `json:"asset"`
	Address              string `json:"address"`
	OnChainBalance       string `json:"onChainBalance"`
	ConfirmedBalance     string `json:"confirmedBalance"`
	CreditedBalance      string `json:"creditedBalance"`
	CreditedBalanceCents int64  `json:"creditedBalanceCents"`
	MinimumSweepAmount   string `json:"minimumSweepAmount"`
	ConfirmationBlocks   uint64 `json:"confirmationBlocks"`
	RawOnChainUnits      int64  `json:"rawOnChainUnits"`
	RawConfirmedUnits    int64  `json:"rawConfirmedUnits"`
	RawMinimumSweepUnits int64  `json:"rawMinimumSweepUnits"`
	TreasuryAddress      string `json:"treasuryAddress,omitempty"`
}

type BuyerDepositSweepRecord struct {
	ID              string    `json:"id"`
	BuyerOrgID      string    `json:"buyerOrgId"`
	Asset           string    `json:"asset"`
	DepositAddress  string    `json:"depositAddress"`
	TreasuryAddress string    `json:"treasuryAddress"`
	AmountRaw       int64     `json:"amountRaw"`
	TxHash          string    `json:"txHash"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type BuyerDepositSweepFilter struct {
	BuyerOrgID string
}

type BuyerDepositAddressRepository interface {
	NextID() (string, error)
	NextDerivationIndex() (int, error)
	Save(BuyerDepositAddress) error
	GetByBuyerOrgID(buyerOrgID string) (BuyerDepositAddress, error)
	List() ([]BuyerDepositAddress, error)
}

type BuyerDepositSweepRepository interface {
	NextID() (string, error)
	Save(BuyerDepositSweepRecord) error
	List(filter BuyerDepositSweepFilter) ([]BuyerDepositSweepRecord, error)
}

type BuyerDepositWallet interface {
	DeriveAddress(index int) (string, error)
	QueryBalance(ctx context.Context, record BuyerDepositAddress) (BuyerDepositChainBalance, error)
	SweepToTreasury(ctx context.Context, record BuyerDepositAddress, treasuryAddress string, confirmationBlocks uint64) (BuyerDepositSweepResult, error)
}

type BuyerDepositServiceOptions struct {
	Addresses            BuyerDepositAddressRepository
	Sweeps               BuyerDepositSweepRepository
	Funding              FundingRecordRepository
	Wallet               BuyerDepositWallet
	Asset                string
	TreasuryAddress      string
	MinSweepAmountRaw    int64
	ConfirmationBlocks   uint64
	RawUnitsPerWholeUSDI int64
	Now                  func() time.Time
}

type BuyerDepositService struct {
	addresses            BuyerDepositAddressRepository
	sweeps               BuyerDepositSweepRepository
	funding              FundingRecordRepository
	wallet               BuyerDepositWallet
	asset                string
	treasuryAddress      string
	minSweepAmountRaw    int64
	confirmationBlocks   uint64
	rawUnitsPerWholeUSDI int64
	now                  func() time.Time
}

func NewBuyerDepositService(options BuyerDepositServiceOptions) *BuyerDepositService {
	if options.Addresses == nil {
		options.Addresses = NewMemoryBuyerDepositAddressRepository()
	}
	if options.Sweeps == nil {
		options.Sweeps = NewMemoryBuyerDepositSweepRepository()
	}
	if options.Funding == nil {
		options.Funding = NewMemoryFundingRecordRepository()
	}
	if options.Asset == "" {
		options.Asset = "USDI"
	}
	if options.MinSweepAmountRaw <= 0 {
		options.MinSweepAmountRaw = 10 * defaultBuyerDepositRawUnitsPerWholeUSDI
	}
	if options.ConfirmationBlocks == 0 {
		options.ConfirmationBlocks = 24
	}
	if options.RawUnitsPerWholeUSDI <= 0 {
		options.RawUnitsPerWholeUSDI = defaultBuyerDepositRawUnitsPerWholeUSDI
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	return &BuyerDepositService{
		addresses:            options.Addresses,
		sweeps:               options.Sweeps,
		funding:              options.Funding,
		wallet:               options.Wallet,
		asset:                options.Asset,
		treasuryAddress:      options.TreasuryAddress,
		minSweepAmountRaw:    options.MinSweepAmountRaw,
		confirmationBlocks:   options.ConfirmationBlocks,
		rawUnitsPerWholeUSDI: options.RawUnitsPerWholeUSDI,
		now:                  options.Now,
	}
}

func (s *BuyerDepositService) EnsureAddress(ctx context.Context, buyerOrgID string) (BuyerDepositAddress, error) {
	if strings.TrimSpace(buyerOrgID) == "" {
		return BuyerDepositAddress{}, errors.New("buyerOrgId is required")
	}
	if existing, err := s.addresses.GetByBuyerOrgID(strings.TrimSpace(buyerOrgID)); err == nil {
		return existing, nil
	}
	if s.wallet == nil {
		return BuyerDepositAddress{}, errors.New("buyer deposit wallet is required")
	}
	index, err := s.addresses.NextDerivationIndex()
	if err != nil {
		return BuyerDepositAddress{}, err
	}
	address, err := s.wallet.DeriveAddress(index)
	if err != nil {
		return BuyerDepositAddress{}, err
	}
	if strings.TrimSpace(address) == "" {
		return BuyerDepositAddress{}, errors.New("buyer deposit wallet returned empty address")
	}
	id, err := s.addresses.NextID()
	if err != nil {
		return BuyerDepositAddress{}, err
	}
	now := s.now()
	record := BuyerDepositAddress{
		ID:              id,
		BuyerOrgID:      strings.TrimSpace(buyerOrgID),
		Asset:           s.asset,
		Address:         address,
		DerivationIndex: index,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.addresses.Save(record); err != nil {
		return BuyerDepositAddress{}, err
	}
	return record, nil
}

func (s *BuyerDepositService) GetSummary(ctx context.Context, buyerOrgID string) (BuyerDepositSummary, error) {
	record, err := s.EnsureAddress(ctx, buyerOrgID)
	if err != nil {
		return BuyerDepositSummary{}, err
	}
	balance := BuyerDepositChainBalance{Address: record.Address}
	if s.wallet != nil {
		balance, err = s.wallet.QueryBalance(ctx, record)
		if err != nil {
			return BuyerDepositSummary{}, err
		}
	}
	credits, err := s.funding.List(FundingRecordFilter{Kind: FundingRecordKindBuyerTopUp, BuyerOrgID: buyerOrgID})
	if err != nil {
		return BuyerDepositSummary{}, err
	}
	creditedCents := int64(0)
	for _, record := range credits {
		if strings.EqualFold(record.State, "SETTLED") {
			creditedCents += parseAmountToCents64(record.Amount)
		}
	}
	return BuyerDepositSummary{
		BuyerOrgID:           buyerOrgID,
		Asset:                s.asset,
		Address:              record.Address,
		OnChainBalance:       rawUnitsToAmountString(balance.RawOnChainUnits, s.rawUnitsPerWholeUSDI),
		ConfirmedBalance:     rawUnitsToAmountString(balance.RawConfirmedUnits, s.rawUnitsPerWholeUSDI),
		CreditedBalance:      centsToAmountString(creditedCents),
		CreditedBalanceCents: creditedCents,
		MinimumSweepAmount:   rawUnitsToAmountString(s.minSweepAmountRaw, s.rawUnitsPerWholeUSDI),
		ConfirmationBlocks:   s.confirmationBlocks,
		RawOnChainUnits:      balance.RawOnChainUnits,
		RawConfirmedUnits:    balance.RawConfirmedUnits,
		RawMinimumSweepUnits: s.minSweepAmountRaw,
		TreasuryAddress:      s.treasuryAddress,
	}, nil
}

type BuyerDepositSyncSummary struct {
	CreditedCents int64 `json:"creditedCents"`
	SweepCount    int   `json:"sweepCount"`
}

func (s *BuyerDepositService) SyncDeposits(ctx context.Context) (BuyerDepositSyncSummary, error) {
	if s.wallet == nil {
		return BuyerDepositSyncSummary{}, nil
	}
	records, err := s.addresses.List()
	if err != nil {
		return BuyerDepositSyncSummary{}, err
	}
	summary := BuyerDepositSyncSummary{}
	for _, record := range records {
		balance, err := s.wallet.QueryBalance(ctx, record)
		if err != nil {
			return summary, err
		}
		if balance.RawConfirmedUnits < s.minSweepAmountRaw {
			continue
		}
		sweepResult, err := s.wallet.SweepToTreasury(ctx, record, s.treasuryAddress, s.confirmationBlocks)
		if err != nil {
			return summary, err
		}
		if sweepResult.SweptRawUnits <= 0 {
			continue
		}
		existingSweeps, err := s.sweeps.List(BuyerDepositSweepFilter{BuyerOrgID: record.BuyerOrgID})
		if err != nil {
			return summary, err
		}
		if existingSweep, ok := findBuyerDepositSweepByTxHash(existingSweeps, sweepResult.SweepTxHash); ok {
			if alreadyCredited, err := s.hasSettledBuyerTopUp(record.BuyerOrgID, sweepResult.SweepTxHash); err != nil {
				return summary, err
			} else if alreadyCredited {
				continue
			}
			creditedCents := rawUnitsToCents(existingSweep.AmountRaw, s.rawUnitsPerWholeUSDI)
			if err := s.saveBuyerTopUpFunding(record, firstNonEmptySweepAddress(existingSweep.TreasuryAddress, s.treasuryAddress), existingSweep.TxHash, creditedCents); err != nil {
				return summary, err
			}
			summary.CreditedCents += creditedCents
			summary.SweepCount++
			continue
		}
		now := s.now()
		sweepID, err := s.sweeps.NextID()
		if err != nil {
			return summary, err
		}
		if err := s.sweeps.Save(BuyerDepositSweepRecord{
			ID:              sweepID,
			BuyerOrgID:      record.BuyerOrgID,
			Asset:           s.asset,
			DepositAddress:  record.Address,
			TreasuryAddress: firstNonEmptySweepAddress(sweepResult.TreasuryAddress, s.treasuryAddress),
			AmountRaw:       sweepResult.SweptRawUnits,
			TxHash:          sweepResult.SweepTxHash,
			State:           "SETTLED",
			CreatedAt:       now,
			UpdatedAt:       now,
		}); err != nil {
			return summary, err
		}

		creditedCents := rawUnitsToCents(sweepResult.SweptRawUnits, s.rawUnitsPerWholeUSDI)
		if err := s.saveBuyerTopUpFunding(record, firstNonEmptySweepAddress(sweepResult.TreasuryAddress, s.treasuryAddress), sweepResult.SweepTxHash, creditedCents); err != nil {
			return summary, err
		}
		summary.CreditedCents += creditedCents
		summary.SweepCount++
	}
	return summary, nil
}

func (s *BuyerDepositService) hasSettledBuyerTopUp(buyerOrgID, externalID string) (bool, error) {
	if strings.TrimSpace(externalID) == "" {
		return false, nil
	}
	records, err := s.funding.List(FundingRecordFilter{Kind: FundingRecordKindBuyerTopUp, BuyerOrgID: buyerOrgID})
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if record.ExternalID == externalID && strings.EqualFold(record.State, "SETTLED") {
			return true, nil
		}
	}
	return false, nil
}

func (s *BuyerDepositService) saveBuyerTopUpFunding(record BuyerDepositAddress, treasuryAddress, externalID string, creditedCents int64) error {
	fundingID, err := s.funding.NextID()
	if err != nil {
		return err
	}
	now := s.now()
	return s.funding.Save(FundingRecord{
		ID:         fundingID,
		Kind:       FundingRecordKindBuyerTopUp,
		BuyerOrgID: record.BuyerOrgID,
		Asset:      s.asset,
		Amount:     centsToAmountString(creditedCents),
		ExternalID: externalID,
		State:      "SETTLED",
		Destination: map[string]string{
			"kind":            "CKB_ADDRESS_SWEEP",
			"depositAddress":  record.Address,
			"treasuryAddress": treasuryAddress,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func findBuyerDepositSweepByTxHash(records []BuyerDepositSweepRecord, txHash string) (BuyerDepositSweepRecord, bool) {
	if strings.TrimSpace(txHash) == "" {
		return BuyerDepositSweepRecord{}, false
	}
	for _, record := range records {
		if record.TxHash == txHash {
			return record, true
		}
	}
	return BuyerDepositSweepRecord{}, false
}

func rawUnitsToCents(rawUnits, rawUnitsPerWholeUSDI int64) int64 {
	if rawUnits <= 0 || rawUnitsPerWholeUSDI <= 0 {
		return 0
	}
	return (rawUnits * 100) / rawUnitsPerWholeUSDI
}

func rawUnitsToAmountString(rawUnits, rawUnitsPerWholeUSDI int64) string {
	return centsToAmountString(rawUnitsToCents(rawUnits, rawUnitsPerWholeUSDI))
}

func centsToAmountString(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

func parseAmountToCents64(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	sign := int64(1)
	if strings.HasPrefix(raw, "-") {
		sign = -1
		raw = strings.TrimPrefix(raw, "-")
	}
	parts := strings.SplitN(raw, ".", 2)
	whole, _ := strconv.ParseInt(parts[0], 10, 64)
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1] + "00"
	}
	fraction = fraction[:2]
	cents, _ := strconv.ParseInt(fraction, 10, 64)
	return sign * (whole*100 + cents)
}

func firstNonEmptySweepAddress(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}

type memoryBuyerDepositAddressRepository struct {
	mu        sync.Mutex
	seq       int
	nextIndex int
	data      map[string]BuyerDepositAddress
}

func NewMemoryBuyerDepositAddressRepository() BuyerDepositAddressRepository {
	return &memoryBuyerDepositAddressRepository{data: map[string]BuyerDepositAddress{}}
}

func (r *memoryBuyerDepositAddressRepository) NextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return fmt.Sprintf("depaddr_%d", r.seq), nil
}

func (r *memoryBuyerDepositAddressRepository) NextDerivationIndex() (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	index := r.nextIndex
	r.nextIndex++
	return index, nil
}

func (r *memoryBuyerDepositAddressRepository) Save(record BuyerDepositAddress) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[record.BuyerOrgID] = record
	return nil
}

func (r *memoryBuyerDepositAddressRepository) GetByBuyerOrgID(buyerOrgID string) (BuyerDepositAddress, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.data[buyerOrgID]
	if !ok {
		return BuyerDepositAddress{}, errors.New("buyer deposit address not found")
	}
	return record, nil
}

func (r *memoryBuyerDepositAddressRepository) List() ([]BuyerDepositAddress, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]BuyerDepositAddress, 0, len(r.data))
	for _, record := range r.data {
		records = append(records, record)
	}
	return records, nil
}

type memoryBuyerDepositSweepRepository struct {
	mu   sync.Mutex
	seq  int
	data map[string]BuyerDepositSweepRecord
}

func NewMemoryBuyerDepositSweepRepository() BuyerDepositSweepRepository {
	return &memoryBuyerDepositSweepRepository{data: map[string]BuyerDepositSweepRecord{}}
}

func (r *memoryBuyerDepositSweepRepository) NextID() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return fmt.Sprintf("depsweep_%d", r.seq), nil
}

func (r *memoryBuyerDepositSweepRepository) Save(record BuyerDepositSweepRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[record.ID] = record
	return nil
}

func (r *memoryBuyerDepositSweepRepository) List(filter BuyerDepositSweepFilter) ([]BuyerDepositSweepRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records := make([]BuyerDepositSweepRecord, 0, len(r.data))
	for _, record := range r.data {
		if filter.BuyerOrgID != "" && record.BuyerOrgID != filter.BuyerOrgID {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}
