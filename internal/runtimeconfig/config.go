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
