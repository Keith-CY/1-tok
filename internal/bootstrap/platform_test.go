package bootstrap

import (
	"os"
	"testing"
)

func TestLoadPlatformAppRequiresPersistenceWhenConfigured(t *testing.T) {
	t.Setenv("ONE_TOK_REQUIRE_PERSISTENCE", "true")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "")

	app, cleanup, err := LoadPlatformApp()
	if cleanup != nil {
		_ = cleanup()
	}
	if err == nil {
		t.Fatalf("expected LoadPlatformApp to fail when persistence is required, got app=%v", app)
	}
}

func TestLoadPlatformApp_WithPostgres(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL is not set")
	}

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")

	app, cleanup, err := LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if app == nil {
		t.Fatal("expected non-nil app")
	}

	// Verify app works
	providers, err := app.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Error("expected seeded providers")
	}
}

func TestLoadPlatformApp_Memory(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "")
	t.Setenv("ONE_TOK_REQUIRE_PERSISTENCE", "")

	app, cleanup, err := LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if app == nil {
		t.Fatal("expected non-nil app")
	}

	// Verify memory store works
	providers, err := app.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Error("expected default providers in memory store")
	}
}

func TestLoadPlatformApp_RequireBootstrapped(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")
	t.Setenv("ONE_TOK_REQUIRE_BOOTSTRAP", "true")

	app, cleanup, err := LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestLoadPlatformApp_WithNATS(t *testing.T) {
	natsURL := os.Getenv("ONE_TOK_TEST_NATS_URL")
	if natsURL == "" {
		t.Skip("ONE_TOK_TEST_NATS_URL not set")
	}

	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", natsURL)

	app, cleanup, err := LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestLoadPlatformApp_WithPostgresAndNATS(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	natsURL := os.Getenv("ONE_TOK_TEST_NATS_URL")
	if dsn == "" || natsURL == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL or ONE_TOK_TEST_NATS_URL not set")
	}

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", natsURL)

	app, cleanup, err := LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if app == nil {
		t.Fatal("expected non-nil app")
	}

	// Verify works end to end
	providers, err := app.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Error("expected providers")
	}
}

func TestLoadPlatformApp_RequirePersistence_NoDSN(t *testing.T) {
	t.Setenv("ONE_TOK_REQUIRE_PERSISTENCE", "true")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "")

	_, _, err := LoadPlatformApp()
	if err == nil {
		t.Error("expected error when persistence required but no DSN")
	}
}

func TestLoadPlatformApp_InvalidDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://invalid:invalid@127.0.0.1:1/invalid")
	t.Setenv("NATS_URL", "")
	t.Setenv("ONE_TOK_REQUIRE_PERSISTENCE", "")
	t.Setenv("ONE_TOK_REQUIRE_BOOTSTRAP", "")

	_, _, err := LoadPlatformApp()
	if err == nil {
		t.Error("expected error for invalid DSN")
	}
}

func TestLoadPlatformApp_InvalidNATS(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "nats://127.0.0.1:1")

	_, _, err := LoadPlatformApp()
	if err == nil {
		t.Error("expected error for unreachable NATS")
	}
}

func TestLoadPlatformApp_CleanupWorks(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "")

	app, cleanup, err := LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	if app == nil {
		t.Fatal("expected non-nil app")
	}
	if err := cleanup(); err != nil {
		t.Errorf("cleanup error: %v", err)
	}
}

func TestLoadPlatformApp_WithPostgresAndRequireBootstrap(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("NATS_URL", "")
	t.Setenv("ONE_TOK_REQUIRE_BOOTSTRAP", "true")

	app, cleanup, err := LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if app == nil {
		t.Fatal("expected non-nil app")
	}
}

func TestLoadPlatformApp_MemoryWithCleanup(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NATS_URL", "")
	t.Setenv("ONE_TOK_REQUIRE_PERSISTENCE", "")

	app, cleanup, err := LoadPlatformApp()
	if err != nil {
		t.Fatal(err)
	}

	// Test cleanup
	if err := cleanup(); err != nil {
		t.Errorf("cleanup error: %v", err)
	}

	// App should still work after cleanup (memory store)
	providers, err := app.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Error("expected providers")
	}
}
