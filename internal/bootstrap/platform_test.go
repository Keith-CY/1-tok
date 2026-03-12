package bootstrap

import "testing"

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
