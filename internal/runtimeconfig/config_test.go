package runtimeconfig

import (
	"testing"
)

func TestRequirePersistence(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"unset", "", false},
		{"true", "true", true},
		{"false", "false", false},
		{"1", "1", true},
		{"0", "0", false},
		{"TRUE", "TRUE", true},
		{"invalid", "maybe", false},
		{"whitespace", "  true  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ONE_TOK_REQUIRE_PERSISTENCE", tt.value)

			if got := RequirePersistence(); got != tt.want {
				t.Errorf("RequirePersistence() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequireExternalDependencies(t *testing.T) {
	t.Setenv("ONE_TOK_REQUIRE_EXTERNALS", "true")

	if !RequireExternalDependencies() {
		t.Error("expected true when env set to true")
	}
}

func TestRequireBootstrappedDatabase(t *testing.T) {
	t.Setenv("ONE_TOK_REQUIRE_BOOTSTRAP", "true")

	if !RequireBootstrappedDatabase() {
		t.Error("expected true when env set to true")
	}
}

func TestAPIGatewayUpstream_Default(t *testing.T) {
	t.Setenv("API_GATEWAY_UPSTREAM", "")

	if got := APIGatewayUpstream(); got != DefaultAPIGatewayUpstream {
		t.Errorf("APIGatewayUpstream() = %q, want %q", got, DefaultAPIGatewayUpstream)
	}
}

func TestAPIGatewayUpstream_Custom(t *testing.T) {
	t.Setenv("API_GATEWAY_UPSTREAM", "http://custom:9090")
	defer t.Setenv("API_GATEWAY_UPSTREAM", "")

	if got := APIGatewayUpstream(); got != "http://custom:9090" {
		t.Errorf("APIGatewayUpstream() = %q, want http://custom:9090", got)
	}
}
