package postgres

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/chenyu/1-tok/internal/identity"
)

type IdentityStore struct {
	db *sql.DB
}

func NewIdentityStore(db *sql.DB) *IdentityStore {
	return &IdentityStore{db: db}
}

func (s *IdentityStore) CreateSignup(input identity.Signup) (identity.ActorProfile, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return identity.ActorProfile{}, err
	}
	defer tx.Rollback()

	email := strings.ToLower(strings.TrimSpace(input.Email))
	var existing string
	err = tx.QueryRow(`SELECT id FROM users WHERE email = $1`, email).Scan(&existing)
	switch {
	case err == nil:
		return identity.ActorProfile{}, identity.ErrConflict
	case errors.Is(err, sql.ErrNoRows):
	default:
		return identity.ActorProfile{}, err
	}

	userID, err := nextIDScanner(tx, "user_seq", "usr")
	if err != nil {
		return identity.ActorProfile{}, err
	}
	orgID, err := nextIDScanner(tx, "organization_seq", "org")
	if err != nil {
		return identity.ActorProfile{}, err
	}

	if _, err := tx.Exec(`
		INSERT INTO users (id, email, name, password_hash, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`, userID, email, strings.TrimSpace(input.Name), input.PasswordHash); err != nil {
		return identity.ActorProfile{}, err
	}

	if _, err := tx.Exec(`
		INSERT INTO organizations (id, name, kind, created_at)
		VALUES ($1, $2, $3, NOW())
	`, orgID, strings.TrimSpace(input.OrganizationName), strings.TrimSpace(input.OrganizationKind)); err != nil {
		return identity.ActorProfile{}, err
	}

	if _, err := tx.Exec(`
		INSERT INTO memberships (user_id, organization_id, role, created_at)
		VALUES ($1, $2, $3, NOW())
	`, userID, orgID, "org_owner"); err != nil {
		return identity.ActorProfile{}, err
	}

	if err := tx.Commit(); err != nil {
		return identity.ActorProfile{}, err
	}

	return identity.ActorProfile{
		User: identity.User{
			ID:           userID,
			Email:        email,
			Name:         strings.TrimSpace(input.Name),
			PasswordHash: input.PasswordHash,
		},
		Memberships: []identity.ActorMembership{
			{
				Organization: identity.Organization{
					ID:   orgID,
					Name: strings.TrimSpace(input.OrganizationName),
					Kind: strings.TrimSpace(input.OrganizationKind),
				},
				Role: "org_owner",
			},
		},
	}, nil
}

func (s *IdentityStore) FindUserByEmail(email string) (identity.User, error) {
	var user identity.User
	err := s.db.QueryRow(`
		SELECT id, email, name, password_hash, created_at
		FROM users
		WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(email))).Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.User{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.User{}, err
	}
	return user, nil
}

func (s *IdentityStore) CreateSession(input identity.NewSession) (identity.Session, error) {
	sessionID, err := nextID(s.db, "iam_session_seq", "sess")
	if err != nil {
		return identity.Session{}, err
	}

	_, err = s.db.Exec(`
		INSERT INTO iam_sessions (id, user_id, token_digest, expires_at, created_at, revoked_at)
		VALUES ($1, $2, $3, $4, NOW(), NULL)
	`, sessionID, input.UserID, input.TokenDigest, input.ExpiresAt)
	if err != nil {
		return identity.Session{}, err
	}

	return identity.Session{
		ID:          sessionID,
		UserID:      input.UserID,
		TokenDigest: input.TokenDigest,
		ExpiresAt:   input.ExpiresAt,
	}, nil
}

func (s *IdentityStore) GetAuthenticatedActorBySessionDigest(tokenDigest string) (identity.AuthenticatedActor, error) {
	var actor identity.AuthenticatedActor
	err := s.db.QueryRow(`
		SELECT s.id, s.user_id, s.token_digest, s.expires_at, s.created_at, s.revoked_at,
		       u.id, u.email, u.name, u.password_hash, u.created_at
		FROM iam_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_digest = $1
	`, tokenDigest).Scan(
		&actor.Session.ID,
		&actor.Session.UserID,
		&actor.Session.TokenDigest,
		&actor.Session.ExpiresAt,
		&actor.Session.CreatedAt,
		&actor.Session.RevokedAt,
		&actor.User.ID,
		&actor.User.Email,
		&actor.User.Name,
		&actor.User.PasswordHash,
		&actor.User.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.AuthenticatedActor{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.AuthenticatedActor{}, err
	}

	rows, err := s.db.Query(`
		SELECT o.id, o.name, o.kind, o.created_at, m.role
		FROM memberships m
		JOIN organizations o ON o.id = m.organization_id
		WHERE m.user_id = $1
		ORDER BY o.id ASC
	`, actor.User.ID)
	if err != nil {
		return identity.AuthenticatedActor{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var membership identity.ActorMembership
		if err := rows.Scan(
			&membership.Organization.ID,
			&membership.Organization.Name,
			&membership.Organization.Kind,
			&membership.Organization.CreatedAt,
			&membership.Role,
		); err != nil {
			return identity.AuthenticatedActor{}, err
		}
		actor.Memberships = append(actor.Memberships, membership)
	}

	if err := rows.Err(); err != nil {
		return identity.AuthenticatedActor{}, err
	}

	return actor, nil
}

func (s *IdentityStore) RevokeSession(tokenDigest string) error {
	result, err := s.db.Exec(`
		UPDATE iam_sessions
		SET revoked_at = NOW()
		WHERE token_digest = $1 AND revoked_at IS NULL
	`, tokenDigest)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return identity.ErrNotFound
	}
	return nil
}
