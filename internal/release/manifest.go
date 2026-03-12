package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ReleaseManifestInput struct {
	ArtifactDir       string
	GitSHA            string
	FiberRPCURL       string
	CarrierGatewayURL string
}

type ReleaseManifest struct {
	GeneratedAt          string            `json:"generatedAt"`
	GitSHA               string            `json:"gitSha"`
	ArtifactDir          string            `json:"artifactDir"`
	Artifacts            map[string]string `json:"artifacts"`
	ExternalDependencies struct {
		FiberRPCURL       string `json:"fiberRpcUrl"`
		CarrierGatewayURL string `json:"carrierGatewayUrl"`
	} `json:"externalDependencies"`
	Preflight json.RawMessage `json:"preflight,omitempty"`
	Smoke     json.RawMessage `json:"smoke,omitempty"`
	Portal    json.RawMessage `json:"portal,omitempty"`
}

func WriteReleaseManifest(outputPath string, input ReleaseManifestInput) error {
	manifest := ReleaseManifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		GitSHA:      input.GitSHA,
		ArtifactDir: input.ArtifactDir,
		Artifacts:   map[string]string{},
	}
	manifest.ExternalDependencies.FiberRPCURL = input.FiberRPCURL
	manifest.ExternalDependencies.CarrierGatewayURL = input.CarrierGatewayURL

	if payload, path, err := optionalArtifact(input.ArtifactDir, "external-preflight.json"); err != nil {
		return err
	} else if len(payload) > 0 {
		manifest.Artifacts["preflight"] = path
		manifest.Preflight = payload
	}

	if payload, path, err := optionalArtifact(input.ArtifactDir, "release-smoke.json"); err != nil {
		return err
	} else if len(payload) > 0 {
		manifest.Artifacts["smoke"] = path
		manifest.Smoke = payload
	}

	if payload, path, err := optionalArtifact(input.ArtifactDir, "release-portal-smoke.json"); err != nil {
		return err
	} else if len(payload) > 0 {
		manifest.Artifacts["portal"] = path
		manifest.Portal = payload
	}

	return WriteJSONArtifact(outputPath, manifest)
}

func optionalArtifact(dir, name string) (json.RawMessage, string, error) {
	path := filepath.Join(dir, name)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	return json.RawMessage(raw), path, nil
}
