package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONArtifactWritesStructuredSummary(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "summary.json")

	if err := WriteJSONArtifact(outputPath, map[string]any{
		"orderId": "ord_1",
		"status":  "ok",
	}); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode artifact: %v", err)
	}
	if decoded["orderId"] != "ord_1" || decoded["status"] != "ok" {
		t.Fatalf("unexpected artifact contents: %+v", decoded)
	}
}

func TestWriteJSONArtifactAllowsEmptyOutputPath(t *testing.T) {
	if err := WriteJSONArtifact("", map[string]string{"status": "ok"}); err != nil {
		t.Fatalf("expected empty output path to be a no-op, got %v", err)
	}
}
