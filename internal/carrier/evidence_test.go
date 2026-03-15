package carrier

import "testing"

func TestEvidenceStore_Submit(t *testing.T) {
	store := NewEvidenceStore()
	artifacts := []Artifact{
		{Name: "execution.log", Type: "log", URL: "https://carrier.example.com/logs/123"},
		{Name: "output.json", Type: "output", URL: "https://carrier.example.com/output/123", SizeBytes: 4096},
	}
	usage := &UsageReport{TokenCount: 5000, StepCount: 12, TotalCostCents: 250}

	pkg, err := store.Submit("job_1", "bind_1", "Completed successfully. 12 steps, 5k tokens.", artifacts, usage)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.JobID != "job_1" {
		t.Errorf("jobID = %s", pkg.JobID)
	}
	if len(pkg.Artifacts) != 2 {
		t.Errorf("artifacts = %d", len(pkg.Artifacts))
	}
	if pkg.UsageReport.TokenCount != 5000 {
		t.Errorf("tokens = %d", pkg.UsageReport.TokenCount)
	}
}

func TestEvidenceStore_Get(t *testing.T) {
	store := NewEvidenceStore()
	store.Submit("job_1", "bind_1", "Done", nil, nil)

	pkg, err := store.Get("job_1")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Summary != "Done" {
		t.Errorf("summary = %s", pkg.Summary)
	}
}

func TestEvidenceStore_GetMissing(t *testing.T) {
	store := NewEvidenceStore()
	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestEvidenceStore_DuplicateSubmit(t *testing.T) {
	store := NewEvidenceStore()
	store.Submit("job_1", "bind_1", "Done", nil, nil)
	_, err := store.Submit("job_1", "bind_1", "Again", nil, nil)
	if err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestEvidenceStore_NilUsage(t *testing.T) {
	store := NewEvidenceStore()
	pkg, err := store.Submit("job_1", "bind_1", "Done", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.UsageReport != nil {
		t.Error("expected nil usage report")
	}
}
