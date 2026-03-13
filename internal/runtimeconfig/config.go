package runtimeconfig

import (
	"os"
	"strconv"
	"strings"
)

func RequirePersistence() bool {
	return envBool("ONE_TOK_REQUIRE_PERSISTENCE")
}

func RequireExternalDependencies() bool {
	return envBool("ONE_TOK_REQUIRE_EXTERNALS")
}

func RequireBootstrappedDatabase() bool {
	return envBool("ONE_TOK_REQUIRE_BOOTSTRAP")
}

func envBool(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}

	required, err := strconv.ParseBool(value)
	return err == nil && required
}

// DefaultAPIGatewayUpstream is the default address for the api-gateway service.
const DefaultAPIGatewayUpstream = "http://127.0.0.1:8080"

// APIGatewayUpstream returns the api-gateway upstream URL from the
// API_GATEWAY_UPSTREAM environment variable, falling back to
// DefaultAPIGatewayUpstream.
func APIGatewayUpstream() string {
	if value := os.Getenv("API_GATEWAY_UPSTREAM"); value != "" {
		return value
	}
	return DefaultAPIGatewayUpstream
}
