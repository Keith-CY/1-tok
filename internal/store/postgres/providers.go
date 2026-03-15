package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/chenyu/1-tok/internal/platform"
)

type ProviderRepository struct {
	db *sql.DB
}

func NewProviderRepository(db *sql.DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

func (r *ProviderRepository) List() ([]platform.ProviderProfile, error) {
	rows, err := r.db.QueryContext(context.TODO(), `
		SELECT id, name, capabilities, reputation_tier
		FROM providers
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	providers := make([]platform.ProviderProfile, 0)
	for rows.Next() {
		var provider platform.ProviderProfile
		var capabilities []byte
		if err := rows.Scan(&provider.ID, &provider.Name, &capabilities, &provider.ReputationTier); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(capabilities, &provider.Capabilities); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}

	return providers, rows.Err()
}

func (r *ProviderRepository) Upsert(provider platform.ProviderProfile) error {
	capabilities, err := json.Marshal(provider.Capabilities)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(context.TODO(), `
		INSERT INTO providers (id, name, capabilities, reputation_tier, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			capabilities = EXCLUDED.capabilities,
			reputation_tier = EXCLUDED.reputation_tier,
			updated_at = NOW()
	`, provider.ID, provider.Name, capabilities, provider.ReputationTier)
	return err
}
