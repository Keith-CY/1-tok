package runtimeconfig

import (
	"os"
	"strconv"
	"strings"
)

func RequirePersistence() bool {
	value := strings.TrimSpace(os.Getenv("ONE_TOK_REQUIRE_PERSISTENCE"))
	if value == "" {
		return false
	}

	required, err := strconv.ParseBool(value)
	return err == nil && required
}
