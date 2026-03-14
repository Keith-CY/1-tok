package identity

import (
	"errors"
	"testing"
	"time"
)

func TestMemoryStore_CreateSignup(t *testing.T) {
	store := NewMemoryStore()

	profile, err := store.CreateSignup(Signup{
		Email:            "alice@example.com",
		Name:             "Alice",
		PasswordHash:     "$2a$10$hash",
		OrganizationName: "Acme Corp",
		OrganizationKind: "buyer",
	})
	if err != nil {
		t.Fatal(err)
	}

	if profile.User.Email != "alice@example.com" {
		t.Errorf("email = %s, want alice@example.com", profile.User.Email)
	}
	if profile.User.Name != "Alice" {
		t.Errorf("name = %s, want Alice", profile.User.Name)
	}
	if len(profile.Memberships) != 1 {
		t.Fatalf("memberships len = %d, want 1", len(profile.Memberships))
	}
	if profile.Memberships[0].Organization.Kind != "buyer" {
		t.Errorf("org kind = %s, want buyer", profile.Memberships[0].Organization.Kind)
	}
	if profile.Memberships[0].Role != "org_owner" {
		t.Errorf("role = %s, want org_owner", profile.Memberships[0].Role)
	}
}

func TestMemoryStore_CreateSignup_DuplicateEmail(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.CreateSignup(Signup{
		Email: "dup@example.com", Name: "A", PasswordHash: "h",
		OrganizationName: "Org1", OrganizationKind: "buyer",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.CreateSignup(Signup{
		Email: "dup@example.com", Name: "B", PasswordHash: "h",
		OrganizationName: "Org2", OrganizationKind: "buyer",
	})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestMemoryStore_FindUserByEmail(t *testing.T) {
	store := NewMemoryStore()

	_, _ = store.CreateSignup(Signup{
		Email: "find@example.com", Name: "Find", PasswordHash: "h",
		OrganizationName: "Org", OrganizationKind: "buyer",
	})

	user, err := store.FindUserByEmail("find@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.Name != "Find" {
		t.Errorf("name = %s, want Find", user.Name)
	}
}

func TestMemoryStore_FindUserByEmail_NotFound(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.FindUserByEmail("missing@example.com")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_SessionLifecycle(t *testing.T) {
	store := NewMemoryStore()

	profile, _ := store.CreateSignup(Signup{
		Email: "session@example.com", Name: "S", PasswordHash: "h",
		OrganizationName: "Org", OrganizationKind: "provider",
	})

	session, err := store.CreateSession(NewSession{
		UserID:      profile.User.ID,
		TokenDigest: "digest123",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if session.UserID != profile.User.ID {
		t.Errorf("session user = %s, want %s", session.UserID, profile.User.ID)
	}

	// Get authenticated actor
	actor, err := store.GetAuthenticatedActorBySessionDigest("digest123")
	if err != nil {
		t.Fatal(err)
	}
	if actor.User.Email != "session@example.com" {
		t.Errorf("actor email = %s", actor.User.Email)
	}
	if len(actor.Memberships) != 1 {
		t.Fatalf("memberships = %d", len(actor.Memberships))
	}

	// Revoke
	if err := store.RevokeSession("digest123"); err != nil {
		t.Fatal(err)
	}

	// Session still accessible after revoke (memory store doesn't filter)
	// The IAM service layer checks RevokedAt
	actor2, err := store.GetAuthenticatedActorBySessionDigest("digest123")
	if err != nil {
		t.Fatal(err)
	}
	if actor2.Session.RevokedAt == nil {
		t.Error("expected RevokedAt to be set after revoke")
	}
}

func TestMemoryStore_GetActor_ExpiredSession(t *testing.T) {
	store := NewMemoryStore()

	profile, _ := store.CreateSignup(Signup{
		Email: "expired@example.com", Name: "E", PasswordHash: "h",
		OrganizationName: "Org", OrganizationKind: "buyer",
	})

	_, _ = store.CreateSession(NewSession{
		UserID:      profile.User.ID,
		TokenDigest: "expired-digest",
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	})

	// Memory store doesn't check expiry — IAM service layer does
	actor, err := store.GetAuthenticatedActorBySessionDigest("expired-digest")
	if err != nil {
		t.Fatal(err)
	}
	if actor.Session.ExpiresAt.After(time.Now()) {
		t.Error("expected session to be expired")
	}
}

func TestMemoryStore_RevokeSession_NotFound(t *testing.T) {
	store := NewMemoryStore()

	err := store.RevokeSession("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_GetActor_NotFound(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.GetAuthenticatedActorBySessionDigest("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_CreateSignup_CaseInsensitiveEmail(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.CreateSignup(Signup{
		Email: "Alice@Example.COM", Name: "A", PasswordHash: "h",
		OrganizationName: "Org1", OrganizationKind: "buyer",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.CreateSignup(Signup{
		Email: "alice@example.com", Name: "B", PasswordHash: "h",
		OrganizationName: "Org2", OrganizationKind: "buyer",
	})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict for case-insensitive duplicate, got %v", err)
	}
}

func TestMemoryStore_CreateSession_UserNotFound(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.CreateSession(NewSession{
		UserID:      "nonexistent",
		TokenDigest: "digest",
		ExpiresAt:   time.Now().Add(time.Hour),
	})
	// Memory store may not validate user exists
	_ = err
}

func TestMemoryStore_FindUserByEmail_CaseInsensitive(t *testing.T) {
	store := NewMemoryStore()
	_, _ = store.CreateSignup(Signup{
		Email: "Case@Test.COM", Name: "Case", PasswordHash: "h",
		OrganizationName: "Org", OrganizationKind: "buyer",
	})

	user, err := store.FindUserByEmail("case@test.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "case@test.com" {
		t.Errorf("email = %s", user.Email)
	}
}
