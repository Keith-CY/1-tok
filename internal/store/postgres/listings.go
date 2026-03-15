package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/chenyu/1-tok/internal/platform"
)

type ListingRepository struct {
	db *sql.DB
}

func NewListingRepository(db *sql.DB) *ListingRepository {
	return &ListingRepository{db: db}
}

func (r *ListingRepository) List() ([]platform.Listing, error) {
	rows, err := r.db.QueryContext(context.TODO(), `
		SELECT id, provider_org_id, title, category, base_price_cents, tags
		FROM listings
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	listings := make([]platform.Listing, 0)
	for rows.Next() {
		var listing platform.Listing
		var tags []byte
		if err := rows.Scan(
			&listing.ID,
			&listing.ProviderOrgID,
			&listing.Title,
			&listing.Category,
			&listing.BasePriceCents,
			&tags,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(tags, &listing.Tags); err != nil {
			return nil, err
		}
		listings = append(listings, listing)
	}

	return listings, rows.Err()
}

func (r *ListingRepository) Upsert(listing platform.Listing) error {
	tags, err := json.Marshal(listing.Tags)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(context.TODO(), `
		INSERT INTO listings (id, provider_org_id, title, category, base_price_cents, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET
			provider_org_id = EXCLUDED.provider_org_id,
			title = EXCLUDED.title,
			category = EXCLUDED.category,
			base_price_cents = EXCLUDED.base_price_cents,
			tags = EXCLUDED.tags,
			updated_at = NOW()
	`, listing.ID, listing.ProviderOrgID, listing.Title, listing.Category, listing.BasePriceCents, tags)
	return err
}

func (r *ListingRepository) Get(id string) (platform.Listing, error) {
	var l platform.Listing
	var tags string
	err := r.db.QueryRowContext(context.TODO(), `
		SELECT id, provider_org_id, title, category, base_price_cents, tags
		FROM listings WHERE id = $1
	`, id).Scan(&l.ID, &l.ProviderOrgID, &l.Title, &l.Category, &l.BasePriceCents, &tags)
	if err != nil {
		return platform.Listing{}, fmt.Errorf("listing not found: %s", id)
	}
	if err := json.Unmarshal([]byte(tags), &l.Tags); err != nil {
		l.Tags = []string{}
	}
	return l, nil
}
