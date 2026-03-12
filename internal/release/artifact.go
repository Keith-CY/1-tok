package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func WriteJSONArtifact(outputPath string, payload any) error {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(outputPath, raw, 0o644)
}
