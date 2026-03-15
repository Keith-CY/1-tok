package carrier

import (
	"testing"
	"time"
)

func TestBind_Success(t *testing.T) {
	svc := NewService()
	b, err := svc.Bind("ord_1", "ms_1", "carrier_a", []string{"gpu", "inference"})
	if err != nil {
		t.Fatal(err)
	}
	if b.CarrierID != "carrier_a" {
		t.Errorf("carrierID = %s", b.CarrierID)
	}
	if len(b.Capabilities) != 2 {
		t.Errorf("capabilities = %v", b.Capabilities)
	}
}

func TestBind_Duplicate(t *testing.T) {
	svc := NewService()
	svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	_, err := svc.Bind("ord_1", "ms_1", "carrier_b", nil)
	if err != ErrBindingExists {
		t.Fatalf("expected ErrBindingExists, got %v", err)
	}
}

func TestGetBinding(t *testing.T) {
	svc := NewService()
	svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	b, err := svc.GetBinding("ord_1", "ms_1")
	if err != nil {
		t.Fatal(err)
	}
	if b.CarrierID != "carrier_a" {
		t.Errorf("carrierID = %s", b.CarrierID)
	}
}

func TestGetBinding_NotFound(t *testing.T) {
	svc := NewService()
	_, err := svc.GetBinding("ord_1", "ms_1")
	if err != ErrBindingNotFound {
		t.Fatalf("expected ErrBindingNotFound, got %v", err)
	}
}

