package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReleaseManifestCollectsAvailableArtifacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "external-preflight.json"), []byte(`{"status":"ok"}`), 0o644); err != nil {
		t.Fatalf("write preflight artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release-smoke.json"), []byte(`{"OrderID":"ord_1"}`), 0o644); err != nil {
		t.Fatalf("write smoke artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release-portal-smoke.json"), []byte(`{"orderId":"ord_2"}`), 0o644); err != nil {
		t.Fatalf("write portal artifact: %v", err)
	}

	outputPath := filepath.Join(dir, "release-manifest.json")
	err := WriteReleaseManifest(outputPath, ReleaseManifestInput{
		ArtifactDir:       dir,
		GitSHA:            "abc123",
		FiberRPCURL:       "https://fiber.example/rpc",
		CarrierGatewayURL: "https://carrier.example",
	})
	if err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		GitSHA               string            `json:"gitSha"`
		Artifacts            map[string]string `json:"artifacts"`
		ExternalDependencies struct {
			FiberRPCURL       string `json:"fiberRpcUrl"`
			CarrierGatewayURL string `json:"carrierGatewayUrl"`
		} `json:"externalDependencies"`
		Preflight json.RawMessage `json:"preflight"`
		Smoke     json.RawMessage `json:"smoke"`
		Portal    json.RawMessage `json:"portal"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	if manifest.GitSHA != "abc123" {
		t.Fatalf("unexpected git sha: %+v", manifest)
	}
	if manifest.ExternalDependencies.FiberRPCURL != "https://fiber.example/rpc" || manifest.ExternalDependencies.CarrierGatewayURL != "https://carrier.example" {
		t.Fatalf("unexpected external dependencies: %+v", manifest.ExternalDependencies)
	}
	if manifest.Artifacts["preflight"] == "" || manifest.Artifacts["smoke"] == "" || manifest.Artifacts["portal"] == "" {
		t.Fatalf("expected artifact paths in manifest, got %+v", manifest.Artifacts)
	}
	if string(manifest.Preflight) == "" || string(manifest.Smoke) == "" || string(manifest.Portal) == "" {
		t.Fatalf("expected embedded artifact payloads, got %+v", manifest)
	}
}

func TestWriteReleaseManifestAllowsMissingOptionalArtifacts(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "release-manifest.json")

	if err := WriteReleaseManifest(outputPath, ReleaseManifestInput{
		ArtifactDir: dir,
		GitSHA:      "def456",
	}); err != nil {
		t.Fatalf("write manifest with missing artifacts: %v", err)
	}
}
