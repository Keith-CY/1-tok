package identity

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("identity not found")
	ErrConflict = errors.New("identity conflict")
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"createdAt"`
}

type Membership struct {
	UserID         string    `json:"userId"`
	OrganizationID string    `json:"organizationId"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Session struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	TokenDigest string     `json:"-"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	RevokedAt   *time.Time `json:"revokedAt,omitempty"`
}

type ActorMembership struct {
	Organization Organization `json:"organization"`
	Role         string       `json:"role"`
}

type ActorProfile struct {
	User        User              `json:"user"`
	Memberships []ActorMembership `json:"memberships"`
}

type AuthenticatedActor struct {
	ActorProfile
	Session Session `json:"session"`
}

type Signup struct {
	Email            string
	Name             string
	PasswordHash     string
	OrganizationName string
	OrganizationKind string
}

type NewSession struct {
	UserID      string
	TokenDigest string
	ExpiresAt   time.Time
}

type Store interface {
	CreateSignup(Signup) (ActorProfile, error)
	FindUserByEmail(email string) (User, error)
	CreateSession(NewSession) (Session, error)
	GetAuthenticatedActorBySessionDigest(tokenDigest string) (AuthenticatedActor, error)
	RevokeSession(tokenDigest string) error
}

type MemoryStore struct {
	mu sync.RWMutex

	userSeq    int64
	orgSeq     int64
	sessionSeq int64

	usersByID           map[string]User
	userIDsByEmail      map[string]string
	organizationsByID   map[string]Organization
	membershipsByUserID map[string][]Membership
	sessionsByDigest    map[string]Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		usersByID:           make(map[string]User),
		userIDsByEmail:      make(map[string]string),
		organizationsByID:   make(map[string]Organization),
		membershipsByUserID: make(map[string][]Membership),
		sessionsByDigest:    make(map[string]Session),
	}
}

func (s *MemoryStore) CreateSignup(input Signup) (ActorProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	emailKey := strings.ToLower(strings.TrimSpace(input.Email))
	if emailKey == "" {
		return ActorProfile{}, fmt.Errorf("%w: missing email", ErrConflict)
	}
	if _, exists := s.userIDsByEmail[emailKey]; exists {
		return ActorProfile{}, ErrConflict
	}

	now := time.Now().UTC()
	s.userSeq++
	user := User{
		ID:           fmt.Sprintf("usr_%d", s.userSeq),
		Email:        emailKey,
		Name:         strings.TrimSpace(input.Name),
		PasswordHash: input.PasswordHash,
		CreatedAt:    now,
	}

	s.orgSeq++
	organization := Organization{
		ID:        fmt.Sprintf("org_%d", s.orgSeq),
		Name:      strings.TrimSpace(input.OrganizationName),
		Kind:      strings.TrimSpace(input.OrganizationKind),
		CreatedAt: now,
	}

	membership := Membership{
		UserID:         user.ID,
		OrganizationID: organization.ID,
		Role:           DefaultMembershipRoleForOrganizationKind(organization.Kind),
		CreatedAt:      now,
	}

	s.usersByID[user.ID] = user
	s.userIDsByEmail[emailKey] = user.ID
	s.organizationsByID[organization.ID] = organization
	s.membershipsByUserID[user.ID] = append(s.membershipsByUserID[user.ID], membership)

	return ActorProfile{
		User: user,
		Memberships: []ActorMembership{
			{
				Organization: organization,
				Role:         membership.Role,
			},
		},
	}, nil
}

func (s *MemoryStore) FindUserByEmail(email string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.userIDsByEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return User{}, ErrNotFound
	}
	user, ok := s.usersByID[userID]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) CreateSession(input NewSession) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.usersByID[input.UserID]; !exists {
		return Session{}, ErrNotFound
	}

	s.sessionSeq++
	session := Session{
		ID:          fmt.Sprintf("sess_%d", s.sessionSeq),
		UserID:      input.UserID,
		TokenDigest: input.TokenDigest,
		ExpiresAt:   input.ExpiresAt,
		CreatedAt:   time.Now().UTC(),
	}
	s.sessionsByDigest[input.TokenDigest] = session
	return session, nil
}

func (s *MemoryStore) GetAuthenticatedActorBySessionDigest(tokenDigest string) (AuthenticatedActor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessionsByDigest[tokenDigest]
	if !ok {
		return AuthenticatedActor{}, ErrNotFound
	}

	user, ok := s.usersByID[session.UserID]
	if !ok {
		return AuthenticatedActor{}, ErrNotFound
	}

	memberships := s.membershipsByUserID[user.ID]
	actorMemberships := make([]ActorMembership, 0, len(memberships))
	for _, membership := range memberships {
		org, exists := s.organizationsByID[membership.OrganizationID]
		if !exists {
			continue
		}
		actorMemberships = append(actorMemberships, ActorMembership{
			Organization: org,
			Role:         membership.Role,
		})
	}

	return AuthenticatedActor{
		ActorProfile: ActorProfile{
			User:        user,
			Memberships: actorMemberships,
		},
		Session: session,
	}, nil
}

func (s *MemoryStore) RevokeSession(tokenDigest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessionsByDigest[tokenDigest]
	if !ok {
		return ErrNotFound
	}

	now := time.Now().UTC()
	session.RevokedAt = &now
	s.sessionsByDigest[tokenDigest] = session
	return nil
}