func TestHeartbeat(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	time.Sleep(10 * time.Millisecond)
	err := svc.Heartbeat(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := svc.GetBinding("ord_1", "ms_1")
	if !updated.LastHeartbeat.After(b.LastHeartbeat) {
		t.Error("heartbeat should update timestamp")
	}
}

func TestHeartbeat_NotFound(t *testing.T) {
	svc := NewService()
	err := svc.Heartbeat("nonexistent")
	if err != ErrBindingNotFound {
		t.Fatalf("expected ErrBindingNotFound, got %v", err)
	}
}

func TestIsStale(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	stale, err := svc.IsStale(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("fresh binding should not be stale")
	}
}

func TestIsStale_NotFound(t *testing.T) {
	svc := NewService()
	_, err := svc.IsStale("nonexistent")
	if err != ErrBindingNotFound {
		t.Fatalf("expected ErrBindingNotFound, got %v", err)
	}
}

func TestCreateJob(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, err := svc.CreateJob(b.ID, "ms_1", `{"prompt":"hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != JobStatePending {
		t.Errorf("state = %s", job.State)
	}
	if job.Input != `{"prompt":"hello"}` {
		t.Errorf("input = %s", job.Input)
	}
}

func TestCreateJob_BindingNotFound(t *testing.T) {
	svc := NewService()
	_, err := svc.CreateJob("nonexistent", "ms_1", "")
	if err != ErrBindingNotFound {
		t.Fatalf("expected ErrBindingNotFound, got %v", err)
	}
}

func TestJobLifecycle_HappyPath(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")

	// Start
	started, err := svc.StartJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if started.State != JobStateRunning {
		t.Errorf("state = %s", started.State)
	}
	if started.StartedAt == nil {
		t.Error("startedAt should be set")
	}

	// Progress
	prog, err := svc.UpdateProgress(job.ID, 5, 10, "processing")
	if err != nil {
		t.Fatal(err)
	}
	if prog.Progress.Step != 5 || prog.Progress.Total != 10 {
		t.Errorf("progress = %+v", prog.Progress)
	}

	// Complete
	completed, err := svc.CompleteJob(job.ID, "result data")
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != JobStateCompleted {
		t.Errorf("state = %s", completed.State)
	}
	if completed.Output != "result data" {
		t.Errorf("output = %s", completed.Output)
	}
	if completed.CompletedAt == nil {
		t.Error("completedAt should be set")
	}
}

func TestJobLifecycle_Failure(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")

	svc.StartJob(job.ID)
	failed, err := svc.FailJob(job.ID, "out of memory")
	if err != nil {
		t.Fatal(err)
	}
	if failed.State != JobStateFailed {
		t.Errorf("state = %s", failed.State)
	}
	if failed.ErrorMessage != "out of memory" {
		t.Errorf("error = %s", failed.ErrorMessage)
	}
}

func TestJobLifecycle_Cancel(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")

	// Cancel from pending
	cancelled, err := svc.CancelJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.State != JobStateCancelled {
		t.Errorf("state = %s", cancelled.State)
	}
}

func TestJobLifecycle_CancelRunning(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")
	svc.StartJob(job.ID)

	cancelled, err := svc.CancelJob(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.State != JobStateCancelled {
		t.Errorf("state = %s", cancelled.State)
	}
}

func TestJobInvalidTransition(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")

	// Can't complete from pending
	_, err := svc.CompleteJob(job.ID, "output")
	if err == nil {
		t.Error("expected error for pending → completed")
	}

	// Can't fail from pending
	_, err = svc.FailJob(job.ID, "err")
	if err == nil {
		t.Error("expected error for pending → failed")
	}
}

func TestJobInvalidTransition_FromCompleted(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")
	svc.StartJob(job.ID)
	svc.CompleteJob(job.ID, "done")

	// Can't start completed job
	_, err := svc.StartJob(job.ID)
	if err == nil {
		t.Error("expected error for completed → running")
	}
}

func TestGetJob_NotFound(t *testing.T) {
	svc := NewService()
	_, err := svc.GetJob("nonexistent")
	if err != ErrJobNotFound {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestTransitionJob_NotFound(t *testing.T) {
	svc := NewService()
	_, err := svc.StartJob("nonexistent")
	if err != ErrJobNotFound {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestUpdateProgress_NotRunning(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")

	_, err := svc.UpdateProgress(job.ID, 1, 10, "step 1")
	if err == nil {
		t.Error("expected error for progress on pending job")
	}
}

func TestUpdateProgress_NotFound(t *testing.T) {
	svc := NewService()
	_, err := svc.UpdateProgress("nonexistent", 1, 10, "step 1")
	if err != ErrJobNotFound {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestListJobs(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	svc.CreateJob(b.ID, "ms_1", "input1")
	svc.CreateJob(b.ID, "ms_1", "input2")

	jobs, err := svc.ListJobs(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestListJobs_Empty(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	jobs, err := svc.ListJobs(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestCanTransition_UnknownState(t *testing.T) {
	if canTransition("unknown", JobStateRunning) {
		t.Error("unknown state should not transition")
	}
}

func TestGetJob_Found(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	created, _ := svc.CreateJob(b.ID, "ms_1", "test-input")
	got, err := svc.GetJob(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Input != "test-input" {
		t.Errorf("input = %s", got.Input)
	}
}

func TestBind_MultipleOrders(t *testing.T) {
	svc := NewService()
	b1, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	b2, _ := svc.Bind("ord_2", "ms_1", "carrier_b", nil)
	if b1.ID == b2.ID {
		t.Error("expected different binding IDs")
	}

	// Verify O(1) lookup
	got1, err := svc.GetBinding("ord_1", "ms_1")
	if err != nil || got1.CarrierID != "carrier_a" {
		t.Errorf("got1 = %v, err = %v", got1, err)
	}
	got2, err := svc.GetBinding("ord_2", "ms_1")
	if err != nil || got2.CarrierID != "carrier_b" {
		t.Errorf("got2 = %v, err = %v", got2, err)
	}
}

func TestListJobs_ByBinding(t *testing.T) {
	svc := NewService()
	b1, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	b2, _ := svc.Bind("ord_2", "ms_1", "carrier_b", nil)

	svc.CreateJob(b1.ID, "ms_1", "input1")
	svc.CreateJob(b1.ID, "ms_1", "input2")
	svc.CreateJob(b2.ID, "ms_1", "input3")

	jobs1, _ := svc.ListJobs(b1.ID)
	if len(jobs1) != 2 {
		t.Errorf("expected 2 jobs for b1, got %d", len(jobs1))
	}
	jobs2, _ := svc.ListJobs(b2.ID)
	if len(jobs2) != 1 {
		t.Errorf("expected 1 job for b2, got %d", len(jobs2))
	}
}

func TestDuplicateCallback_CompleteTwice(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")
	svc.StartJob(job.ID)
	svc.CompleteJob(job.ID, "result")

	// Duplicate complete should fail (already in terminal state)
	_, err := svc.CompleteJob(job.ID, "result again")
	if err == nil {
		t.Error("expected error on duplicate complete")
	}
}

func TestOutOfOrder_CompleteBeforeStart(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")

	// Complete from pending state should fail
	_, err := svc.CompleteJob(job.ID, "result")
	if err == nil {
		t.Error("expected error on complete before start")
	}
}

func TestOutOfOrder_FailBeforeStart(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")

	_, err := svc.FailJob(job.ID, "error")
	if err == nil {
		t.Error("expected error on fail before start")
	}
}

func TestDuplicateCallback_StartTwice(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")
	svc.StartJob(job.ID)

	// Duplicate start should fail (already running)
	_, err := svc.StartJob(job.ID)
	if err == nil {
		t.Error("expected error on duplicate start")
	}
}

func TestReconcileStaleJobs_NoStale(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")
	svc.StartJob(job.ID)

	stale := svc.ReconcileStaleJobs()
	if len(stale) != 0 {
		t.Errorf("expected 0 stale, got %d", len(stale))
	}
}

func TestReconcileStaleJobs_OnlyRunning(t *testing.T) {
	svc := NewService()
	b, _ := svc.Bind("ord_1", "ms_1", "carrier_a", nil)
	job, _ := svc.CreateJob(b.ID, "ms_1", "input")

	// Pending job should not appear as stale
	_ = job
	stale := svc.ReconcileStaleJobs()
	if len(stale) != 0 {
		t.Errorf("pending job should not be stale, got %d", len(stale))
	}
}
